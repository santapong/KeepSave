package crypto

import (
	"bytes"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	mk := make([]byte, 32)
	for i := range mk {
		mk[i] = byte(i*7 + 1)
	}
	svc, err := NewService(mk)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestServiceSecret_RoundTrip(t *testing.T) {
	svc := newTestService(t)
	plain := []byte("sso-client-secret-xyz")
	ct, nonce, err := svc.EncryptServiceSecret(plain)
	if err != nil {
		t.Fatalf("EncryptServiceSecret: %v", err)
	}
	if bytes.Contains(ct, plain) {
		t.Fatal("ciphertext contains plaintext")
	}
	got, err := svc.DecryptServiceSecret(ct, nonce)
	if err != nil {
		t.Fatalf("DecryptServiceSecret: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip = %q, want %q", got, plain)
	}
}

// TestServiceSecret_NotMasterKey proves the derived sub-key is distinct from the
// raw master key: a blob sealed under the master key is NOT what EncryptServiceSecret
// produces, and the service-secret key does not equal the master key.
func TestServiceSecret_DerivedKeyDiffersFromMaster(t *testing.T) {
	svc := newTestService(t)
	if bytes.Equal(svc.deriveSubKey(serviceSecretLabel), svc.masterKey) {
		t.Fatal("derived service key equals master key")
	}
}

// TestServiceSecret_LegacyFallback ensures blobs written before ADR-0018 (sealed
// directly under the master key) still decrypt via the fallback path.
func TestServiceSecret_LegacyFallback(t *testing.T) {
	svc := newTestService(t)
	plain := []byte("legacy-secret")
	// Simulate the pre-ADR-0018 scheme: encrypt directly under the master key.
	ct, nonce, err := encrypt(svc.masterKey, plain)
	if err != nil {
		t.Fatalf("legacy encrypt: %v", err)
	}
	got, err := svc.DecryptServiceSecret(ct, nonce)
	if err != nil {
		t.Fatalf("DecryptServiceSecret(legacy): %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("legacy fallback = %q, want %q", got, plain)
	}
}

func TestServiceSecret_RejectsTamper(t *testing.T) {
	svc := newTestService(t)
	ct, nonce, err := svc.EncryptServiceSecret([]byte("tamper-me"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	tampered := append([]byte(nil), ct...)
	tampered[0] ^= 0x01
	if _, err := svc.DecryptServiceSecret(tampered, nonce); err == nil {
		t.Error("DecryptServiceSecret accepted tampered ciphertext")
	}
}

func TestDeriveAuditChainKey(t *testing.T) {
	svc := newTestService(t)
	k := svc.DeriveAuditChainKey()
	if len(k) != 32 {
		t.Fatalf("key length = %d, want 32", len(k))
	}
	if !bytes.Equal(k, svc.DeriveAuditChainKey()) {
		t.Error("DeriveAuditChainKey not deterministic")
	}
	if bytes.Equal(k, svc.masterKey) {
		t.Error("audit chain key equals master key")
	}
	if bytes.Equal(k, svc.deriveSubKey(serviceSecretLabel)) {
		t.Error("audit chain key not domain-separated from the service-secret key")
	}
}

func TestSecureZero(t *testing.T) {
	b := []byte("super-secret-key-material")
	SecureZero(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("byte %d = %d, want 0", i, v)
		}
	}
}
