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

// flakyHTTP fails the first failN calls (transport error unless status5xx is
// set, in which case a 5xx response) then serves body with status 200.
type flakyHTTP struct {
	failN     int
	calls     int
	body      string
	status5xx bool
}

func (f *flakyHTTP) Do(*http.Request) (*http.Response, error) {
	f.calls++
	if f.calls <= f.failN {
		if f.status5xx {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Body:       io.NopCloser(strings.NewReader(`{"errors":["sealed"]}`)),
				Header:     make(http.Header),
			}, nil
		}
		return nil, errors.New("connection reset by peer")
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

func TestVaultProviderRetryRecovers(t *testing.T) {
	// Keep the test fast: tiny backoff, no waiting on real delays.
	t.Setenv("KEEPSAVE_VAULT_MAX_ATTEMPTS", "4")
	t.Setenv("KEEPSAVE_VAULT_RETRY_BASE_DELAY", "1ms")
	t.Setenv("KEEPSAVE_VAULT_RETRY_MAX_DELAY", "2ms")
	t.Setenv("KEEPSAVE_VAULT_RETRY_BUDGET", "5s")

	okResp := `{"data":{"plaintext":"` + base64.StdEncoding.EncodeToString(validKey()) + `"}}`

	// Transient transport errors then success.
	transientClient := &flakyHTTP{failN: 2, body: okResp}
	p, err := NewVaultProvider(transientClient, "https://vault.example", "token", "k", "vault:v1:xxx")
	if err != nil {
		t.Fatal(err)
	}
	key, err := p.GetMasterKey(context.Background())
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if !bytes.Equal(key, validKey()) {
		t.Fatal("key mismatch after retry")
	}
	if transientClient.calls != 3 {
		t.Fatalf("expected 3 calls (2 fail + 1 ok), got %d", transientClient.calls)
	}

	// Transient 5xx then success is also retried.
	fiveXX := &flakyHTTP{failN: 3, status5xx: true, body: okResp}
	p, _ = NewVaultProvider(fiveXX, "https://vault.example", "token", "k", "vault:v1:xxx")
	if _, err := p.GetMasterKey(context.Background()); err != nil {
		t.Fatalf("expected recovery after 5xx, got %v", err)
	}
	if fiveXX.calls != 4 {
		t.Fatalf("expected 4 calls (3 fail + 1 ok), got %d", fiveXX.calls)
	}
}

func TestVaultProviderRetryExhausts(t *testing.T) {
	t.Setenv("KEEPSAVE_VAULT_MAX_ATTEMPTS", "3")
	t.Setenv("KEEPSAVE_VAULT_RETRY_BASE_DELAY", "1ms")
	t.Setenv("KEEPSAVE_VAULT_RETRY_MAX_DELAY", "2ms")
	t.Setenv("KEEPSAVE_VAULT_RETRY_BUDGET", "5s")

	// Persistent transport failure: never recovers, must still error after the
	// bound and must have tried exactly maxAttempts times.
	persistent := &flakyHTTP{failN: 100}
	p, err := NewVaultProvider(persistent, "https://vault.example", "token", "k", "vault:v1:xxx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetMasterKey(context.Background()); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if persistent.calls != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", persistent.calls)
	}
}

func TestVaultProviderNoRetryOnAuthFailure(t *testing.T) {
	t.Setenv("KEEPSAVE_VAULT_MAX_ATTEMPTS", "5")
	t.Setenv("KEEPSAVE_VAULT_RETRY_BASE_DELAY", "1ms")

	// A 403 is a clear auth failure — must fail on the FIRST attempt, no retry.
	authFail := &flakyHTTP{failN: 100, status5xx: false}
	// status5xx false + failN>0 would give transport error; instead use a
	// dedicated fake returning 403 every time.
	_ = authFail
	client := &fakeHTTP{status: http.StatusForbidden, body: `{"errors":["permission denied"]}`}
	counting := &countingHTTP{inner: client}
	p, err := NewVaultProvider(counting, "https://vault.example", "token", "k", "vault:v1:xxx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetMasterKey(context.Background()); err == nil {
		t.Fatal("expected error on 403")
	}
	if counting.calls != 1 {
		t.Fatalf("auth failure must not be retried: got %d calls", counting.calls)
	}
}

// countingHTTP wraps a VaultHTTPClient and counts calls.
type countingHTTP struct {
	inner VaultHTTPClient
	calls int
}

func (c *countingHTTP) Do(r *http.Request) (*http.Response, error) {
	c.calls++
	return c.inner.Do(r)
}
