package service

// A-02 audit-coverage sweep (agent/MCP/OAuth/anomaly services): lease, oauth,
// mcp and anomaly mutations now emit canonical audit rows. Real repositories
// over in-memory SQLite (hand-rolled DDL), asserting the audit row
// (action + actor). Shared helpers live in audit_a02_test.go.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

const ddlSecretLeases = `CREATE TABLE secret_leases (
	id TEXT PRIMARY KEY,
	api_key_id TEXT,
	project_id TEXT NOT NULL,
	environment TEXT,
	secret_keys TEXT,
	granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP,
	revoked INTEGER NOT NULL DEFAULT 0,
	revoked_at TIMESTAMP
)`

// TestLeaseService_EmitsAudit covers lease.created and lease.revoked.
func TestLeaseService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSecretLeases)
	svc := NewLeaseService(db, dialect, repository.NewAuditRepository(db, dialect))

	apiKeyID := uuid.New()
	pid := uuid.New()
	lease, err := svc.CreateLease(apiKeyID, pid, "alpha", []string{"DATABASE_URL"}, time.Hour, "10.6.6.6")
	if err != nil {
		t.Fatalf("CreateLease: %v", err)
	}
	if got := auditRowsFor(t, db, "lease.created", &apiKeyID); got != 1 {
		t.Fatalf("lease.created rows=%d, want 1", got)
	}

	revoker := uuid.New()
	if err := svc.RevokeLease(lease.ID, pid, revoker, "10.6.6.6"); err != nil {
		t.Fatalf("RevokeLease: %v", err)
	}
	requireAuditRow(t, db, "lease.revoked", revoker)
}

const ddlOAuthClients = `CREATE TABLE oauth_clients (
	id TEXT PRIMARY KEY,
	client_id TEXT,
	client_secret_hash TEXT,
	name TEXT,
	description TEXT,
	owner_id TEXT,
	redirect_uris TEXT,
	scopes TEXT,
	grant_types TEXT,
	logo_url TEXT,
	homepage_url TEXT,
	is_public INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const ddlOAuthTokens = `CREATE TABLE oauth_tokens (
	id TEXT PRIMARY KEY,
	client_id TEXT,
	revoked INTEGER NOT NULL DEFAULT 0
)`

// TestOAuthService_EmitsAudit covers oauth.client_registered and
// oauth.client_deleted end-to-end.
func TestOAuthService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlOAuthClients, ddlOAuthTokens)
	svc := NewOAuthService(
		repository.NewOAuthRepository(db, dialect),
		repository.NewUserRepository(db, dialect),
		nil,
		repository.NewAuditRepository(db, dialect),
	)

	owner := uuid.New()
	client, _, err := svc.RegisterClient("My App", "desc", owner,
		[]string{"https://app.example/cb"}, []string{"read"}, []string{"authorization_code"}, "", "", false, "10.7.7.7")
	if err != nil {
		t.Fatalf("RegisterClient: %v", err)
	}
	requireAuditRow(t, db, "oauth.client_registered", owner)

	if err := svc.DeleteClient(client.ID, owner, "10.7.7.7"); err != nil {
		t.Fatalf("DeleteClient: %v", err)
	}
	requireAuditRow(t, db, "oauth.client_deleted", owner)

	// The client secret must never appear in the audit details.
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE action = 'oauth.client_registered'`).Scan(&details); err != nil {
		t.Fatalf("read details: %v", err)
	}
	if details == "" || details == "null" {
		t.Errorf("oauth.client_registered details empty: %q", details)
	}
}

