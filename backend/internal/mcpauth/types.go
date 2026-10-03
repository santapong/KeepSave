// Package mcpauth owns resource-bound OAuth. Harness identities are public
// routing identifiers, never evidence of device trust or provider credentials.
package mcpauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"net/url"
	"strings"
	"time"
)

const Scope = "keepsave:tools"

var (
	ErrInvalid     = errors.New("invalid OAuth request")
	ErrDenied      = errors.New("OAuth access denied")
	ErrUnavailable = errors.New("OAuth authority unavailable")
)

type Config struct {
	Issuer, Resource, AppURL string
	AllowedOrigins           []string
}

func (c Config) Validate() error {
	for _, raw := range []string{c.Issuer, c.Resource, c.AppURL} {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(raw, "/") {
			return ErrInvalid
		}
	}
	issuer, _ := url.Parse(c.Issuer)
	resource, _ := url.Parse(c.Resource)
	app, _ := url.Parse(c.AppURL)
	if issuer.Host != app.Host || resource.Host != app.Host || app.Path != "" || resource.Path != "/mcp" {
		return ErrInvalid
	}
	if c.Issuer != c.AppURL {
		return ErrInvalid
	}
	for _, raw := range c.AllowedOrigins {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return ErrInvalid
		}
	}
	return nil
}

type SessionAuthority interface {
	RequireActiveTx(context.Context, *sql.Tx, uuid.UUID, uuid.UUID) error
	RequireRecentTx(context.Context, *sql.Tx, uuid.UUID, uuid.UUID) error
}
type Principal struct {
	UserID, SessionID, FamilyID, TokenID uuid.UUID
	ClientID, Scope                      string
	ExpiresAt                            time.Time
}
type AuthorizationRequest struct {
	ID          uuid.UUID `json:"request_id"`
	ClientID    string    `json:"client_id"`
	Harness     string    `json:"harness"`
	Resource    string    `json:"resource"`
	RedirectURI string    `json:"redirect_uri"`
	Scope       string    `json:"scope"`
	State       string    `json:"-"`
	Challenge   string    `json:"-"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type TokenRequest struct{ GrantType, ClientID, Code, RedirectURI, Verifier, RefreshToken, Resource, Scope string }
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func token() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func canonicalScope(s string) (string, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ErrInvalid
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if (f != Scope && f != "offline_access") || seen[f] {
			return "", ErrInvalid
		}
		seen[f] = true
	}
	if !seen[Scope] {
		return "", ErrInvalid
	}
	if seen["offline_access"] {
		return Scope + " offline_access", nil
	}
	return Scope, nil
}
func validChallenge(s string) bool {
	b, e := base64.RawURLEncoding.DecodeString(s)
	return e == nil && len(b) == 32 && len(s) == 43
}
func validVerifier(s string) bool {
	if len(s) < 43 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-._~", r)) {
			return false
		}
	}
	return true
}
