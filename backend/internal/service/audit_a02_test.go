package service

// A-02 audit-coverage sweep: each previously-uncovered state-mutating service
// group now emits an audit_log row. These tests build the real repositories
// over in-memory SQLite (hand-rolled DDL, per the convention in
// keyrotation_service_test.go / audit_helper_test.go) and assert that the
// canonical entity.action row is written with the acting user.

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// auditRowsFor returns the number of audit_log rows for a given action that are
// attributed to the given actor. A nil actor matches any user_id.
func auditRowsFor(t *testing.T, db *sql.DB, action string, actor *uuid.UUID) int {
	t.Helper()
	var (
		count int
		err   error
	)
	if actor == nil {
		err = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, action).Scan(&count)
	} else {
		err = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ? AND user_id = ?`, action, actor.String()).Scan(&count)
	}
	if err != nil {
		t.Fatalf("count audit rows action=%s: %v", action, err)
	}
	return count
}

// requireAuditRow asserts exactly one audit row for (action, actor) exists.
func requireAuditRow(t *testing.T, db *sql.DB, action string, actor uuid.UUID) {
	t.Helper()
	if got := auditRowsFor(t, db, action, &actor); got != 1 {
		t.Fatalf("audit action=%s actor=%s rows=%d, want 1", action, actor, got)
	}
}

// newA02TestDB opens a one-connection in-memory SQLite DB with the given DDL
// statements applied plus the standard audit_log table.
func newA02TestDB(t *testing.T, ddls ...string) (*sql.DB, repository.Dialect) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1) // ":memory:" is per-connection
	t.Cleanup(func() { _ = db.Close() })
	all := append([]string{`CREATE TABLE audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		project_id TEXT,
		action TEXT,
		environment TEXT,
		details TEXT,
		ip_address TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`}, ddls...)
	for _, ddl := range all {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	return db, repository.NewDialect(repository.DBTypeSQLite)
}

const ddlOrganizations = `CREATE TABLE organizations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const ddlOrgMembers = `CREATE TABLE organization_members (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (organization_id, user_id)
)`

// TestOrganizationService_EmitsAudit walks an organization through its whole
// mutating lifecycle and asserts every action lands in the audit log,
// attributed to the acting admin.
func TestOrganizationService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlOrganizations, ddlOrgMembers)
	svc := NewOrganizationService(
		repository.NewOrganizationRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
	)

	owner := uuid.New()
	org, err := svc.Create("Acme Corp", owner, "10.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	requireAuditRow(t, db, "org.created", owner)

	if _, err := svc.Update(org.ID, owner, "Acme Inc", "10.0.0.1"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	requireAuditRow(t, db, "org.updated", owner)

	target := uuid.New()
	if _, err := svc.AddMember(org.ID, owner, target, "editor", "10.0.0.1"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	requireAuditRow(t, db, "org.member_added", owner)

	if _, err := svc.UpdateMemberRole(org.ID, owner, target, "admin", "10.0.0.1"); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	requireAuditRow(t, db, "org.member_role_updated", owner)

	if err := svc.RemoveMember(org.ID, owner, target, "10.0.0.1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	requireAuditRow(t, db, "org.member_removed", owner)

	if err := svc.Delete(org.ID, owner, "10.0.0.1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	requireAuditRow(t, db, "org.deleted", owner)

	// The org id should be captured in the details payload.
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE action = 'org.created'`).Scan(&details); err != nil {
		t.Fatalf("read org.created details: %v", err)
	}
	if details == "" || details == "null" {
		t.Errorf("org.created details empty: %q", details)
	}
}

const ddlSecretTemplates = `CREATE TABLE secret_templates (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	stack TEXT,
	keys TEXT,
	created_by TEXT,
	organization_id TEXT,
	is_global INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// TestTemplateService_DeleteEmitsAudit covers template.deleted end-to-end over
// the real repository on SQLite. NOTE: TemplateService.Create/Update route a
// models.JSONMap straight into db.Exec via TemplateRepository, and JSONMap's
// Value() returns (interface{}, error) — it does NOT satisfy driver.Valuer, so
// the mattn/go-sqlite3 driver rejects the bare map ("unsupported type
// models.JSONMap, a map"). That is a pre-existing repository limitation that
// only works under Postgres (pq handles the map reflectively); it is unrelated
// to the audit emission added here. We therefore exercise the JSONMap-free
// Delete path end-to-end, which still asserts the emitAudit wiring; the
// created/updated emit uses the identical emitAudit call shape, verified
// end-to-end for the JSONMap-free org and application services above.
func TestTemplateService_DeleteEmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlSecretTemplates)
	svc := NewTemplateService(
		repository.NewTemplateRepository(db, dialect),
		nil, nil, nil,
		repository.NewAuditRepository(db, dialect),
		nil,
	)

	// Seed a template row directly (keys stored as a JSON string, bypassing the
	// JSONMap-through-Exec path the repo Create uses).
	actor := uuid.New()
	tmplID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO secret_templates (id, name, description, stack, keys, created_by, is_global) VALUES (?,?,?,?,?,?,0)`,
		tmplID.String(), "Node App", "desc", "nodejs", `{"keys":[]}`, actor.String(),
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	if err := svc.Delete(tmplID, actor, "127.0.0.1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	requireAuditRow(t, db, "template.deleted", actor)

	// The template id should be captured in the details payload.
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE action = 'template.deleted'`).Scan(&details); err != nil {
		t.Fatalf("read template.deleted details: %v", err)
	}
	if details == "" || details == "null" {
		t.Errorf("template.deleted details empty: %q", details)
	}
}

const ddlApplications = `CREATE TABLE applications (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	description TEXT,
	icon TEXT,
	category TEXT,
	owner_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

// TestApplicationService_EmitsAudit covers application create/update/delete.
func TestApplicationService_EmitsAudit(t *testing.T) {
	db, dialect := newA02TestDB(t, ddlApplications)
	svc := NewApplicationService(
		repository.NewApplicationRepository(db, dialect),
		repository.NewAuditRepository(db, dialect),
	)

	owner := uuid.New()
	app, err := svc.Create("Dashboard", "https://x.example", "d", "", "", owner, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	requireAuditRow(t, db, "application.created", owner)

	if _, err := svc.Update(app.ID, "Dashboard v2", "https://x.example", "d", "🚀", "General", owner, "127.0.0.1"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	requireAuditRow(t, db, "application.updated", owner)

	if err := svc.Delete(app.ID, owner, "127.0.0.1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	requireAuditRow(t, db, "application.deleted", owner)
}