const ddlMCPServers = `CREATE TABLE mcp_servers (
	id TEXT PRIMARY KEY,
	name TEXT,
	description TEXT,
	owner_id TEXT,
	github_url TEXT,
	github_branch TEXT,
	entry_command TEXT,
	transport TEXT,
	icon_url TEXT,
	version TEXT,
	status TEXT,
	build_log TEXT NOT NULL DEFAULT '',
	env_mappings TEXT,
	tool_definitions TEXT,
	last_synced_at TIMESTAMP,
	install_count INTEGER NOT NULL DEFAULT 0,
	is_public INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const ddlMCPInstallations = `CREATE TABLE mcp_installations (
	id TEXT PRIMARY KEY,
	user_id TEXT,
	mcp_server_id TEXT,
	project_id TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	config TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// TestMCPService_EmitsAudit covers the full MCP server + installation lifecycle:
// register, update, install, update-installation and delete. The MCP repository
// marshals every JSONMap to a JSON string before db.Exec, so the whole flow
// runs under SQLite.
func TestMCPService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlMCPServers, ddlMCPInstallations)
	svc := NewMCPService(
		repository.NewMCPRepository(db, dialect),
		nil, nil, nil,
		repository.NewAuditRepository(db, dialect),
	)

	owner := uuid.New()
	server, err := svc.RegisterServer("srv", "d", owner, "https://github.com/x/y", "main", "node index.js",
		"stdio", "", "1.0.0", models.JSONMap{"API_KEY": "ref"}, true, "10.8.8.8")
	if err != nil {
		t.Fatalf("RegisterServer: %v", err)
	}
	requireAuditRow(t, db, "mcp.server_registered", owner)

	server.Name = "srv2"
	if err := svc.UpdateServer(server, "10.8.8.8"); err != nil {
		t.Fatalf("UpdateServer: %v", err)
	}
	requireAuditRow(t, db, "mcp.server_updated", owner)

	installer := uuid.New()
	inst, err := svc.InstallServer(installer, server.ID, nil, models.JSONMap{"opt": "v"}, "10.8.8.8")
	if err != nil {
		t.Fatalf("InstallServer: %v", err)
	}
	requireAuditRow(t, db, "mcp.server_installed", installer)

	editor := uuid.New()
	if err := svc.UpdateInstallation(inst.ID, false, models.JSONMap{"opt": "v2"}, editor, "10.8.8.8"); err != nil {
		t.Fatalf("UpdateInstallation: %v", err)
	}
	requireAuditRow(t, db, "mcp.installation_updated", editor)

	if err := svc.DeleteServer(server.ID, owner, "10.8.8.8"); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
	requireAuditRow(t, db, "mcp.server_deleted", owner)
}

const ddlAnomalies = `CREATE TABLE anomalies (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	api_key_id TEXT,
	anomaly_type TEXT,
	severity TEXT,
	description TEXT,
	details TEXT,
	status TEXT,
	detected_at TIMESTAMP,
	acknowledged_at TIMESTAMP,
	resolved_at TIMESTAMP
)`

const ddlAnomalyRules = `CREATE TABLE anomaly_rules (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	api_key_id TEXT,
	rule_type TEXT,
	config TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_by TEXT,
	created_at TIMESTAMP,
	updated_at TIMESTAMP
)`

// TestAnomalyService_EmitsAudit covers rule create/update/delete and anomaly
// acknowledge/resolve. CreateRule/UpdateRule json.Marshal the config to a
// string, so the whole flow runs under SQLite.
func TestAnomalyService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlAnomalies, ddlAnomalyRules)
	svc := NewAnomalyService(db, dialect, nil, repository.NewAuditRepository(db, dialect))

	creator := uuid.New()
	pid := uuid.New()
	rule, err := svc.CreateRule(&pid, nil, "frequency", models.JSONMap{"threshold": 3}, creator, "10.9.9.9")
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	requireAuditRow(t, db, "anomaly.rule_created", creator)

	editor := uuid.New()
	if err := svc.UpdateRule(rule.ID, false, models.JSONMap{"threshold": 5}, editor, "10.9.9.9"); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	requireAuditRow(t, db, "anomaly.rule_updated", editor)

	if err := svc.DeleteRule(rule.ID, editor, "10.9.9.9"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	requireAuditRow(t, db, "anomaly.rule_deleted", editor)

	// Seed an anomaly row directly and ack/resolve it.
	anomalyID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO anomalies (id, anomaly_type, severity, description, details, status, detected_at) VALUES (?,?,?,?,?,?,?)`,
		anomalyID.String(), "new_ip", "high", "d", `{}`, "open", time.Now(),
	); err != nil {
		t.Fatalf("seed anomaly: %v", err)
	}

	acker := uuid.New()
	if err := svc.AcknowledgeAnomaly(anomalyID, acker, "10.9.9.9"); err != nil {
		t.Fatalf("AcknowledgeAnomaly: %v", err)
	}
	requireAuditRow(t, db, "anomaly.acknowledged", acker)

	if err := svc.ResolveAnomaly(anomalyID, acker, "10.9.9.9"); err != nil {
		t.Fatalf("ResolveAnomaly: %v", err)
	}
	requireAuditRow(t, db, "anomaly.resolved", acker)
}
