package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// serviceSecretLabel domain-separates the sub-key used to encrypt service-level
// secrets (SSO client secrets, backup blobs) from the master key's DEK-wrapping
// role. See ADR-0018.
const serviceSecretLabel = "keepsave/service-secret/v1"

// Service provides AES-256-GCM envelope encryption operations.
type Service struct {
	masterKey []byte
}

// NewService creates a new crypto service with the given 32-byte master key.
func NewService(masterKey []byte) (*Service, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}
	return &Service{masterKey: masterKey}, nil
}

// GenerateDEK generates a random 32-byte data encryption key.
func (s *Service) GenerateDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("generating DEK: %w", err)
	}
	return dek, nil
}

// EncryptDEK encrypts a DEK using the master key. Returns (ciphertext, nonce, error).
func (s *Service) EncryptDEK(dek []byte) ([]byte, []byte, error) {
	return encrypt(s.masterKey, dek)
}

// DecryptDEK decrypts a DEK using the master key.
func (s *Service) DecryptDEK(ciphertext, nonce []byte) ([]byte, error) {
	return decrypt(s.masterKey, ciphertext, nonce)
}

// deriveSubKey returns a 32-byte key derived from the master key for a given
// label, using HMAC-SHA256 as a one-step KDF. This keeps the master key inside
// this package and domain-separates derived keys from DEK wrapping.
func (s *Service) deriveSubKey(label string) []byte {
	m := hmac.New(sha256.New, s.masterKey)
	m.Write([]byte(label))
	return m.Sum(nil)
}

// auditChainLabel domain-separates the audit hash-chain key (ADR-0019).
const auditChainLabel = "keepsave/audit-chain/v1"

// DeriveAuditChainKey returns the keyed-hash key for the tamper-evident audit
// chain (ADR-0019). It is derived from the master key so the raw key never
// leaves this package and an attacker who can write audit rows still cannot
// forge valid chain hashes.
func (s *Service) DeriveAuditChainKey() []byte {
	return s.deriveSubKey(auditChainLabel)
}

// EncryptServiceSecret encrypts a service-level secret (e.g. an SSO client
// secret or a backup blob) under a master-key-derived sub-key, so callers never
// handle the raw master key (ADR-0018, C-02).
func (s *Service) EncryptServiceSecret(plaintext []byte) ([]byte, []byte, error) {
	return encrypt(s.deriveSubKey(serviceSecretLabel), plaintext)
}

// DecryptServiceSecret decrypts a service secret. It first tries the derived
// sub-key, then falls back to the legacy raw-master-key scheme so blobs written
// before ADR-0018 still decrypt (and are upgraded on next write).
func (s *Service) DecryptServiceSecret(ciphertext, nonce []byte) ([]byte, error) {
	if pt, err := decrypt(s.deriveSubKey(serviceSecretLabel), ciphertext, nonce); err == nil {
		return pt, nil
	}
	return decrypt(s.masterKey, ciphertext, nonce)
}

// SecureZero overwrites b with zeros. Call it on decrypted DEKs and plaintext
// secret values once they are no longer needed, to shorten their lifetime in
// memory. Best-effort under a managed runtime (ADR-0018, H-04).
func SecureZero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Encrypt encrypts plaintext using the given key. Returns (ciphertext, nonce, error).
func Encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	return encrypt(key, plaintext)
}

// Decrypt decrypts ciphertext using the given key and nonce.
func Decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	return decrypt(key, ciphertext, nonce)
}

func encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("creating cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	// GCM Open panics on a wrong-length nonce; guard so corrupt or
	// attacker-supplied ciphertext yields an error, not a crash.
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("invalid nonce length: got %d, want %d", len(nonce), aead.NonceSize())
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}
