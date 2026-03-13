package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

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

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}
