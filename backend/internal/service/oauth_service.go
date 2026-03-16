package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type OAuthService struct {
	oauthRepo *repository.OAuthRepository
	userRepo  *repository.UserRepository
}

func NewOAuthService(oauthRepo *repository.OAuthRepository, userRepo *repository.UserRepository) *OAuthService {
	return &OAuthService{oauthRepo: oauthRepo, userRepo: userRepo}
}

// RegisterClient creates a new OAuth client and returns the raw client secret.
func (s *OAuthService) RegisterClient(name, description string, ownerID uuid.UUID, redirectURIs, scopes, grantTypes []string, logoURL, homepageURL string, isPublic bool) (*models.OAuthClient, string, error) {
	clientID := generateClientID()
	rawSecret := generateClientSecret()
	secretHash := hashSecret(rawSecret)

	if len(scopes) == 0 {
		scopes = []string{"read"}
	}
	if len(grantTypes) == 0 {
		grantTypes = []string{"authorization_code"}
	}

	client := &models.OAuthClient{
		ClientID:         clientID,
		ClientSecretHash: secretHash,
		Name:             name,
		Description:      description,
		OwnerID:          ownerID,
		RedirectURIs:     redirectURIs,
		Scopes:           scopes,
		GrantTypes:        grantTypes,
		LogoURL:          logoURL,
		HomepageURL:      homepageURL,
		IsPublic:         isPublic,
	}

	if err := s.oauthRepo.CreateClient(client); err != nil {
		return nil, "", fmt.Errorf("creating oauth client: %w", err)
	}

	return client, rawSecret, nil
}

func (s *OAuthService) ListClients(ownerID uuid.UUID) ([]models.OAuthClient, error) {
	return s.oauthRepo.ListClientsByOwner(ownerID)
}

func (s *OAuthService) GetClient(clientID string) (*models.OAuthClient, error) {
	return s.oauthRepo.GetClientByClientID(clientID)
}

func (s *OAuthService) DeleteClient(id, ownerID uuid.UUID) error {
	if err := s.oauthRepo.RevokeAllTokensForClient(id); err != nil {
		return fmt.Errorf("revoking tokens: %w", err)
	}
	return s.oauthRepo.DeleteClient(id, ownerID)
}

// Authorize creates an authorization code for the given client and user.
func (s *OAuthService) Authorize(clientID string, userID uuid.UUID, redirectURI string, scopes []string, codeChallenge, codeChallengeMethod string) (string, error) {
	client, err := s.oauthRepo.GetClientByClientID(clientID)
	if err != nil {
		return "", fmt.Errorf("client not found: %w", err)
	}

	if !s.isValidRedirectURI(client, redirectURI) {
		return "", fmt.Errorf("invalid redirect_uri")
	}

	code := generateAuthCode()

	authCode := &models.OAuthAuthorizationCode{
		Code:                code,
		ClientID:            client.ID,
		UserID:              userID,
		RedirectURI:         redirectURI,
		Scopes:              scopes,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		ExpiresAt:           time.Now().Add(10 * time.Minute),
	}

	if err := s.oauthRepo.CreateAuthorizationCode(authCode); err != nil {
		return "", fmt.Errorf("creating authorization code: %w", err)
	}

	return code, nil
}

// ExchangeCode exchanges an authorization code for access and refresh tokens.
func (s *OAuthService) ExchangeCode(code, clientID, clientSecret, redirectURI, codeVerifier string) (*models.OAuthTokenResponse, error) {
	authCode, err := s.oauthRepo.GetAuthorizationCode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization code")
	}

	if authCode.Used {
		return nil, fmt.Errorf("authorization code already used")
	}

	if time.Now().After(authCode.ExpiresAt) {
		return nil, fmt.Errorf("authorization code expired")
	}

	client, err := s.oauthRepo.GetClientByID(authCode.ClientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	if client.ClientID != clientID {
		return nil, fmt.Errorf("client_id mismatch")
	}

	if !client.IsPublic {
		if !s.verifyClientSecret(client, clientSecret) {
			return nil, fmt.Errorf("invalid client_secret")
		}
	}

	if authCode.RedirectURI != redirectURI {
		return nil, fmt.Errorf("redirect_uri mismatch")
	}

	if authCode.CodeChallenge != "" {
		if !s.verifyPKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod, codeVerifier) {
			return nil, fmt.Errorf("invalid code_verifier")
		}
	}

	if err := s.oauthRepo.MarkCodeUsed(authCode.ID); err != nil {
		return nil, fmt.Errorf("marking code used: %w", err)
	}

	return s.issueTokens(client.ID, &authCode.UserID, authCode.Scopes)
}

