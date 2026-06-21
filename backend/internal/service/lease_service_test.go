package service

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func newLeaseTestService(t *testing.T) (*LeaseService, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE secret_leases (
		id TEXT PRIMARY KEY,
		api_key_id TEXT,
		project_id TEXT NOT NULL,
		environment TEXT,
		secret_keys TEXT,
		granted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP,
		revoked INTEGER NOT NULL DEFAULT 0,
		revoked_at TIMESTAMP
	)`); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	return NewLeaseService(db, dialect, nil), db
}

// TestRevokeLease_ScopedToProject verifies AUTH-04: a lease can only be revoked
// through the project it belongs to. A revoke aimed at another project must
// leave the lease active and report ErrLeaseNotFound.
func TestRevokeLease_ScopedToProject(t *testing.T) {
	svc, db := newLeaseTestService(t)
	projectA := uuid.New()
	projectB := uuid.New()
	leaseID := uuid.New()
	if _, err := db.Exec(
		`INSERT INTO secret_leases (id, api_key_id, project_id, environment, secret_keys, expires_at, revoked)
		 VALUES (?, ?, ?, 'alpha', '[]', CURRENT_TIMESTAMP, 0)`,
		leaseID.String(), uuid.New().String(), projectA.String(),
	); err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	isRevoked := func() bool {
		var v int
		if err := db.QueryRow(`SELECT revoked FROM secret_leases WHERE id = ?`, leaseID.String()).Scan(&v); err != nil {
			t.Fatalf("query revoked: %v", err)
		}
		return v != 0
	}

	// Wrong project: no-op + not-found.
	if err := svc.RevokeLease(leaseID, projectB, uuid.New(), ""); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("cross-project revoke err = %v, want ErrLeaseNotFound", err)
	}
	if isRevoked() {
		t.Fatal("lease was revoked by a cross-project caller")
	}

	// Owning project: revoked.
	if err := svc.RevokeLease(leaseID, projectA, uuid.New(), ""); err != nil {
		t.Fatalf("same-project revoke err = %v, want nil", err)
	}
	if !isRevoked() {
		t.Fatal("lease not revoked by its owning project")
	}
}
