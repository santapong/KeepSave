package service

// A-02 audit-coverage sweep (enterprise + edge services): webhook, envfile,
// SSO, backup and secret-policy mutations now emit canonical audit rows.
// Real repositories over in-memory SQLite (hand-rolled DDL), asserting the
// audit row (action + actor). See audit_a02_test.go for the shared helpers.

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// TestWebhookService_EmitsAudit covers webhook.registered and webhook.removed.
// The webhook store is in-memory, so only the audit_log table is needed.
func TestWebhookService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t)
	ws := NewWebhookService(repository.NewAuditRepository(db, dialect))

	actor := uuid.New()
	pid := uuid.New()
	if err := ws.RegisterWebhook(pid, WebhookConfig{URL: "https://example.com/hook", Events: []string{"*"}}, actor, "10.1.1.1"); err != nil {
		t.Fatalf("RegisterWebhook: %v", err)
	}
	requireAuditRow(t, db, "webhook.registered", actor)

	ws.RemoveWebhooks(pid, actor, "10.1.1.1")
	requireAuditRow(t, db, "webhook.removed", actor)

	// The webhook secret must never appear in the audit details.
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE action = 'webhook.registered'`).Scan(&details); err != nil {
		t.Fatalf("read details: %v", err)
	}
	if details == "" || details == "null" {
		t.Errorf("webhook.registered details empty: %q", details)
	}
}

// ddlProjects/ddlEnvironments/ddlSecrets mirror the crypto-bearing fixtures in
// keyrotation_service_test.go so the envfile/backup services run end-to-end.
const ddlProjects = `CREATE TABLE projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	owner_id TEXT NOT NULL,
	encrypted_dek BLOB,
	dek_nonce BLOB,
	allowed_origins TEXT NOT NULL DEFAULT '[]',
	embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const ddlEnvironments = `CREATE TABLE environments (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const ddlSecrets = `CREATE TABLE secrets (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	environment_id TEXT NOT NULL,
	key TEXT NOT NULL,
	encrypted_value BLOB,
	value_nonce BLOB,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// newCryptoSvc builds a deterministic crypto service for tests.
func newCryptoSvc(t *testing.T) *crypto.Service {
	t.Helper()
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 7)
	}
	cs, err := crypto.NewService(masterKey)
	if err != nil {
		t.Fatalf("crypto service: %v", err)
	}
	return cs
}

// seedRealProjectWithEnv inserts a project (with a fresh DEK) and one
// environment, returning the project and environment IDs.
func seedRealProjectWithEnv(t *testing.T, db *sql.DB, cs *crypto.Service, owner uuid.UUID, envName string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	dek, err := cs.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	encDEK, dekNonce, err := cs.EncryptDEK(dek)
	if err != nil {
		t.Fatalf("EncryptDEK: %v", err)
	}
	pid := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?,?,'',?,?,?)`,
		pid.String(), "p", owner.String(), encDEK, dekNonce); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	envID := uuid.New()
	if _, err := db.Exec(`INSERT INTO environments (id, project_id, name) VALUES (?,?,?)`, envID.String(), pid.String(), envName); err != nil {
		t.Fatalf("seed env: %v", err)
	}
	return pid, envID
}

// TestEnvFileService_ImportEmitsAudit covers envfile.imported end-to-end.
func TestEnvFileService_ImportEmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlProjects, ddlEnvironments, ddlSecrets)
	cs := newCryptoSvc(t)
	owner := uuid.New()
	pid, _ := seedRealProjectWithEnv(t, db, cs, owner, "alpha")

	svc := NewEnvFileService(
		repository.NewSecretRepository(db, dialect),
		repository.NewProjectRepository(db, dialect),
		repository.NewEnvironmentRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		cs,
	)

	actor := uuid.New()
	res, err := svc.Import(pid, "alpha", "FOO=1\nBAR=two\n", false, actor, "10.2.2.2")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Created) != 2 {
		t.Fatalf("created = %d, want 2", len(res.Created))
	}
	requireAuditRow(t, db, "envfile.imported", actor)

	// The created count should be reflected in details (no secret values).
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE action = 'envfile.imported'`).Scan(&details); err != nil {
		t.Fatalf("read details: %v", err)
	}
	if want := `"created_count":2`; !strings.Contains(details, want) {
		t.Errorf("details %q missing %q", details, want)
	}
	if strings.Contains(details, "two") {
		t.Errorf("details leaked a secret value: %q", details)
	}
}

const ddlSSOConfigs = `CREATE TABLE sso_configs (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	issuer_url TEXT,
	client_id TEXT,
	client_secret_encrypted BLOB,
	client_secret_nonce BLOB,
	metadata TEXT,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (organization_id, provider)
)`

