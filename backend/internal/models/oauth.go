package models

import (
	"time"

	"github.com/google/uuid"
)

// OAuthClient represents a registered OAuth 2.0 client application.
type OAuthClient struct {
	ID               uuid.UUID  `json:"id"`
	ClientID         string     `json:"client_id"`
	ClientSecretHash string     `json:"-"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	OwnerID          uuid.UUID  `json:"owner_id"`
	RedirectURIs     StringList `json:"redirect_uris"`
	Scopes           StringList `json:"scopes"`
	GrantTypes       StringList `json:"grant_types"`
	LogoURL          string     `json:"logo_url,omitempty"`
	HomepageURL      string     `json:"homepage_url,omitempty"`
	IsPublic         bool       `json:"is_public"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// OAuthAuthorizationCode represents a temporary authorization code.
type OAuthAuthorizationCode struct {
	ID                  uuid.UUID  `json:"id"`
	Code                string     `json:"-"`
	ClientID            uuid.UUID  `json:"client_id"`
	UserID              uuid.UUID  `json:"user_id"`
	RedirectURI         string     `json:"redirect_uri"`
	Scopes              StringList `json:"scopes"`
	CodeChallenge       string     `json:"-"`
	CodeChallengeMethod string     `json:"-"`
	ExpiresAt           time.Time  `json:"expires_at"`
	Used                bool       `json:"used"`
	CreatedAt           time.Time  `json:"created_at"`
}

// OAuthToken represents an issued access/refresh token pair.
type OAuthToken struct {
	ID               uuid.UUID  `json:"id"`
	AccessTokenHash  string     `json:"-"`
	RefreshTokenHash string     `json:"-"`
	ClientID         uuid.UUID  `json:"client_id"`
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	Scopes           StringList `json:"scopes"`
	TokenType        string     `json:"token_type"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RefreshExpiresAt *time.Time `json:"refresh_expires_at,omitempty"`
	Revoked          bool       `json:"revoked"`
	CreatedAt        time.Time  `json:"created_at"`
}

// OAuthTokenResponse is the JSON response for token requests.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}
