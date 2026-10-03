// Package diagnostics provides read-only setup evidence, never hidden upstream
// probes. Configuration presence is explicitly separate from live acceptance.
package diagnostics

import (
	"context"
	"database/sql"
	"time"
)

type Check struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Message string `json:"message"`
}
type Report struct {
	CheckedAt     time.Time `json:"checked_at"`
	Status        string    `json:"status"`
	Checks        []Check   `json:"checks"`
	ExternalReads bool      `json:"external_reads"`
}
type Config struct{ PostgreSQL, GoogleConfigured, GitHubConfigured, SMTPConfigured, SMTPAccepted, RecoveryAccepted, MCPEnabled, RunsEnabled bool }
type Service struct {
	DB     *sql.DB
	Config Config
}

func (s Service) Inspect(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	r := Report{CheckedAt: time.Now().UTC(), Status: "ready", Checks: []Check{}}
	add := func(name string, ok bool, message string) {
		state := "unconfigured"
		if ok {
			state = "configured"
		}
		r.Checks = append(r.Checks, Check{name, state, message})
	}
	if s.DB == nil || s.DB.PingContext(ctx) != nil {
		r.Status = "unavailable"
		r.Checks = append(r.Checks, Check{"database", "unavailable", "Database authority is unavailable."})
		return r
	}
	r.Checks = append(r.Checks, Check{"database", "reachable", "Read-only database connectivity check passed."})
	add("platform_database", s.Config.PostgreSQL, "New platform guarantees require PostgreSQL.")
	if s.Config.PostgreSQL {
		var present bool
		err := s.DB.QueryRowContext(ctx, `SELECT to_regclass('member_authority_state') IS NOT NULL AND to_regclass('tool_runs') IS NOT NULL AND to_regclass('identity_proofs') IS NOT NULL AND to_regclass('audit_exports') IS NOT NULL`).Scan(&present)
		if err != nil || !present {
			r.Status = "unavailable"
			r.Checks = append(r.Checks, Check{"migrations", "unavailable", "Platform migrations are incomplete."})
		} else {
			r.Checks = append(r.Checks, Check{"migrations", "present", "Expected platform tables are present; acceptance tests remain separate."})
		}
	}
	add("google_sign_in", s.Config.GoogleConfigured, "Real consent and callback acceptance must be recorded separately.")
	add("github_sign_in", s.Config.GitHubConfigured, "The sign-in OAuth app is separate from the broker GitHub App.")
	add("smtp", s.Config.SMTPConfigured, "No mail is sent by this check.")
	add("smtp_acceptance", s.Config.SMTPAccepted, "Proof email flows require installation-specific SMTP acceptance.")
	add("recovery_acceptance", s.Config.RecoveryAccepted, "Daily backups require an independently verified recovery drill.")
	add("mcp_admission", s.Config.MCPEnabled, "Exact client interoperability must be accepted separately.")
	add("run_admission", s.Config.RunsEnabled, "Live provider and runner isolation acceptance must be recorded separately.")
	return r
}
