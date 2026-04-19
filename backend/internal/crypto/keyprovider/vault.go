package keyprovider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VaultHTTPClient is the narrow interface VaultProvider needs. stdlib
// *http.Client satisfies it; tests pass a fake.
type VaultHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
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
	return &VaultProvider{http: client, addr: strings.TrimRight(addr, "/"), token: token, keyName: keyName, ciphertext: ciphertext}, nil
}

func (p *VaultProvider) Name() string { return "vault" }

func (p *VaultProvider) GetMasterKey(ctx context.Context) ([]byte, error) {
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

func (p *VaultProvider) Rotate(_ context.Context) error { return ErrUnsupported }
