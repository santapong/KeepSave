package crypto

import (
	"bytes"
	"testing"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name      string
		keyLen    int
		wantError bool
	}{
		{"valid 32-byte key", 32, false},
		{"too short key", 16, true},
		{"too long key", 64, true},
		{"empty key", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)
			_, err := NewService(key)
			if (err != nil) != tt.wantError {
				t.Errorf("NewService() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "hello world"},
		{"empty string", ""},
		{"special characters", "p@$$w0rd!#%^&*()"},
		{"long string", "this is a much longer string that contains multiple words and should still encrypt and decrypt correctly"},
		{"unicode", "こんにちは世界 🌍"},
		{"json value", `{"key": "value", "nested": {"a": 1}}`},
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, nonce, err := Encrypt(key, []byte(tt.plaintext))
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			if len(nonce) == 0 {
				t.Fatal("nonce should not be empty")
			}

			if tt.plaintext != "" && bytes.Equal(ciphertext, []byte(tt.plaintext)) {
				t.Fatal("ciphertext should differ from plaintext")
			}

			decrypted, err := Decrypt(key, ciphertext, nonce)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	ciphertext, nonce, err := Encrypt(key1, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	_, err = Decrypt(key2, ciphertext, nonce)
	if err == nil {
		t.Fatal("Decrypt() should fail with wrong key")
	}
}

func TestDecryptWithWrongNonce(t *testing.T) {
	key := make([]byte, 32)

	ciphertext, nonce, err := Encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Modify nonce
	badNonce := make([]byte, len(nonce))
	copy(badNonce, nonce)
	badNonce[0] ^= 0xFF

	_, err = Decrypt(key, ciphertext, badNonce)
	if err == nil {
		t.Fatal("Decrypt() should fail with wrong nonce")
	}
}

func TestUniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("same data")

	_, nonce1, _ := Encrypt(key, plaintext)
	_, nonce2, _ := Encrypt(key, plaintext)

	if bytes.Equal(nonce1, nonce2) {
		t.Fatal("consecutive encryptions should produce different nonces")
	}
}

func TestEnvelopeEncryption(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 10)
	}

	svc, err := NewService(masterKey)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Generate DEK
	dek, err := svc.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK() error = %v", err)
	}
	if len(dek) != 32 {
		t.Fatalf("DEK length = %d, want 32", len(dek))
	}

	// Encrypt DEK with master key
	encryptedDEK, dekNonce, err := svc.EncryptDEK(dek)
	if err != nil {
		t.Fatalf("EncryptDEK() error = %v", err)
	}

	// Decrypt DEK
	decryptedDEK, err := svc.DecryptDEK(encryptedDEK, dekNonce)
	if err != nil {
		t.Fatalf("DecryptDEK() error = %v", err)
	}

	if !bytes.Equal(dek, decryptedDEK) {
		t.Fatal("decrypted DEK does not match original")
	}

	// Use DEK to encrypt data
	plaintext := []byte("my secret value")
	ciphertext, nonce, err := Encrypt(decryptedDEK, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() with DEK error = %v", err)
	}

	// Decrypt data with DEK
	result, err := Decrypt(decryptedDEK, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt() with DEK error = %v", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Errorf("round-trip failed: got %q, want %q", string(result), string(plaintext))
	}
}
