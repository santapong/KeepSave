package keyprovider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// VaultHTTPClient is the narrow interface VaultProvider needs. stdlib
// *http.Client satisfies it; tests pass a fake.
type VaultHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Retry defaults for the Vault Transit unwrap at boot. A transient Vault blip
// (network reset, 5xx, 429) must not hard-crash the process; a bounded
// exponential backoff rides it out. A clear auth failure (401/403) is NOT
// retried — it is not transient. See docs/system/09-infrastructure.md.
const (
	defaultVaultMaxAttempts = 4
	defaultVaultBaseDelay   = 200 * time.Millisecond
	defaultVaultMaxDelay    = 5 * time.Second
	// defaultVaultBudget caps the total wall-clock spent across all attempts
	// (including backoff sleeps) so retries can never hang boot indefinitely,
	// even if the caller passed a context without a deadline.
	defaultVaultBudget = 20 * time.Second
)

// vaultRetryConfig controls the bounded retry / time-budget behavior. Zero
// values fall back to the defaults above.
type vaultRetryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	budget      time.Duration
}

// loadVaultRetryConfig reads the retry knobs from the environment, clamping to
// safe defaults on absence or malformed input (a boot-time typo must not
// disable resilience or set a pathological value).
func loadVaultRetryConfig() vaultRetryConfig {
	c := vaultRetryConfig{
		maxAttempts: defaultVaultMaxAttempts,
		baseDelay:   defaultVaultBaseDelay,
		maxDelay:    defaultVaultMaxDelay,
		budget:      defaultVaultBudget,
	}
	if n, err := strconv.Atoi(os.Getenv("KEEPSAVE_VAULT_MAX_ATTEMPTS")); err == nil && n >= 1 {
		c.maxAttempts = n
	}
	if d, err := time.ParseDuration(os.Getenv("KEEPSAVE_VAULT_RETRY_BASE_DELAY")); err == nil && d > 0 {
		c.baseDelay = d
	}
	if d, err := time.ParseDuration(os.Getenv("KEEPSAVE_VAULT_RETRY_MAX_DELAY")); err == nil && d > 0 {
		c.maxDelay = d
	}
	if d, err := time.ParseDuration(os.Getenv("KEEPSAVE_VAULT_RETRY_BUDGET")); err == nil && d > 0 {
		c.budget = d
	}
	return c
}

// VaultProvider calls HashiCorp Vault's Transit engine (`transit/decrypt/<key>`)
// to unwrap a pre-encrypted master key. Implemented with stdlib net/http
// so no extra Go module dependency is required.
type VaultProvider struct {
	http       VaultHTTPClient
	addr       string
	token      string
	keyName    string
	ciphertext string
	retry      vaultRetryConfig
}

// NewVaultProvider builds the provider.
//
//	addr        - Vault address, e.g. https://vault.example.com:8200
//	token       - Vault token with transit decrypt permission
//	keyName     - Name of the Transit key, e.g. "keepsave-master"
//	ciphertext  - "vault:v1:..." string to decrypt
func NewVaultProvider(client VaultHTTPClient, addr, token, keyName, ciphertext string) (*VaultProvider, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if addr == "" || token == "" || keyName == "" || ciphertext == "" {
		return nil, fmt.Errorf("vault provider: addr, token, keyName, and ciphertext are all required")
	}
	if _, err := url.Parse(addr); err != nil {
		return nil, fmt.Errorf("parsing Vault addr: %w", err)
	}
	return &VaultProvider{http: client, addr: strings.TrimRight(addr, "/"), token: token, keyName: keyName, ciphertext: ciphertext, retry: loadVaultRetryConfig()}, nil
}

func (p *VaultProvider) Name() string { return "vault" }

// errVaultAuth marks a non-transient auth failure (401/403). It is never
// retried — a bad token will not fix itself and retrying only delays the
// (correct) fail-closed boot error.
var errVaultAuth = errors.New("vault auth failure")

// GetMasterKey unwraps the master key from Vault Transit with a bounded
// exponential backoff. Transient failures (transport error, 5xx, 429) are
// retried up to the configured attempt count within a total time budget; a
// clear auth failure (401/403) fails immediately. On exhaustion the last
// error is wrapped so boot fails closed with a clear cause. The master key
// bytes are never logged.
func (p *VaultProvider) GetMasterKey(ctx context.Context) ([]byte, error) {
	// Cap total wall-clock across all attempts so retries can never hang boot
	// indefinitely, independent of the caller's context deadline.
	ctx, cancel := context.WithTimeout(ctx, p.retry.budget)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= p.retry.maxAttempts; attempt++ {
		key, err := p.fetchOnce(ctx)
		if err == nil {
			return key, nil
		}
		lastErr = err

		// Non-transient auth failures are not worth retrying.
		if errors.Is(err, errVaultAuth) {
			return nil, err
		}
		// No point sleeping after the final attempt or once the budget/context
		// is spent.
		if attempt == p.retry.maxAttempts || ctx.Err() != nil {
			break
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("vault unwrap aborted after %d attempt(s): %w", attempt, ctx.Err())
		case <-time.After(p.backoff(attempt)):
		}
	}

	return nil, fmt.Errorf("vault unwrap failed after %d attempt(s): %w", p.retry.maxAttempts, lastErr)
}

// fetchOnce performs a single Transit decrypt round-trip. It classifies
// 401/403 as errVaultAuth (non-transient) so the caller can stop retrying.
func (p *VaultProvider) fetchOnce(ctx context.Context) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{"ciphertext": p.ciphertext})
	urlStr := fmt.Sprintf("%s/v1/transit/decrypt/%s", p.addr, p.keyName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call vault: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("vault returned %d: %s: %w", resp.StatusCode, string(b), errVaultAuth)
		}
		return nil, fmt.Errorf("vault returned %d: %s", resp.StatusCode, string(b))
	}

	var decoded struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decoding vault response: %w", err)
	}

	key, err := base64.StdEncoding.DecodeString(decoded.Data.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("decoding vault plaintext: %w", err)
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

// backoff returns the delay before the given (1-based) attempt's retry:
// exponential growth from baseDelay, capped at maxDelay, with full jitter to
// avoid a thundering herd when many replicas boot against a recovering Vault.
func (p *VaultProvider) backoff(attempt int) time.Duration {
	d := p.retry.baseDelay << (attempt - 1)
	if d <= 0 || d > p.retry.maxDelay {
		d = p.retry.maxDelay
	}
	// Full jitter: random in [0, d]. crypto/rand avoids importing math/rand.
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		d = time.Duration(binary.BigEndian.Uint64(b[:]) % uint64(d+1))
	}
	return d
}

func (p *VaultProvider) Rotate(_ context.Context) error { return ErrUnsupported }