// ClientCredentialsGrant issues tokens for client_credentials grant type.
func (s *OAuthService) ClientCredentialsGrant(clientID, clientSecret string, scopes []string) (*models.OAuthTokenResponse, error) {
	client, err := s.oauthRepo.GetClientByClientID(clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	if !s.verifyClientSecret(client, clientSecret) {
		return nil, fmt.Errorf("invalid client_secret")
	}

	hasGrant := false
	for _, gt := range client.GrantTypes {
		if gt == "client_credentials" {
			hasGrant = true
			break
		}
	}
	if !hasGrant {
		return nil, fmt.Errorf("client_credentials grant not allowed for this client")
	}

	if len(scopes) == 0 {
		scopes = client.Scopes
	}

	return s.issueTokens(client.ID, nil, scopes)
}

// RefreshToken exchanges a refresh token for new access/refresh tokens.
func (s *OAuthService) RefreshToken(refreshToken, clientID, clientSecret string) (*models.OAuthTokenResponse, error) {
	refreshHash := hashSecret(refreshToken)
	token, err := s.oauthRepo.GetTokenByRefreshHash(refreshHash)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	if token.Revoked {
		return nil, fmt.Errorf("refresh token revoked")
	}

	if token.RefreshExpiresAt != nil && time.Now().After(*token.RefreshExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}

	client, err := s.oauthRepo.GetClientByID(token.ClientID)
	if err != nil {
		return nil, fmt.Errorf("client not found")
	}

	if client.ClientID != clientID {
		return nil, fmt.Errorf("client_id mismatch")
	}

	if !client.IsPublic && !s.verifyClientSecret(client, clientSecret) {
		return nil, fmt.Errorf("invalid client_secret")
	}

	if err := s.oauthRepo.RevokeToken(token.ID); err != nil {
		return nil, fmt.Errorf("revoking old token: %w", err)
	}

	return s.issueTokens(client.ID, token.UserID, token.Scopes)
}

// ValidateAccessToken validates an OAuth access token and returns token info.
func (s *OAuthService) ValidateAccessToken(accessToken string) (*models.OAuthToken, error) {
	accessHash := hashSecret(accessToken)
	token, err := s.oauthRepo.GetTokenByAccessHash(accessHash)
	if err != nil {
		return nil, fmt.Errorf("invalid access token")
	}

	if token.Revoked {
		return nil, fmt.Errorf("token revoked")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return token, nil
}

// RevokeToken revokes an access token.
func (s *OAuthService) RevokeToken(accessToken string) error {
	accessHash := hashSecret(accessToken)
	token, err := s.oauthRepo.GetTokenByAccessHash(accessHash)
	if err != nil {
		return fmt.Errorf("token not found")
	}
	return s.oauthRepo.RevokeToken(token.ID)
}

// GetUserInfo returns user information for the given access token.
func (s *OAuthService) GetUserInfo(accessToken string) (map[string]interface{}, error) {
	token, err := s.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, err
	}

	if token.UserID == nil {
		return map[string]interface{}{
			"sub":         token.ClientID.String(),
			"token_type":  "client_credentials",
			"scopes":      token.Scopes,
		}, nil
	}

	user, err := s.userRepo.GetByID(*token.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return map[string]interface{}{
		"sub":        user.ID.String(),
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"scopes":     token.Scopes,
	}, nil
}

// Internal helpers

func (s *OAuthService) issueTokens(clientID uuid.UUID, userID *uuid.UUID, scopes []string) (*models.OAuthTokenResponse, error) {
	rawAccessToken := generateToken()
	rawRefreshToken := generateToken()

	accessHash := hashSecret(rawAccessToken)
	refreshHash := hashSecret(rawRefreshToken)

	accessExpiry := time.Now().Add(1 * time.Hour)
	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)

	token := &models.OAuthToken{
		AccessTokenHash:  accessHash,
		RefreshTokenHash: refreshHash,
		ClientID:         clientID,
		UserID:           userID,
		Scopes:           scopes,
		TokenType:        "Bearer",
		ExpiresAt:        accessExpiry,
		RefreshExpiresAt: &refreshExpiry,
	}

	if err := s.oauthRepo.CreateToken(token); err != nil {
		return nil, fmt.Errorf("creating token: %w", err)
	}

	return &models.OAuthTokenResponse{
		AccessToken:  rawAccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: rawRefreshToken,
		Scope:        strings.Join(scopes, " "),
	}, nil
}

func (s *OAuthService) isValidRedirectURI(client *models.OAuthClient, uri string) bool {
	for _, allowed := range client.RedirectURIs {
		if allowed == uri {
			return true
		}
	}
	return false
}

func (s *OAuthService) verifyClientSecret(client *models.OAuthClient, secret string) bool {
	hash := hashSecret(secret)
	return subtle.ConstantTimeCompare([]byte(client.ClientSecretHash), []byte(hash)) == 1
}

func (s *OAuthService) verifyPKCE(challenge, method, verifier string) bool {
	if method == "S256" {
		h := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(h[:])
		return subtle.ConstantTimeCompare([]byte(challenge), []byte(computed)) == 1
	}
	// plain method
	return subtle.ConstantTimeCompare([]byte(challenge), []byte(verifier)) == 1
}

func generateClientID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "ks_" + hex.EncodeToString(b)
}

func generateClientSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateAuthCode() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}
