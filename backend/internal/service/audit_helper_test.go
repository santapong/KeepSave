package service

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// newAuditTestRepo builds an in-memory SQLite-backed AuditRepository so
// service-level tests can assert that audit rows are written without
// standing up Postgres. Mirrors the pattern in
// internal/repository/audit_repo_test.go::newAuditTestDB.
func newAuditTestRepo(t *testing.T) (*repository.AuditRepository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		project_id TEXT,
		action TEXT,
		environment TEXT,
		details TEXT,
		ip_address TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	return repository.NewAuditRepository(db, dialect), db
}

// TestEmitAudit_WritesRow exercises the helper directly: a row appears with
// the canonical action and details, and a nil repo is silently ignored.
func TestEmitAudit_WritesRow(t *testing.T) {
	repo, db := newAuditTestRepo(t)
	actor := uuid.New()
	project := uuid.New()

	emitAudit(repo, &actor, &project, "secret.created", "alpha",
		models.JSONMap{"secret_key": "API_TOKEN"}, "127.0.0.1")

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, "secret.created").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d audit rows, want 1", count)
	}
}

func TestEmitAudit_NilRepoIsNoOp(t *testing.T) {
	// Must not panic, must not error - simply does nothing.
	emitAudit(nil, nil, nil, "noop", "", nil, "")
}
