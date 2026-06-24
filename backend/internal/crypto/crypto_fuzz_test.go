package crypto

import (
	"bytes"
	"testing"
)

// FuzzEncryptDecryptRoundTrip checks that any plaintext survives an
// encrypt→decrypt cycle under a valid 32-byte key.
func FuzzEncryptDecryptRoundTrip(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte(""))
	f.Add([]byte{0x00, 0xff, 0x10})
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i * 3)
	}
	f.Fuzz(func(t *testing.T, plaintext []byte) {
		ct, nonce, err := Encrypt(key, plaintext)
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		got, err := Decrypt(key, ct, nonce)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("round-trip mismatch: got %x want %x", got, plaintext)
		}
	})
}

// FuzzDecryptNoPanic feeds arbitrary key/ciphertext/nonce and asserts Decrypt
// never panics (it may error) — regression guard for the nonce-length check.
func FuzzDecryptNoPanic(f *testing.F) {
	f.Add([]byte("0123456789012345678901234567890123"), []byte("ciphertext-bytes"), []byte("123456789012"))
	f.Add([]byte{}, []byte{}, []byte{})
	f.Fuzz(func(t *testing.T, key, ct, nonce []byte) {
		_, _ = Decrypt(key, ct, nonce) // must not panic
	})
}
