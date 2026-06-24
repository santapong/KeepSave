package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
)

// StoredKey is the persisted form of a JWT signing key (ADR-0008). The private
// key is wrapped with the crypto service's service-secret sub-key (ADR-0018),
// so the raw master key never leaves internal/crypto.
type StoredKey struct {
	Kid                 string
	Alg                 string
	PublicKeyDER        []byte
	PrivateKeyEncrypted []byte
	PrivateKeyNonce     []byte
	Status              string // signing | verifying | retired
}

// KeyStorage persists JWT keys. Implemented by repository.JWTKeyRepository in
// production and by an in-memory fake in tests, keeping the auth package free
// of a direct DB dependency.
type KeyStorage interface {
	ListKeys() ([]StoredKey, error)
	InsertKey(StoredKey) error
	SetKeyStatus(kid, status string) error
}

const (
	keyStatusSigning   = "signing"
	keyStatusVerifying = "verifying"
	algRS256           = "RS256"
	rsaKeyBits         = 2048
)

type keyEntry struct {
	kid    string
	alg    string
	priv   *rsa.PrivateKey
	status string
}

// Keystore holds the RSA signing key plus any verifying keys for the overlap
// window (ADR-0008). It serves token signing, kid-based verification, and the
// JWKS public-key set.
type Keystore struct {
	crypto  *crypto.Service
	storage KeyStorage

	mu      sync.RWMutex
	signing *keyEntry
	byKid   map[string]*keyEntry // signing + verifying (never retired)
}

// LoadOrInit loads existing keys and, if none can sign, generates a fresh
// RSA-2048 signing key. It self-heals a missing signing key at startup.
func LoadOrInit(cryptoSvc *crypto.Service, storage KeyStorage) (*Keystore, error) {
	ks := &Keystore{crypto: cryptoSvc, storage: storage, byKid: map[string]*keyEntry{}}
	stored, err := storage.ListKeys()
	if err != nil {
		return nil, fmt.Errorf("listing jwt keys: %w", err)
	}
	for _, sk := range stored {
		if sk.Status == "retired" {
			continue
		}
		der, derr := cryptoSvc.DecryptServiceSecret(sk.PrivateKeyEncrypted, sk.PrivateKeyNonce)
		if derr != nil {
			return nil, fmt.Errorf("decrypting jwt key %s: %w", sk.Kid, derr)
		}
		priv, perr := x509.ParsePKCS1PrivateKey(der)
		if perr != nil {
			return nil, fmt.Errorf("parsing jwt key %s: %w", sk.Kid, perr)
		}
		e := &keyEntry{kid: sk.Kid, alg: sk.Alg, priv: priv, status: sk.Status}
		ks.byKid[sk.Kid] = e
		if sk.Status == keyStatusSigning {
			ks.signing = e
		}
	}
	if ks.signing == nil {
		if _, err := ks.generateSigningKey(); err != nil {
			return nil, err
		}
	}
	return ks, nil
}

func (ks *Keystore) generateSigningKey() (*keyEntry, error) {
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}
	kid := uuid.NewString()
	encPriv, nonce, err := ks.crypto.EncryptServiceSecret(x509.MarshalPKCS1PrivateKey(priv))
	if err != nil {
		return nil, fmt.Errorf("wrapping RSA private key: %w", err)
	}
	sk := StoredKey{
		Kid:                 kid,
		Alg:                 algRS256,
		PublicKeyDER:        x509.MarshalPKCS1PublicKey(&priv.PublicKey),
		PrivateKeyEncrypted: encPriv,
		PrivateKeyNonce:     nonce,
		Status:              keyStatusSigning,
	}
	if err := ks.storage.InsertKey(sk); err != nil {
		return nil, fmt.Errorf("persisting jwt key: %w", err)
	}
	e := &keyEntry{kid: kid, alg: algRS256, priv: priv, status: keyStatusSigning}
	ks.mu.Lock()
	ks.byKid[kid] = e
	ks.signing = e
	ks.mu.Unlock()
	return e, nil
}

// SigningKey returns the current signing key id and private key.
func (ks *Keystore) SigningKey() (string, *rsa.PrivateKey, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if ks.signing == nil {
		return "", nil, fmt.Errorf("no signing key")
	}
	return ks.signing.kid, ks.signing.priv, nil
}

// VerifyKey returns the public key for a kid (signing or verifying), or an
// error for unknown/retired kids.
func (ks *Keystore) VerifyKey(kid string) (*rsa.PublicKey, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	e, ok := ks.byKid[kid]
	if !ok {
		return nil, fmt.Errorf("unknown kid %q", kid)
	}
	return &e.priv.PublicKey, nil
}

// JWK is an RFC 7517 RSA public key.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// PublicKeySet returns the JWKS (signing + verifying keys).
func (ks *Keystore) PublicKeySet() []JWK {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	out := make([]JWK, 0, len(ks.byKid))
	for _, e := range ks.byKid {
		pub := &e.priv.PublicKey
		eBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(eBytes, uint64(pub.E))
		// trim leading zero bytes of the exponent
		i := 0
		for i < len(eBytes)-1 && eBytes[i] == 0 {
			i++
		}
		out = append(out, JWK{
			Kty: "RSA", Use: "sig", Alg: e.alg, Kid: e.kid,
			N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E: base64.RawURLEncoding.EncodeToString(eBytes[i:]),
		})
	}
	return out
}
