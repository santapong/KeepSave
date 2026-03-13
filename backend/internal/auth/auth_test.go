package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestJWTRoundTrip(t *testing.T) {
	svc := NewJWTService("test-secret")
	userID := uuid.New()
	email := "test@example.com"

	token, err := svc.GenerateToken(userID, email)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Email != email {
		t.Errorf("Email = %v, want %v", claims.Email, email)
	}
}

func TestJWTInvalidToken(t *testing.T) {
	svc := NewJWTService("test-secret")

	_, err := svc.ValidateToken("invalid-token")
	if err == nil {
		t.Fatal("ValidateToken() should fail with invalid token")
	}
}

func TestJWTWrongSecret(t *testing.T) {
	svc1 := NewJWTService("secret-1")
	svc2 := NewJWTService("secret-2")

	token, _ := svc1.GenerateToken(uuid.New(), "test@example.com")
	_, err := svc2.ValidateToken(token)
	if err == nil {
		t.Fatal("ValidateToken() should fail with wrong secret")
	}
}

func TestPasswordHashAndCheck(t *testing.T) {
	password := "mySecurePassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash == password {
		t.Fatal("hash should not equal plaintext")
	}

	if err := CheckPassword(password, hash); err != nil {
		t.Fatalf("CheckPassword() should succeed: %v", err)
	}

	if err := CheckPassword("wrongPassword", hash); err == nil {
		t.Fatal("CheckPassword() should fail with wrong password")
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	rawKey, hashedKey, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	if rawKey[:3] != "ks_" {
		t.Errorf("raw key should start with 'ks_', got %q", rawKey[:3])
	}

	if len(hashedKey) != 64 {
		t.Errorf("hashed key length = %d, want 64", len(hashedKey))
	}

	// Verify hash matches
	computed := HashAPIKey(rawKey)
	if computed != hashedKey {
		t.Error("HashAPIKey should produce the same hash")
	}

	// Generate another key, should be different
	rawKey2, _, _ := GenerateAPIKey()
	if rawKey == rawKey2 {
		t.Error("consecutive keys should be unique")
	}
}
