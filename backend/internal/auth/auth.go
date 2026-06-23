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
	jwt.RegisteredClaims
}

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

func (s *JWTService) GenerateToken(userID uuid.UUID, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

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
	return claims, nil
}
