package auth

import (
	"crypto/x509"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
)

type memKeyStore struct{ keys []StoredKey }

func (m *memKeyStore) ListKeys() ([]StoredKey, error) { return m.keys, nil }
func (m *memKeyStore) InsertKey(k StoredKey) error    { m.keys = append(m.keys, k); return nil }
func (m *memKeyStore) SetKeyStatus(kid, status string) error {
	for i := range m.keys {
		if m.keys[i].Kid == kid {
			m.keys[i].Status = status
		}
	}
	return nil
}

func newRS256Service(t *testing.T, verifyAlgs ...string) (*JWTService, *Keystore) {
	t.Helper()
	mk := make([]byte, 32)
	for i := range mk {
		mk[i] = byte(i + 9)
	}
	cs, err := crypto.NewService(mk)
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	ks, err := LoadOrInit(cs, &memKeyStore{})
	if err != nil {
		t.Fatalf("LoadOrInit: %v", err)
	}
	svc := NewJWTService("hs256-secret")
	svc.EnableRS256(ks, verifyAlgs)
	return svc, ks
}

func TestRS256_RoundTrip(t *testing.T) {
	svc, _ := newRS256Service(t)
	uid := uuid.New()
	tok, err := svc.GenerateToken(uid, "a@b.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	claims, err := svc.ValidateToken(tok)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != uid {
		t.Errorf("UserID = %v, want %v", claims.UserID, uid)
	}
	// The token must actually be RS256.
	parsed, _, _ := jwt.NewParser().ParseUnverified(tok, &Claims{})
	if parsed.Method.Alg() != "RS256" {
		t.Errorf("alg = %s, want RS256", parsed.Method.Alg())
	}
}

func TestRS256_RejectsNoneAlg(t *testing.T) {
	svc, _ := newRS256Service(t)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{Email: "x"}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := svc.ValidateToken(tok); err == nil {
		t.Error("alg=none token was accepted")
	}
}

// TestRS256_RejectsAlgConfusion: an attacker signs an HS256 token using the RSA
// public key bytes as the HMAC secret. Even with HS256 in the allowlist
// (cutover), it must be rejected because verification uses the server secret,
// never the public key.
func TestRS256_RejectsAlgConfusion(t *testing.T) {
	svc, ks := newRS256Service(t, "RS256", "HS256")
	kid, priv, _ := ks.SigningKey()
	_ = kid
	pubDER := x509.MarshalPKCS1PublicKey(&priv.PublicKey)
	forged := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{Email: "attacker"})
	forgedStr, err := forged.SignedString(pubDER)
	if err != nil {
		t.Fatalf("forge: %v", err)
	}
	if _, err := svc.ValidateToken(forgedStr); err == nil {
		t.Error("algorithm-confusion token was accepted")
	}
}

func TestRS256_PostCutoverRejectsHS256(t *testing.T) {
	svc, _ := newRS256Service(t, "RS256") // HS256 not allowed
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{Email: "x"})
	tok, _ := hs.SignedString([]byte("hs256-secret"))
	if _, err := svc.ValidateToken(tok); err == nil {
		t.Error("HS256 token accepted after cutover (RS256-only)")
	}
}

func TestRS256_CutoverAcceptsHS256(t *testing.T) {
	svc, _ := newRS256Service(t, "RS256", "HS256")
	hs := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: uuid.New(), Email: "x",
		RegisteredClaims: jwt.RegisteredClaims{},
	})
	tok, _ := hs.SignedString([]byte("hs256-secret"))
	if _, err := svc.ValidateToken(tok); err != nil {
		t.Errorf("HS256 token rejected during cutover: %v", err)
	}
}

func TestJWKS_PublicKeySet(t *testing.T) {
	_, ks := newRS256Service(t)
	set := ks.PublicKeySet()
	if len(set) != 1 {
		t.Fatalf("JWKS has %d keys, want 1", len(set))
	}
	jwk := set[0]
	if jwk.Kty != "RSA" || jwk.Alg != "RS256" || jwk.Use != "sig" || jwk.Kid == "" || jwk.N == "" || jwk.E == "" {
		t.Errorf("malformed JWK: %+v", jwk)
	}
}
