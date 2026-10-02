package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"github.com/santapong/KeepSave/backend/internal/config"
)

// Provider responses and transport errors can contain credentials: return only a sentinel.
func (s *SocialAuthService) providerJSON(ctx context.Context, method, endpoint, token string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return ErrSocialIdentity
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "KeepSave")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return ErrSocialIdentity
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ErrSocialIdentity
	}
	bytes, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(bytes) > 1<<20 {
		return ErrSocialIdentity
	}
	if json.Unmarshal(bytes, out) != nil {
		return ErrSocialIdentity
	}
	return nil
}
func (s *SocialAuthService) exchange(ctx context.Context, provider string, p config.SocialProvider, code, state, verifier string) (*SocialIdentity, error) {
	form := url.Values{"client_id": {p.ClientID}, "client_secret": {p.ClientSecret}, "code": {code}, "redirect_uri": {s.callback(provider)}, "code_verifier": {verifier}, "grant_type": {"authorization_code"}}
	endpoint := "https://github.com/login/oauth/access_token"
	if provider == "google" {
		endpoint = "https://oauth2.googleapis.com/token"
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		Error       string `json:"error"`
		TokenType   string `json:"token_type"`
	}
	if err := s.providerJSON(ctx, http.MethodPost, endpoint, "", form, &tokens); err != nil {
		return nil, err
	}
	if tokens.Error != "" {
		return nil, ErrSocialIdentity
	}
	identity := &SocialIdentity{}
	if provider == "google" {
		if tokens.IDToken == "" {
			return nil, ErrSocialIdentity
		}
		verified, err := s.googleVerifier.Verify(ctx, tokens.IDToken)
		if err != nil {
			return nil, ErrSocialIdentity
		}
		if subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(state)) != 1 {
			return nil, ErrSocialIdentity
		}
		var claims struct {
			Email           string `json:"email"`
			Verified        bool   `json:"email_verified"`
			AuthorizedParty string `json:"azp"`
		}
		if verified.Claims(&claims) != nil || !claims.Verified || (claims.AuthorizedParty != "" && claims.AuthorizedParty != p.ClientID) || (len(verified.Audience) > 1 && claims.AuthorizedParty != p.ClientID) {
			return nil, ErrSocialIdentity
		}
		identity.Subject = verified.Subject
		identity.Email = claims.Email
	} else {
		if tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "bearer") {
			return nil, ErrSocialIdentity
		}
		var user struct {
			ID int64 `json:"id"`
		}
		if err := s.providerJSON(ctx, http.MethodGet, "https://api.github.com/user", tokens.AccessToken, nil, &user); err != nil || user.ID <= 0 {
			return nil, ErrSocialIdentity
		}
		var emails []struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		if err := s.providerJSON(ctx, http.MethodGet, "https://api.github.com/user/emails", tokens.AccessToken, nil, &emails); err != nil {
			return nil, err
		}
		identity.Subject = strconv.FormatInt(user.ID, 10)
		for _, email := range emails {
			if email.Primary && email.Verified {
				identity.Email = email.Email
				break
			}
		}
	}
	address, err := mail.ParseAddress(identity.Email)
	if err != nil || address.Address != identity.Email || len(identity.Email) > 255 || identity.Subject == "" || len(identity.Subject) > 255 {
		return nil, ErrSocialIdentity
	}
	return identity, nil
}
