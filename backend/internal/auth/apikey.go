package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

const apiKeyPrefix = "ks_"

// GenerateAPIKey generates a new API key with a ks_ prefix.
// Returns the raw key (shown once to the user) and the SHA-256 hash (stored in DB).
func GenerateAPIKey() (rawKey string, hashedKey string, err error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", fmt.Errorf("generating api key: %w", err)
	}

	rawKey = apiKeyPrefix + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(rawKey))
	hashedKey = hex.EncodeToString(hash[:])

	return rawKey, hashedKey, nil
}

// HashAPIKey computes the SHA-256 hash of a raw API key for lookup.
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}
