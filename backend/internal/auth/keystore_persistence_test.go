package auth

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
)

func persistenceCrypto(t *testing.T) *crypto.Service {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	service, err := crypto.NewService(key)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

// Restart must reuse persisted signing material: existing tokens still verify,
// a verifying key remains valid during overlap, and retired keys are rejected.
func TestKeystoreRestartAndRetirement(t *testing.T) {
	cs, storage := persistenceCrypto(t), &memKeyStore{}
	first, err := LoadOrInit(cs, storage)
	if err != nil {
		t.Fatal(err)
	}
	signer := NewJWTService(uuid.NewString())
	signer.EnableRS256(first, []string{"RS256"})
	uid := uuid.New()
	token, err := signer.GenerateToken(uid, "")
	if err != nil {
		t.Fatal(err)
	}
	oldKid := storage.keys[0].Kid

	reloaded, err := LoadOrInit(cs, storage)
	if err != nil {
		t.Fatal(err)
	}
	if len(storage.keys) != 1 {
		t.Fatal("restart created an unnecessary signing key")
	}
	verifier := NewJWTService(uuid.NewString())
	verifier.EnableRS256(reloaded, []string{"RS256"})
	claims, err := verifier.ValidateToken(token)
	if err != nil || claims.UserID != uid {
		t.Fatal("existing token did not survive key reload")
	}

	if err := storage.SetKeyStatus(oldKid, keyStatusVerifying); err != nil {
		t.Fatal(err)
	}
	overlap, err := LoadOrInit(cs, storage)
	if err != nil {
		t.Fatal(err)
	}
	newKid, _, err := overlap.SigningKey()
	if err != nil || newKid == oldKid || len(storage.keys) != 2 {
		t.Fatal("missing replacement signing key")
	}
	verifier.EnableRS256(overlap, []string{"RS256"})
	if _, err := verifier.ValidateToken(token); err != nil {
		t.Fatal("overlap rejected existing token")
	}

	if err := storage.SetKeyStatus(oldKid, "retired"); err != nil {
		t.Fatal(err)
	}
	retired, err := LoadOrInit(cs, storage)
	if err != nil {
		t.Fatal(err)
	}
	verifier.EnableRS256(retired, []string{"RS256"})
	if _, err := verifier.ValidateToken(token); err == nil {
		t.Fatal("retired key still verifies")
	}
	publicKeys := retired.PublicKeySet()
	if len(publicKeys) != 1 || publicKeys[0].Kid != newKid {
		t.Fatal("public set does not contain only the replacement key")
	}
}

func TestKeystoreRejectsCorruptStoredMaterial(t *testing.T) {
	cs, storage := persistenceCrypto(t), &memKeyStore{}
	if _, err := LoadOrInit(cs, storage); err != nil {
		t.Fatal(err)
	}
	original := storage.keys[0]
	for _, mode := range []string{"wrong wrapping key", "altered ciphertext", "invalid private key"} {
		t.Run(mode, func(t *testing.T) {
			stored := original
			service := cs
			switch mode {
			case "wrong wrapping key":
				service = persistenceCrypto(t)
			case "altered ciphertext":
				stored.PrivateKeyEncrypted = append([]byte(nil), original.PrivateKeyEncrypted...)
				stored.PrivateKeyEncrypted[0] ^= 1
			case "invalid private key":
				var err error
				stored.PrivateKeyEncrypted, stored.PrivateKeyNonce, err = cs.EncryptServiceSecret([]byte(uuid.NewString()))
				if err != nil {
					t.Fatal(err)
				}
			}
			copy := &memKeyStore{keys: []StoredKey{stored}}
			ks, err := LoadOrInit(service, copy)
			if err == nil || ks != nil {
				t.Fatal("corrupt stored material was accepted")
			}
			if len(copy.keys) != 1 {
				t.Fatal("corrupt key was silently replaced")
			}
		})
	}
}

type unavailableKeyStorage struct {
	memKeyStore
	failRead bool
}

func (s *unavailableKeyStorage) ListKeys() ([]StoredKey, error) {
	if s.failRead {
		return nil, errors.New("storage unavailable")
	}
	return s.memKeyStore.ListKeys()
}

func (*unavailableKeyStorage) InsertKey(StoredKey) error { return errors.New("storage unavailable") }

func TestKeystoreRequiresSuccessfulStorage(t *testing.T) {
	cs := persistenceCrypto(t)
	for _, failRead := range []bool{true, false} {
		ks, err := LoadOrInit(cs, &unavailableKeyStorage{failRead: failRead})
		if err == nil || ks != nil {
			t.Fatal("keystore started without readable, durable key storage")
		}
	}
}
