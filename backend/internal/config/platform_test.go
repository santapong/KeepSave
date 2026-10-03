package config

import (
	"testing"
)

func TestPlatformFlagsDefaultOff(t *testing.T) {
	p, e := loadPlatform(SocialAuth{})
	if e != nil {
		t.Fatal(e)
	}
	if p.IdentityEnabled || p.TeamVaultEnabled || p.MCPEnabled || p.RunsEnabled || p.RunAdmissionEnabled || p.BrokerDispatchEnabled || p.SMTPAccepted {
		t.Fatal("new admission enabled by default")
	}
}
func TestPlatformConfigurationFailsClosed(t *testing.T) {
	for _, c := range []struct {
		name   string
		values map[string]string
	}{
		{"invalid_flag", map[string]string{"KEEPSAVE_MCP_ENABLED": "perhaps"}},
		{"missing_origin", map[string]string{"KEEPSAVE_MCP_ENABLED": "true"}},
		{"plain_http", map[string]string{"KEEPSAVE_MCP_ENABLED": "true", "KEEPSAVE_APPLICATION_ORIGIN": "http://127.0.0.1:8080"}},
		{"resource_in_origin", map[string]string{"KEEPSAVE_APPLICATION_ORIGIN": "https://app.keepsave.invalid/mcp"}},
		{"partial_smtp", map[string]string{"KEEPSAVE_SMTP_HOST": "smtp.invalid"}},
		{"unconfigured_smtp_acceptance", map[string]string{"KEEPSAVE_SMTP_ACCEPTED": "true"}},
		{"dispatch_without_platform", map[string]string{"KEEPSAVE_BROKER_DISPATCH_ENABLED": "true"}},
		{"partial_runner", map[string]string{"KEEPSAVE_RUNS_ENABLED": "true", "KEEPSAVE_APPLICATION_ORIGIN": "https://app.keepsave.invalid", "KEEPSAVE_RUNNER_LISTEN_ADDRESS": "127.0.0.1:8444"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			for key, value := range c.values {
				t.Setenv(key, value)
			}
			if _, e := loadPlatform(SocialAuth{}); e == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}

func TestPlatformCompletePrivateRunnerConfiguration(t *testing.T) {
	for key, value := range map[string]string{
		"KEEPSAVE_APPLICATION_ORIGIN": "https://app.keepsave.invalid", "KEEPSAVE_MCP_ENABLED": "true", "KEEPSAVE_RUNS_ENABLED": "true", "KEEPSAVE_BROKER_DISPATCH_ENABLED": "true",
		"KEEPSAVE_RUNNER_LISTEN_ADDRESS": "127.0.0.1:8444", "KEEPSAVE_RUNNER_TLS_CERT": "/private/server.crt", "KEEPSAVE_RUNNER_TLS_KEY": "/private/server.key", "KEEPSAVE_RUNNER_CLIENT_CA": "/private/runner-ca.crt",
	} {
		t.Setenv(key, value)
	}
	if _, err := loadPlatform(SocialAuth{}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KEEPSAVE_RUNNER_LISTEN_ADDRESS", ":8444")
	if _, err := loadPlatform(SocialAuth{}); err == nil {
		t.Fatal("implicit all-interface host accepted")
	}
}
