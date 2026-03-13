package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
)

func TestKeyRotationServiceNew(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	svc := NewKeyRotationService(nil, nil, nil, cryptoSvc)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.cryptoSvc != cryptoSvc {
		t.Error("crypto service not set correctly")
	}
}

func TestRotationResultFields(t *testing.T) {
	result := RotationResult{
		ProjectID:        uuid.New(),
		SecretsRotated:   15,
		EnvironmentsUsed: 3,
	}

	if result.SecretsRotated != 15 {
		t.Errorf("SecretsRotated = %d, want 15", result.SecretsRotated)
	}
	if result.EnvironmentsUsed != 3 {
		t.Errorf("EnvironmentsUsed = %d, want 3", result.EnvironmentsUsed)
	}
	if result.ProjectID == uuid.Nil {
		t.Error("ProjectID should not be nil")
	}
}

func TestKeyRotationEncryptDecryptCycle(t *testing.T) {
	// Simulate the crypto operations that happen during key rotation
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 5)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	// Generate and encrypt old DEK
	oldDEK, err := cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK() error = %v", err)
	}

	encryptedOldDEK, oldDEKNonce, err := cryptoSvc.EncryptDEK(oldDEK)
	if err != nil {
		t.Fatalf("EncryptDEK() error = %v", err)
	}

	// Encrypt a secret with old DEK
	plaintext := []byte("super-secret-value-123")
	secretCiphertext, secretNonce, err := crypto.Encrypt(oldDEK, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Simulate rotation: decrypt old DEK
	decryptedOldDEK, err := cryptoSvc.DecryptDEK(encryptedOldDEK, oldDEKNonce)
	if err != nil {
		t.Fatalf("DecryptDEK() error = %v", err)
	}

	// Decrypt secret with old DEK
	decryptedSecret, err := crypto.Decrypt(decryptedOldDEK, secretCiphertext, secretNonce)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Generate new DEK
	newDEK, err := cryptoSvc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK() error = %v", err)
	}

	// Re-encrypt secret with new DEK
	newCiphertext, newNonce, err := crypto.Encrypt(newDEK, decryptedSecret)
	if err != nil {
		t.Fatalf("Encrypt() with new DEK error = %v", err)
	}

	// Encrypt new DEK with master key
	encryptedNewDEK, newDEKNonce, err := cryptoSvc.EncryptDEK(newDEK)
	if err != nil {
		t.Fatalf("EncryptDEK() error = %v", err)
	}

	// Verify: decrypt new DEK, then decrypt secret
	finalDEK, err := cryptoSvc.DecryptDEK(encryptedNewDEK, newDEKNonce)
	if err != nil {
		t.Fatalf("DecryptDEK() error = %v", err)
	}

	finalPlaintext, err := crypto.Decrypt(finalDEK, newCiphertext, newNonce)
	if err != nil {
		t.Fatalf("Decrypt() with new DEK error = %v", err)
	}

	if string(finalPlaintext) != string(plaintext) {
		t.Errorf("after rotation, got %q, want %q", string(finalPlaintext), string(plaintext))
	}
}

func TestKeyRotationDEKsAreDifferent(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 10)
	}

	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("failed to create crypto service: %v", err)
	}

	dek1, _ := cryptoSvc.GenerateDEK()
	dek2, _ := cryptoSvc.GenerateDEK()

	if string(dek1) == string(dek2) {
		t.Error("consecutive DEKs should be different")
	}
}
