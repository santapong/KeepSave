package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	// TokenType is "agent" for short-lived agent tokens (ADR-0021); empty for
	// ordinary user tokens. LeaseID is set only on agent tokens and binds the
	// token to the JIT lease it was minted from, enabling lease-cascade
	// revocation. The jti (RegisteredClaims.ID) is the denylist key.
	TokenType string     `json:"token_type,omitempty"`
	LeaseID   *uuid.UUID `json:"lease_id,omitempty"`
	// Agent-token scope (ADR-0021). For TokenType=="agent" these mirror the
	// lease the token was minted from and are the ONLY authority the token
	// carries: the consuming agent may read just SecretKeys, only in ProjectID
	// and Environment. They are embedded at mint because the lease is immutable
	// for its (≤15 min) lifetime; lease revocation/expiry is enforced
	// independently by the denylist in ValidateToken. Empty for user tokens —
	// a user token MUST NOT be treated as project/environment scoped.
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	Environment string     `json:"env,omitempty"`
	SecretKeys  []string   `json:"secret_keys,omitempty"`
	jwt.RegisteredClaims
}

// Denylister reports whether an agent token has been revoked, either explicitly
// by its jti or transitively because the lease it was minted from is no longer
// active (ADR-0021). It is an interface so the auth package stays free of a
// direct DB dependency (mirrors KeyStorage from ADR-0008).
type Denylister interface {
	IsTokenRevoked(jti string, leaseID *uuid.UUID) (bool, error)
}

// maxAgentTokenTTL caps the lifetime of a minted agent token regardless of the
// requested duration or the lease's remaining lifetime (ADR-0021).
const maxAgentTokenTTL = 15 * time.Minute

// MaxAgentTokenTTL exposes the agent-token lifetime cap for callers that need a
// conservative prune horizon (ADR-0021).
func MaxAgentTokenTTL() time.Duration { return maxAgentTokenTTL }

type JWTService struct {
	secret     []byte
	expiration time.Duration

	// RS256 support (ADR-0008). When keystore is non-nil, tokens are signed
	// RS256 with a kid header and verified by kid. algVerify is the allowlist of
	// accepted verification algorithms (e.g. {"RS256","HS256"} during the HS256
	// cutover window, then {"RS256"}). When keystore is nil, the service is
	// HS256-only (legacy behaviour).
	keystore  *Keystore
	algVerify map[string]bool

	// denylist, when set, is consulted by ValidateToken for tokens that carry a
	// jti (agent tokens, ADR-0021). User tokens have no jti and skip it.
	denylist Denylister
}

func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		expiration: 24 * time.Hour,
	}
}

// EnableRS256 attaches a keystore so new tokens are signed RS256, and sets the
// verification allowlist (ADR-0008). verifyAlgs defaults to {"RS256","HS256"}
// when empty (cutover); pass {"RS256"} once HS256 tokens have aged out.
func (s *JWTService) EnableRS256(keystore *Keystore, verifyAlgs []string) {
	s.keystore = keystore
	if len(verifyAlgs) == 0 {
		verifyAlgs = []string{"RS256", "HS256"}
	}
	s.algVerify = map[string]bool{}
	for _, a := range verifyAlgs {
		s.algVerify[a] = true
	}
}

// EnableDenylist attaches a revocation checker consulted by ValidateToken for
// tokens carrying a jti (agent tokens, ADR-0021).
func (s *JWTService) EnableDenylist(d Denylister) {
	s.denylist = d
}

func (s *JWTService) GenerateToken(userID uuid.UUID, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return s.signClaims(claims)
}

// GenerateAgentToken mints a short-lived token bound to a JIT lease (ADR-0021).
// ttl is capped at maxAgentTokenTTL and further clamped to leaseRemaining when
// that is smaller and positive, so a token can never outlive its lease. The
// lease's project, environment and secret keys are embedded as the token's
// scope — they are the only authority the token carries (it never inherits the
// owning user's broader access). It returns the signed token, its jti (the
// denylist key) and the chosen expiry.
func (s *JWTService) GenerateAgentToken(userID uuid.UUID, email string, leaseID, projectID uuid.UUID, environment string, secretKeys []string, ttl, leaseRemaining time.Duration) (string, string, time.Time, error) {
	if ttl <= 0 || ttl > maxAgentTokenTTL {
		ttl = maxAgentTokenTTL
	}
	if leaseRemaining > 0 && leaseRemaining < ttl {
		ttl = leaseRemaining
	}
	jti := uuid.NewString()
	leaseRef := leaseID
	projRef := projectID
	expiresAt := time.Now().Add(ttl)
	claims := &Claims{
		UserID:      userID,
		Email:       email,
		TokenType:   "agent",
		LeaseID:     &leaseRef,
		ProjectID:   &projRef,
		Environment: environment,
		SecretKeys:  secretKeys,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	signed, err := s.signClaims(claims)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, expiresAt, nil
}

// signClaims signs claims with RS256 via the keystore when one is configured,
// falling back to HS256 (legacy). Shared by GenerateToken/GenerateAgentToken.
func (s *JWTService) signClaims(claims *Claims) (string, error) {
	// Prefer RS256 via the keystore; fall back to HS256 when no keystore is set.
	if s.keystore != nil {
		kid, priv, err := s.keystore.SigningKey()
		if err == nil && priv != nil {
			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = kid
			signed, serr := token.SignedString(priv)
			if serr != nil {
				return "", fmt.Errorf("signing token: %w", serr)
			}
			return signed, nil
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return signed, nil
}

func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		alg, _ := token.Header["alg"].(string)

		// Algorithm allowlist (ADR-0008 / RFC 8725). When RS256 is enabled the
		// allowlist is explicit; otherwise only HS256 is accepted. This rejects
		// "none" and any non-allowlisted algorithm up front.
		if s.algVerify != nil {
			if !s.algVerify[alg] {
				return nil, fmt.Errorf("algorithm %q not allowed", alg)
			}
		}

		switch token.Method.(type) {
		case *jwt.SigningMethodRSA:
			// Verify by kid against the keystore's public keys. Returning the
			// RSA public key (never the HMAC secret) is the algorithm-confusion
			// guard: an HS256 token forged with the public key as the HMAC
			// secret takes the HMAC branch below and fails against s.secret.
			if s.keystore == nil {
				return nil, fmt.Errorf("RS256 token but no keystore configured")
			}
			kid, _ := token.Header["kid"].(string)
			if kid == "" {
				return nil, fmt.Errorf("RS256 token missing kid")
			}
			pub, kerr := s.keystore.VerifyKey(kid)
			if kerr != nil {
				return nil, kerr
			}
			return pub, nil
		case *jwt.SigningMethodHMAC:
			// Only reachable when HS256 is allowlisted (cutover window) or the
			// service is HS256-only.
			if s.algVerify != nil && !s.algVerify["HS256"] {
				return nil, fmt.Errorf("HS256 not allowed")
			}
			return s.secret, nil
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Revocation check (ADR-0021): only agent tokens carry a jti, so user tokens
	// skip the denylist entirely (no hot-path cost). A revoked jti or a
	// revoked/expired owning lease invalidates the token.
	if s.denylist != nil && claims.ID != "" {
		revoked, derr := s.denylist.IsTokenRevoked(claims.ID, claims.LeaseID)
		if derr != nil {
			return nil, fmt.Errorf("checking token revocation: %w", derr)
		}
		if revoked {
			return nil, fmt.Errorf("token revoked")
		}
	}
	return claims, nil
}
