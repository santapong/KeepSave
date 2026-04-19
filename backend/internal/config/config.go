package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL string

	// KeyProvider selects the master-key source. Valid values: env (default),
	// awskms, gcpkms, vault. See docs/THREAT_MODEL.md for guidance.
	KeyProvider string

	// MasterKey is populated only when KeyProvider == "env". KMS providers
	// fetch the key at startup in main.go via the keyprovider package.
	MasterKey []byte

	// KMS / Vault parameters. Empty when not in use.
	KMSKeyID         string
	KMSCiphertext    string
	VaultAddr        string
	VaultToken       string
	VaultKeyName     string
	VaultCiphertext  string

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
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
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
		masterKey = decoded
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
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

	return &Config{
		DatabaseURL:           databaseURL,
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
	}, nil
}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
