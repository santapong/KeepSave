package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// leakedDevMasterKeyHashHex is the SHA-256 of the 32 raw bytes that decode
// from the development MASTER_KEY committed to docker-compose.yml. Production
// refuses any MASTER_KEY whose decoded bytes match this hash — the dev key
// is treated as permanently leaked.
const leakedDevMasterKeyHashHex = "f69968df7fb0fa71e2cdad7f258e0c1af7270a8cb759ca9168657e111821669a"

// prodMinJWTSecretBytes is the minimum acceptable JWT_SECRET length when
// KEEPSAVE_ENV=production. 32 bytes matches HS256 output width and current
// industry guidance.
const prodMinJWTSecretBytes = 32

type Config struct {
	DatabaseURL    string
	DBMaxOpenConns int
	DBMaxIdleConns int

	// KeyProvider selects the master-key source. Valid values: env (default),
	// awskms, gcpkms, vault. See docs/THREAT_MODEL.md for guidance.
	KeyProvider string

	// MasterKey is populated only when KeyProvider == "env". KMS providers
	// fetch the key at startup in main.go via the keyprovider package.
	MasterKey []byte

	// KMS / Vault parameters. Empty when not in use.
	KMSKeyID        string
	KMSCiphertext   string
	VaultAddr       string
	VaultToken      string
	VaultKeyName    string
	VaultCiphertext string

	JWTSecret   string
	Port        string
	CORSOrigins string

	// Env is "development" or "production" (lowercased). Production mode
	// rejects insecure defaults such as CORS_ORIGINS=* or sslmode=disable.
	Env string

	TLSCertFile     string
	TLSKeyFile      string
	TLSRedirect     bool
	TLSCipherSuites string

	AuditLogRetentionDays int

	// PromotionsEnabled is the promotion kill switch (FOLLOWUPS #0e,
	// ADR-0003 rollback plan step 1). When false, POST /promote and
	// POST /promotions/:id/approve return 503 without reaching the
	// promotion engine. Reject, rollback, and read endpoints stay live
	// so operators can drain the queue mid-incident. Toggled via
	// KEEPSAVE_PROMOTIONS_ENABLED; absent/empty means enabled.
	PromotionsEnabled bool

	// PlatformAdminEmails is the allowlist of user emails permitted to reach
	// the cross-tenant /admin endpoints (dashboard, traces, security events).
	// Sourced from KEEPSAVE_PLATFORM_ADMIN_EMAILS (comma-separated, lowercased).
	// Empty means nobody — /admin fails closed (DB-06).
	PlatformAdminEmails []string

	// FeedbackGitHubToken enables the in-app feedback endpoint. When empty
	// (default), POST /feedback returns 503. Sourced from FEEDBACK_GITHUB_TOKEN.
	FeedbackGitHubToken string

	// FeedbackGitHubRepo is the "owner/repo" that feedback issues are filed
	// against. Sourced from FEEDBACK_GITHUB_REPO.
	FeedbackGitHubRepo string

	// TrustedProxies is the allow-list of reverse-proxy CIDRs (comma-separated
	// in TRUSTED_PROXIES) whose X-Forwarded-For / X-Real-IP headers gin will
	// honour when deriving the client IP. Empty (default) ⇒ gin trusts NO proxy,
	// so c.ClientIP() returns the direct peer and a forged XFF header cannot
	// spoof the rate-limit key or the audit IP (CWE-348).
	TrustedProxies []string
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	maxOpen, err := strconv.Atoi(getenvOr("DB_MAX_OPEN_CONNS", "10"))
	if err != nil || maxOpen < 1 {
		return nil, fmt.Errorf("DB_MAX_OPEN_CONNS must be a positive integer")
	}
	maxIdle, err := strconv.Atoi(getenvOr("DB_MAX_IDLE_CONNS", "2"))
	if err != nil || maxIdle < 0 || maxIdle > maxOpen {
		return nil, fmt.Errorf("DB_MAX_IDLE_CONNS must be between zero and DB_MAX_OPEN_CONNS")
	}

	env := getenvOr("KEEPSAVE_ENV", "")
	if env == "" {
		env = getenvOr("APP_ENV", "development")
	}
	env = strings.ToLower(env)

	keyProvider := strings.ToLower(getenvOr("KEEPSAVE_KEY_PROVIDER", "env"))

	var masterKey []byte
	if keyProvider == "env" {
		mkB64 := os.Getenv("MASTER_KEY")
		if mkB64 == "" {
			return nil, fmt.Errorf("MASTER_KEY is required when KEEPSAVE_KEY_PROVIDER=env")
		}
		decoded, err := base64.StdEncoding.DecodeString(mkB64)
		if err != nil {
			return nil, fmt.Errorf("decoding MASTER_KEY: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("MASTER_KEY must be exactly 32 bytes, got %d", len(decoded))
		}
		if env == "production" {
			sum := sha256.Sum256(decoded)
			if hex.EncodeToString(sum[:]) == leakedDevMasterKeyHashHex {
				return nil, fmt.Errorf("MASTER_KEY matches the development key committed to docker-compose.yml; generate a fresh 32-byte key for production")
			}
		}
		masterKey = decoded
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if env == "production" && len(jwtSecret) < prodMinJWTSecretBytes {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d bytes when KEEPSAVE_ENV=production, got %d", prodMinJWTSecretBytes, len(jwtSecret))
	}

	corsOrigins := getenvOr("CORS_ORIGINS", "*")
	if env == "production" && strings.TrimSpace(corsOrigins) == "*" {
		return nil, fmt.Errorf("CORS_ORIGINS=* is not allowed when KEEPSAVE_ENV=production; set an explicit comma-separated origin list")
	}
	if env == "production" && strings.Contains(databaseURL, "sslmode=disable") {
		return nil, fmt.Errorf("sslmode=disable in DATABASE_URL is not allowed when KEEPSAVE_ENV=production")
	}

	retention := 365
	if v := os.Getenv("AUDIT_LOG_RETENTION_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("AUDIT_LOG_RETENTION_DAYS must be a positive integer, got %q", v)
		}
		retention = n
	}

	// A malformed value fails the boot loudly rather than defaulting to
	// enabled: an operator typo at incident time must not silently leave
	// promotions running.
	promotionsEnabled := true
	if v := os.Getenv("KEEPSAVE_PROMOTIONS_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("KEEPSAVE_PROMOTIONS_ENABLED must be a boolean (true/false/1/0), got %q", v)
		}
		promotionsEnabled = b
	}

	return &Config{
		DatabaseURL:           databaseURL,
		DBMaxOpenConns:        maxOpen,
		DBMaxIdleConns:        maxIdle,
		KeyProvider:           keyProvider,
		MasterKey:             masterKey,
		KMSKeyID:              os.Getenv("KEEPSAVE_KMS_KEY_ID"),
		KMSCiphertext:         os.Getenv("KEEPSAVE_KMS_CIPHERTEXT"),
		VaultAddr:             os.Getenv("VAULT_ADDR"),
		VaultToken:            os.Getenv("VAULT_TOKEN"),
		VaultKeyName:          os.Getenv("KEEPSAVE_VAULT_KEY_NAME"),
		VaultCiphertext:       os.Getenv("KEEPSAVE_VAULT_CIPHERTEXT"),
		JWTSecret:             jwtSecret,
		Port:                  getenvOr("PORT", "8080"),
		CORSOrigins:           corsOrigins,
		Env:                   env,
		TLSCertFile:           os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:            os.Getenv("TLS_KEY_FILE"),
		TLSRedirect:           strings.EqualFold(os.Getenv("TLS_REDIRECT"), "true"),
		TLSCipherSuites:       os.Getenv("TLS_CIPHER_SUITES"),
		AuditLogRetentionDays: retention,
		PromotionsEnabled:     promotionsEnabled,
		PlatformAdminEmails:   parseCommaList(os.Getenv("KEEPSAVE_PLATFORM_ADMIN_EMAILS")),
		FeedbackGitHubToken:   os.Getenv("FEEDBACK_GITHUB_TOKEN"),
		FeedbackGitHubRepo:    getenvOr("FEEDBACK_GITHUB_REPO", "santapong/KeepSave"),
		TrustedProxies:        parseCommaList(os.Getenv("TRUSTED_PROXIES")),
	}, nil
}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseCommaList splits a comma-separated env value into a trimmed, lowercased,
// de-duplicated slice, dropping empty entries.
func parseCommaList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		v := strings.ToLower(strings.TrimSpace(part))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
