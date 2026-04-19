package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func setenv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func goodKey() string {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestLoad_Defaults(t *testing.T) {
	setenv(t, map[string]string{
		"DATABASE_URL": "postgres://u:p@db/k?sslmode=require",
		"MASTER_KEY":   goodKey(),
		"JWT_SECRET":   "secret",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.KeyProvider != "env" {
		t.Errorf("KeyProvider = %q, want env", cfg.KeyProvider)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.AuditLogRetentionDays != 365 {
		t.Errorf("retention = %d, want 365", cfg.AuditLogRetentionDays)
	}
	if len(cfg.MasterKey) != 32 {
		t.Errorf("MasterKey len = %d, want 32", len(cfg.MasterKey))
	}
}

func TestLoad_ProdLockdown(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "cors wildcard rejected",
			env: map[string]string{
				"DATABASE_URL":  "postgres://u:p@db/k?sslmode=require",
				"MASTER_KEY":    goodKey(),
				"JWT_SECRET":    "s",
				"KEEPSAVE_ENV":  "production",
				"CORS_ORIGINS":  "*",
			},
			want: "CORS_ORIGINS=*",
		},
		{
			name: "sslmode disable rejected",
			env: map[string]string{
				"DATABASE_URL":  "postgres://u:p@db/k?sslmode=disable",
				"MASTER_KEY":    goodKey(),
				"JWT_SECRET":    "s",
				"KEEPSAVE_ENV":  "production",
				"CORS_ORIGINS":  "https://app.example.com",
			},
			want: "sslmode=disable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setenv(t, tc.env)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestLoad_KMSProviderSkipsEnvKey(t *testing.T) {
	setenv(t, map[string]string{
		"DATABASE_URL":             "postgres://u:p@db/k?sslmode=require",
		"JWT_SECRET":               "s",
		"KEEPSAVE_KEY_PROVIDER":    "awskms",
		"KEEPSAVE_KMS_KEY_ID":     "alias/keepsave",
		"KEEPSAVE_KMS_CIPHERTEXT": "AAA",
	})
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.KeyProvider != "awskms" {
		t.Errorf("KeyProvider = %q", cfg.KeyProvider)
	}
	if cfg.MasterKey != nil {
		t.Errorf("MasterKey should be nil for KMS provider, got %d bytes", len(cfg.MasterKey))
	}
}
