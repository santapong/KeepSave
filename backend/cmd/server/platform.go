package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/santapong/KeepSave/backend/internal/api"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/broker"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/diagnostics"
	"github.com/santapong/KeepSave/backend/internal/harness"
	"github.com/santapong/KeepSave/backend/internal/identity"
	"github.com/santapong/KeepSave/backend/internal/mcpauth"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/runs"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"github.com/santapong/KeepSave/backend/internal/version"
	"os"
	"strconv"
)

// All adapters share current policy and the same authority barrier. No connector
// process, provider network request or credential decryption occurs here.
func configurePlatform(deps *api.Dependencies, cfg *config.Config, db *sql.DB, d repository.Dialect, keys *crypto.Service, audit *repository.AuditRepository, v *vault.Service, sessions *service.SessionService) error {
	p := cfg.Platform
	pg := d.DBType() == repository.DBTypePostgres
	if !pg && (p.IdentityEnabled || p.TeamVaultEnabled || p.MCPEnabled || p.RunsEnabled) {
		return fmt.Errorf("PostgreSQL required for platform capabilities")
	}
	guard := authority.Guard{DB: db, Dialect: d}
	authorize := func(ctx context.Context, tx *sql.Tx, principal policy.Principal, action policy.Action, resource policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: d, RequireHumanSession: true}}).Authorize(ctx, principal, action, resource)
	}
	recoveryAccepted, _ := strconv.ParseBool(os.Getenv("KEEPSAVE_RECOVERY_VERIFIED"))
	doctor := &diagnostics.Service{DB: db, Config: diagnostics.Config{PostgreSQL: pg, GoogleConfigured: cfg.SocialAuth.Google.ClientID != "", GitHubConfigured: cfg.SocialAuth.GitHub.ClientID != "", SMTPConfigured: p.SMTPConfigured(), SMTPAccepted: p.SMTPAccepted, RecoveryAccepted: recoveryAccepted, MCPEnabled: p.MCPEnabled, RunsEnabled: p.RunAdmissionEnabled}}
	deps.TeamVaultHandler = api.NewTeamVaultHandler(v, auditview.New(db, authorize, audit), doctor, p.TeamVaultEnabled)
	if p.TeamVaultEnabled {
		deps.PlatformCapabilities = append(deps.PlatformCapabilities, "team_vault")
	}
	identities := identity.New(db, identity.Config{Enabled: p.IdentityEnabled, PostgreSQL: pg, SMTPConfigured: p.SMTPConfigured() && p.SMTPAccepted, ApplicationOrigin: p.ApplicationOrigin, EnabledSocialProviders: map[string]bool{"google": cfg.SocialAuth.Google.ClientID != "", "github": cfg.SocialAuth.GitHub.ClientID != ""}}, audit, vault.NewPayloadCustody(keys), guard)
	identities.EnableRecoveryRevocation(sessions)
	deps.IdentityPlatformHandler = api.NewIdentityPlatformHandler(identities)
	if p.IdentityEnabled {
		deps.PlatformCapabilities = append(deps.PlatformCapabilities, "identity_platform")
		if p.SMTPAccepted {
			deps.PlatformCapabilities = append(deps.PlatformCapabilities, "identity_email_proofs")
		}
	}
	if !pg {
		return nil
	}
	b := broker.New(broker.CryptoCustody{Service: keys})
	toolRuns := runs.New(db, b, authorize, audit, runs.Flags{Enabled: p.RunsEnabled, Admission: p.RunAdmissionEnabled, Dispatch: p.BrokerDispatchEnabled, Endpoint: p.ApplicationOrigin + "/mcp"})
	toolRuns.LockProjectSubjects = guard.LockProjectSubjects
	toolRuns.LockMaintenanceSubjects = guard.LockProjectMaintenance
	toolRuns.PackageExporter = harness.ExportAutomation
	// Construct OAuth authority independently of issuance kill switches so current
	// result/status checks and revocation never rely on a stale in-memory allow.
	if p.ApplicationOrigin != "" {
		oauth, e := mcpauth.New(db, d, audit, guard, mcpauth.Config{Issuer: p.ApplicationOrigin, Resource: p.ApplicationOrigin + "/mcp", AppURL: p.ApplicationOrigin, AllowedOrigins: []string{p.ApplicationOrigin}})
		if e != nil {
			return e
		}
		toolRuns.RequireGrantTx = oauth.RequireGrantTx
		toolRuns.FamilyPrincipalTx = oauth.FamilyPrincipalTx
		if p.MCPEnabled {
			transport, e := mcpgateway.New(oauth, toolRuns, mcpgateway.Config{OAuth: oauth.Config(), Version: version.Version})
			if e != nil {
				return e
			}
			deps.MCPPlatformHandler = api.NewMCPPlatformHandler(oauth)
			deps.MCPTransport = transport
			deps.PlatformCapabilities = append(deps.PlatformCapabilities, "mcp_delegation")
		}
	}
	// Tool management routes and lifecycle cascades are wired below once their
	// application service is configured; the shared core remains client neutral.
	deps.ToolPlatformHandler = api.NewToolPlatformHandler(toolRuns)
	identities.EnableCascade(toolRuns)
	if v != nil {
		v.EnableProjectArchive(toolRuns)
	}
	if p.RunsEnabled {
		deps.PlatformCapabilities = append(deps.PlatformCapabilities, "controlled_tools")
	}
	return nil
}
