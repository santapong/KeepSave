package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

var ErrSocialUnavailable = errors.New("provider unavailable")
var ErrSocialIdentity = errors.New("provider identity could not be verified")
var pkceChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var pkceVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var socialStatePattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type SocialAuthService struct {
	config         config.SocialAuth
	repo           *repository.SocialAuthRepository
	audit          *repository.AuditRepository
	jwt            *auth.JWTService
	client         *http.Client
	googleVerifier *oidc.IDTokenVerifier
	sessions       *SessionService
}

func (s *SocialAuthService) EnableSessions(v *SessionService) { s.sessions = v }

type SocialStart struct {
	URL   string `json:"authorization_url"`
	State string `json:"state"`
}
type SocialIdentity struct {
	Subject string
	Email   string
}

func NewSocialAuthService(c config.SocialAuth, repo *repository.SocialAuthRepository, audit *repository.AuditRepository, jwt *auth.JWTService) *SocialAuthService {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx := oidc.ClientContext(context.Background(), client)
	keys := oidc.NewRemoteKeySet(ctx, "https://www.googleapis.com/oauth2/v3/certs")
	return &SocialAuthService{config: c, repo: repo, audit: audit, jwt: jwt, client: client,
		googleVerifier: oidc.NewVerifier("https://accounts.google.com", keys, &oidc.Config{ClientID: c.Google.ClientID})}
}
func (s *SocialAuthService) provider(name string) (config.SocialProvider, error) {
	var p config.SocialProvider
	switch name {
	case "github":
		p = s.config.GitHub
	case "google":
		p = s.config.Google
	default:
		return p, ErrSocialUnavailable
	}
	if p.ClientID == "" || p.ClientSecret == "" || s.config.Origin == "" {
		return p, ErrSocialUnavailable
	}
	return p, nil
}
func (s *SocialAuthService) Providers() map[string]bool {
	result := map[string]bool{}
	for _, name := range []string{"github", "google"} {
		_, err := s.provider(name)
		result[name] = err == nil
	}
	return result
}
func socialHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func (s *SocialAuthService) callback(provider string) string {
	return s.config.Origin + "/auth/callback/" + provider
}
func (s *SocialAuthService) auditEvent(user *uuid.UUID, action, provider, ip string) error {
	if s.audit == nil {
		return errors.New("social auth audit unavailable")
	}
	return s.audit.Create(user, nil, action, "", models.JSONMap{"provider": provider}, ip)
}
func (s *SocialAuthService) Start(ctx context.Context, provider, challenge, ip string, linkUser *uuid.UUID) (*SocialStart, error) {
	return s.StartWithSession(ctx, provider, challenge, ip, linkUser, nil)
}
func (s *SocialAuthService) StartWithSession(ctx context.Context, provider, challenge, ip string, linkUser, linkSession *uuid.UUID) (*SocialStart, error) {
	p, err := s.provider(provider)
	if err != nil {
		return nil, err
	}
	if !pkceChallengePattern.MatchString(challenge) {
		return nil, repository.ErrSocialFlow
	}
	if linkUser != nil && s.sessions != nil {
		if linkSession == nil {
			return nil, repository.ErrRecentAuthentication
		}
		if err = s.sessions.RequireRecent(ctx, *linkUser, *linkSession); err != nil {
			return nil, err
		}
	}
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return nil, err
	}
	state := hex.EncodeToString(bytes)
	if err = s.auditEvent(linkUser, "auth.social_started", provider, ip); err != nil {
		return nil, err
	}
	if err = s.repo.CreateFlowForSession(ctx, socialHash(state), provider, challenge, linkUser, linkSession); err != nil {
		return nil, err
	}
	values := url.Values{"client_id": {p.ClientID}, "redirect_uri": {s.callback(provider)}, "state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "response_type": {"code"}}
	endpoint := "https://github.com/login/oauth/authorize"
	if provider == "google" {
		endpoint = "https://accounts.google.com/o/oauth2/v2/auth"
		values.Set("scope", "openid email")
		values.Set("nonce", state)
		values.Set("prompt", "select_account")
	} else {
		values.Set("scope", "read:user user:email")
	}
	return &SocialStart{URL: endpoint + "?" + values.Encode(), State: state}, nil
}
func (s *SocialAuthService) Complete(ctx context.Context, provider, code, state, verifier, ip string, linkUser *uuid.UUID) (*AuthResponse, error) {
	return s.CompleteWithSession(ctx, provider, code, state, verifier, ip, linkUser, nil, "")
}
func (s *SocialAuthService) CompleteWithSession(ctx context.Context, provider, code, state, verifier, ip string, linkUser, linkSession *uuid.UUID, ua string) (*AuthResponse, error) {
	p, err := s.provider(provider)
	if err != nil {
		return nil, err
	}
	if code == "" || len(code) > 4096 || !socialStatePattern.MatchString(state) || !pkceVerifierPattern.MatchString(verifier) {
		return nil, repository.ErrSocialFlow
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if linkUser != nil && s.sessions != nil {
		if linkSession == nil {
			return nil, repository.ErrRecentAuthentication
		}
		if err = s.sessions.RequireRecent(ctx, *linkUser, *linkSession); err != nil {
			return nil, err
		}
	}
	if err = s.repo.ConsumeFlowForSession(ctx, socialHash(state), provider, challenge, linkUser, linkSession); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	identity, err := s.exchange(ctx, provider, p, code, state, verifier)
	if err != nil {
		if auditErr := s.auditEvent(linkUser, "auth.login_failed", provider, ip); auditErr != nil {
			return nil, auditErr
		}
		return nil, ErrSocialIdentity
	}
	// Record the intent before creating an identity, then the result before issuing a JWT.
	if err = s.auditEvent(linkUser, "auth.social_verified", provider, ip); err != nil {
		return nil, err
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if linkUser != nil && s.sessions != nil {
		if err = s.sessions.RequireRecentTx(ctx, tx, *linkUser, *linkSession); err != nil {
			return nil, err
		}
	}
	user, err := s.repo.ResolveTx(ctx, tx, provider, identity.Subject, identity.Email, linkUser)
	if err != nil {
		return nil, err
	}
	action := "auth.login"
	if linkUser != nil {
		action = "auth.social_linked"
	}
	var token string
	if linkUser == nil {
		if s.sessions != nil {
			token, err = s.sessions.IssueTx(ctx, tx, user, provider, ip, ua)
		} else {
			token, err = s.jwt.GenerateToken(user.ID, user.Email)
		}
	}
	if err != nil {
		return nil, err
	}
	if err = s.audit.CreateTx(tx, &user.ID, nil, action, "", models.JSONMap{"provider": provider}, ip); err != nil {
		return nil, err
	}
	if err = identityEventTx(ctx, tx, s.repo.Dialect(), user.ID, action, map[string]string{"provider": provider}); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &AuthResponse{User: user, Token: token}, nil
}
func (s *SocialAuthService) Connections(ctx context.Context, user uuid.UUID) ([]string, error) {
	return s.repo.Connections(ctx, user)
}
