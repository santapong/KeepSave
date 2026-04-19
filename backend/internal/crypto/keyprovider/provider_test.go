package keyprovider

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func validKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestEnvProvider(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(validKey())
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))

	cases := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{"happy", map[string]string{"MASTER_KEY": good}, false},
		{"missing", map[string]string{}, true},
		{"not base64", map[string]string{"MASTER_KEY": "!!not-base64!!"}, true},
		{"wrong length", map[string]string{"MASTER_KEY": short}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewEnvProvider("MASTER_KEY", func(k string) (string, bool) {
				v, ok := tc.env[k]
				return v, ok
			})
			if p.Name() != "env" {
				t.Fatalf("Name() = %q", p.Name())
			}
			key, err := p.GetMasterKey(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got key len %d", len(key))
				}
				return
			}
			if err != nil {
				t.Fatalf("GetMasterKey: %v", err)
			}
			if !bytes.Equal(key, validKey()) {
				t.Fatalf("key mismatch")
			}
		})
	}

	p := NewEnvProvider("MASTER_KEY", func(string) (string, bool) { return "", false })
	if err := p.Rotate(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Rotate err = %v, want ErrUnsupported", err)
	}
}

type fakeAWS struct {
	key    []byte
	err    error
	lastID string
}

func (f *fakeAWS) Decrypt(_ context.Context, keyID string, _ []byte) ([]byte, error) {
	f.lastID = keyID
	return f.key, f.err
}

func TestAWSKMSProvider(t *testing.T) {
	ok := &fakeAWS{key: validKey()}
	p, err := NewAWSKMSProvider(ok, "alias/test", base64.StdEncoding.EncodeToString([]byte("opaque")))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "awskms" {
		t.Fatalf("Name() = %q", p.Name())
	}
	key, err := p.GetMasterKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, validKey()) {
		t.Fatal("key mismatch")
	}
	if ok.lastID != "alias/test" {
		t.Fatalf("keyID not forwarded: %q", ok.lastID)
	}

	bad := &fakeAWS{err: errors.New("access denied")}
	p, _ = NewAWSKMSProvider(bad, "alias/test", base64.StdEncoding.EncodeToString([]byte("opaque")))
	if _, err := p.GetMasterKey(context.Background()); err == nil {
		t.Fatal("expected error")
	}

	if _, err := NewAWSKMSProvider(nil, "x", ""); err == nil {
		t.Fatal("expected nil-client error")
	}
	if _, err := NewAWSKMSProvider(ok, "", ""); err == nil {
		t.Fatal("expected empty-keyID error")
	}
}

type fakeGCP struct {
	key []byte
	err error
}

func (f *fakeGCP) Decrypt(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return f.key, f.err
}

func TestGCPKMSProvider(t *testing.T) {
	p, err := NewGCPKMSProvider(&fakeGCP{key: validKey()},
		"projects/p/locations/l/keyRings/r/cryptoKeys/k",
		base64.StdEncoding.EncodeToString([]byte("opaque")))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "gcpkms" {
		t.Fatalf("Name() = %q", p.Name())
	}
	if _, err := p.GetMasterKey(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type fakeHTTP struct {
	status int
	body   string
}

func (f *fakeHTTP) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

func TestVaultProvider(t *testing.T) {
	plaintext := base64.StdEncoding.EncodeToString(validKey())
	okResp := `{"data":{"plaintext":"` + plaintext + `"}}`

	p, err := NewVaultProvider(&fakeHTTP{status: 200, body: okResp},
		"https://vault.example", "token", "keepsave-master", "vault:v1:xxx")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "vault" {
		t.Fatalf("Name() = %q", p.Name())
	}
	key, err := p.GetMasterKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, validKey()) {
		t.Fatal("key mismatch")
	}

	p, _ = NewVaultProvider(&fakeHTTP{status: 403, body: `{"errors":["denied"]}`},
		"https://vault.example", "token", "k", "vault:v1:xxx")
	if _, err := p.GetMasterKey(context.Background()); err == nil {
		t.Fatal("expected error on non-200")
	}

	if _, err := NewVaultProvider(nil, "", "", "", ""); err == nil {
		t.Fatal("expected error on missing params")
	}
}
