package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// TestAccessPolicyRepo_DeleteBoundToProject proves the IDOR fix: a policy can
// only be deleted through the project it belongs to; deleting via another
// (authorized) project id affects nothing and returns sql.ErrNoRows.
func TestAccessPolicyRepo_DeleteBoundToProject(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE access_policies (
		id TEXT PRIMARY KEY, project_id TEXT, policy_type TEXT, config TEXT,
		enabled INTEGER, created_by TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	repo := NewAccessPolicyRepository(db, NewDialect(DBTypeSQLite))

	projA, projB := uuid.New(), uuid.New()
	policyB := uuid.New()
	if _, err := db.Exec(`INSERT INTO access_policies (id, project_id, policy_type, config, enabled, created_by) VALUES (?,?,?,?,?,?)`,
		policyB.String(), projB.String(), "ip_restriction", `{}`, 1, uuid.New().String()); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	// Deleting projB's policy through projA must not touch it.
	if err := repo.DeletePolicy(policyB, projA); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("cross-project DeletePolicy: err = %v, want sql.ErrNoRows", err)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM access_policies WHERE id = ?`, policyB.String()).Scan(&count)
	if count != 1 {
		t.Errorf("policy should be intact after cross-project delete, count = %d", count)
	}

	// Through its own project it succeeds.
	if err := repo.DeletePolicy(policyB, projB); err != nil {
		t.Errorf("own-project DeletePolicy: %v", err)
	}
}
