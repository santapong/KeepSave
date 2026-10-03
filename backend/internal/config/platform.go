package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Platform struct {
	RunnerAddress, RunnerCert, RunnerKey, RunnerClientCA                                                                 string
	ApplicationOrigin                                                                                                    string
	IdentityEnabled, TeamVaultEnabled, MCPEnabled, RunsEnabled, RunAdmissionEnabled, BrokerDispatchEnabled, SMTPAccepted bool
	SMTPHost, SMTPPort, SMTPUsername, SMTPPassword, SMTPFrom                                                             string
}

func loadPlatform(social SocialAuth) (Platform, error) {
	p := Platform{ApplicationOrigin: strings.TrimRight(os.Getenv("KEEPSAVE_APPLICATION_ORIGIN"), "/"), SMTPHost: os.Getenv("KEEPSAVE_SMTP_HOST"), SMTPPort: getenvOr("KEEPSAVE_SMTP_PORT", "587"), SMTPUsername: os.Getenv("KEEPSAVE_SMTP_USERNAME"), SMTPPassword: os.Getenv("KEEPSAVE_SMTP_PASSWORD"), SMTPFrom: os.Getenv("KEEPSAVE_SMTP_FROM"), RunnerAddress: os.Getenv("KEEPSAVE_RUNNER_LISTEN_ADDRESS"), RunnerCert: os.Getenv("KEEPSAVE_RUNNER_TLS_CERT"), RunnerKey: os.Getenv("KEEPSAVE_RUNNER_TLS_KEY"), RunnerClientCA: os.Getenv("KEEPSAVE_RUNNER_CLIENT_CA")}
	for key, target := range map[string]*bool{"KEEPSAVE_IDENTITY_ENABLED": &p.IdentityEnabled, "KEEPSAVE_TEAM_VAULT_ENABLED": &p.TeamVaultEnabled, "KEEPSAVE_MCP_ENABLED": &p.MCPEnabled, "KEEPSAVE_RUNS_ENABLED": &p.RunsEnabled, "KEEPSAVE_RUN_ADMISSION_ENABLED": &p.RunAdmissionEnabled, "KEEPSAVE_BROKER_DISPATCH_ENABLED": &p.BrokerDispatchEnabled, "KEEPSAVE_SMTP_ACCEPTED": &p.SMTPAccepted} {
		if raw := os.Getenv(key); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return p, fmt.Errorf("%s must be a boolean", key)
			}
			*target = value
		}
	}
	enabled := p.IdentityEnabled || p.TeamVaultEnabled || p.MCPEnabled || p.RunsEnabled
	if enabled || p.ApplicationOrigin != "" {
		u, e := url.Parse(p.ApplicationOrigin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return p, fmt.Errorf("KEEPSAVE_APPLICATION_ORIGIN requires a canonical HTTPS origin")
		}
		if social.Origin != "" && social.Origin != p.ApplicationOrigin {
			return p, fmt.Errorf("application and social login origins must match")
		}
	}
	if (p.RunAdmissionEnabled || p.BrokerDispatchEnabled) && (!p.RunsEnabled || !p.MCPEnabled) {
		return p, fmt.Errorf("run admission and dispatch require the MCP and run platform flags")
	}
	count := 0
	for _, v := range []string{p.SMTPHost, p.SMTPUsername, p.SMTPPassword, p.SMTPFrom} {
		if v != "" {
			count++
		}
	}
	if count != 0 && count != 4 {
		return p, fmt.Errorf("SMTP requires host, authenticated username/password and sender")
	}
	if p.SMTPAccepted && (count != 4 || !p.IdentityEnabled) {
		return p, fmt.Errorf("SMTP acceptance requires configured identity proof delivery")
	}
	port, e := strconv.Atoi(p.SMTPPort)
	if e != nil || port < 1 || port > 65535 {
		return p, fmt.Errorf("SMTP port is invalid")
	}
	runnerFields := 0
	for _, v := range []string{p.RunnerAddress, p.RunnerCert, p.RunnerKey, p.RunnerClientCA} {
		if v != "" {
			runnerFields++
		}
	}
	if runnerFields != 0 && (runnerFields != 4 || !p.RunsEnabled) {
		return p, fmt.Errorf("runner listener requires platform flag and complete mutual TLS configuration")
	}
	if runnerFields == 4 {
		host, port, err := net.SplitHostPort(p.RunnerAddress)
		value, parseErr := strconv.Atoi(port)
		if err != nil || host == "" || parseErr != nil || value < 1 || value > 65535 {
			return p, fmt.Errorf("runner listener requires an explicit host and valid port")
		}
	}
	if p.BrokerDispatchEnabled && runnerFields != 4 {
		return p, fmt.Errorf("broker dispatch requires the dedicated mutual TLS runner listener")
	}
	return p, nil
}
func (p Platform) SMTPConfigured() bool {
	return p.SMTPHost != "" && p.SMTPUsername != "" && p.SMTPPassword != "" && p.SMTPFrom != ""
}
