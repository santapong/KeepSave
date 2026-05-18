package service

import (
	"strings"
	"testing"
)

// TestValidateWebhookURL_RejectsSSRFTargets exercises the audit S-H2 /
// ADR-0013 deny-list. The "production" cases force KEEPSAVE_ENV=production
// so loopback / RFC-1918 are blocked too; the "dev" cases use the default
// relaxed mode where local integration testing is the norm.
func TestValidateWebhookURL_RejectsSSRFTargets(t *testing.T) {
	t.Setenv("KEEPSAVE_ENV", "production")

	cases := []struct {
		name string
		url  string
		want string // substring of error message
	}{
		{"loopback v4", "https://127.0.0.1/x", "private or special-purpose"},
		{"rfc1918 10/8", "https://10.0.0.1/x", "private or special-purpose"},
		{"link-local 169.254", "https://169.254.169.254/", "private or special-purpose"},
		{"metadata hostname GCP", "https://metadata.google.internal/", "cloud metadata"},
		{"http in prod", "http://example.com/", "only permitted when KEEPSAVE_ENV=dev"},
		{"unsupported scheme", "ftp://example.com/", "unsupported url scheme"},
		{"no host", "https:///", "no host"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWebhookURL(tc.url)
			if err == nil {
				t.Fatalf("ValidateWebhookURL(%q): expected error, got nil", tc.url)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateWebhookURL_AcceptsPublicHTTPS(t *testing.T) {
	t.Setenv("KEEPSAVE_ENV", "production")
	if err := ValidateWebhookURL("https://example.com/hook"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateWebhookURL_DevAllowsLoopback(t *testing.T) {
	t.Setenv("KEEPSAVE_ENV", "dev")
	if err := ValidateWebhookURL("http://127.0.0.1:1234/x"); err != nil {
		t.Errorf("dev mode should allow loopback, got %v", err)
	}
}

func TestValidateWebhookURL_DevStillRejectsLinkLocal(t *testing.T) {
	t.Setenv("KEEPSAVE_ENV", "dev")
	if err := ValidateWebhookURL("http://169.254.169.254/"); err == nil {
		t.Error("dev mode should still reject cloud-metadata link-local")
	}
}