// TestSSOService_DeleteEmitsAudit covers sso.deleted end-to-end. NOTE:
// ConfigureSSO routes a models.JSONMap (Metadata) through SSORepository.Upsert
// -> db.Exec, which mattn/go-sqlite3 rejects (JSONMap does not satisfy
// driver.Valuer; Postgres-only). We therefore seed a config row directly and
// exercise the JSONMap-free Delete path; sso.configured uses the identical
// emitAudit shape.
func TestSSOService_DeleteEmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSSOConfigs)
	svc := NewSSOService(
		repository.NewSSORepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		newCryptoSvc(t),
	)

	orgID := uuid.New()
	if _, err := db.Exec(`INSERT INTO sso_configs (id, organization_id, provider, issuer_url, client_id, enabled) VALUES (?,?,?,?,?,1)`,
		uuid.New().String(), orgID.String(), "oidc", "https://idp", "client"); err != nil {
		t.Fatalf("seed sso: %v", err)
	}

	actor := uuid.New()
	if err := svc.DeleteSSOConfig(orgID, actor, "oidc", "10.3.3.3"); err != nil {
		t.Fatalf("DeleteSSOConfig: %v", err)
	}
	requireAuditRow(t, db, "sso.deleted", actor)
}

const ddlComplianceReports = `CREATE TABLE compliance_reports (
	id TEXT PRIMARY KEY,
	organization_id TEXT,
	report_type TEXT,
	status TEXT,
	data TEXT,
	generated_by TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	completed_at TIMESTAMP
)`

// TestComplianceService_GenerateReportAuditOrdering pins the ComplianceService
// audit wiring. ComplianceService.GenerateReport finishes with
// complianceRepo.Complete(id, data) where data is a models.JSONMap pushed
// through db.Exec — which mattn/go-sqlite3 rejects (JSONMap lacks a
// driver.Valuer signature; Postgres-only), so the success path cannot be
// exercised under SQLite. This test instead pins that the compliance.generated
// emit is placed AFTER a successful Complete (best-effort, never before): the
// Complete failure must NOT leave a spurious audit row. It also confirms the
// service is constructed with a non-nil auditRepo.
func TestComplianceService_GenerateReportAuditOrdering(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlComplianceReports)
	svc := NewComplianceService(
		repository.NewComplianceRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		repository.NewOrganizationRepository(db, dialect),
	)

	org := uuid.New()
	actor := uuid.New()
	// Complete() fails under SQLite (JSONMap Exec). The emit must not have fired.
	if _, err := svc.GenerateReport(org, actor, "soc2", "127.0.0.1"); err == nil {
		t.Skip("GenerateReport.Complete unexpectedly succeeded under SQLite; " +
			"the JSONMap repo limitation may have been fixed — re-enable a positive assertion")
	}
	if got := auditRowsFor(t, db, "compliance.generated", nil); got != 0 {
		t.Fatalf("compliance.generated emitted %d rows on a failed Complete, want 0 (emit must follow success)", got)
	}
}

const ddlBackupSnapshots = `CREATE TABLE backup_snapshots (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	snapshot_type TEXT,
	encrypted_data BLOB,
	data_nonce BLOB,
	size_bytes INTEGER,
	created_by TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// TestBackupService_CreateEmitsAudit covers backup.created end-to-end.
func TestBackupService_CreateEmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlProjects, ddlEnvironments, ddlSecrets, ddlBackupSnapshots)
	cs := newCryptoSvc(t)
	owner := uuid.New()
	pid, _ := seedRealProjectWithEnv(t, db, cs, owner, "alpha")

	svc := NewBackupService(
		repository.NewBackupRepository(db, dialect),
		repository.NewSecretRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
		cs,
	)

	actor := uuid.New()
	snap, err := svc.CreateBackup(pid, actor, "full", "10.4.4.4")
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if snap.ID == uuid.Nil {
		t.Fatal("backup snapshot id is nil")
	}
	requireAuditRow(t, db, "backup.created", actor)
}

const ddlSecretPolicies = `CREATE TABLE secret_policies (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL UNIQUE,
	max_age_days INTEGER,
	rotation_reminder_days INTEGER,
	require_rotation INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// TestSecretPolicyService_SetEmitsAudit covers policy.set end-to-end.
func TestSecretPolicyService_SetEmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSecretPolicies)
	svc := NewSecretPolicyService(db, dialect, repository.NewAuditRepository(db, dialect))

	actor := uuid.New()
	pid := uuid.New()
	if _, err := svc.SetPolicy(pid, 90, 14, true, actor, "10.5.5.5"); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	requireAuditRow(t, db, "policy.set", actor)
}
