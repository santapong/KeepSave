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
	db, dialect := newA02TestDB(t, ddlSecretLeases)
	return NewLeaseService(db, dialect, repository.NewAuditRepository(db, dialect)), db
}

// TestRevokeLease_ScopedToProject verifies AUTH-04: a lease can only be revoked
// through the project it belongs to. A revoke aimed at another project must
// leave the lease active and report ErrLeaseNotFound.
func TestRevokeLease_ScopedToProject(t *testing.T) {
	svc, db := newLeaseTestService(t)
	projectA := uuid.New()
	projectB := uuid.New()
	leaseID := uuid.New()
	actor := uuid.New()
	seedLeaseParent(t, db, actor, projectA)
	if _, err := db.Exec(
		`INSERT INTO secret_leases (id, api_key_id, project_id, environment, secret_keys, expires_at, revoked)
		 VALUES (?, ?, ?, 'alpha', '[]', CURRENT_TIMESTAMP, 0)`,
		leaseID.String(), actor.String(), projectA.String(),
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
	if err := svc.RevokeLease(leaseID, projectB, actor, ""); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatalf("cross-project revoke err = %v, want ErrLeaseNotFound", err)
	}
	if isRevoked() {
		t.Fatal("lease was revoked by a cross-project caller")
	}

	// Owning project: revoked.
	if err := svc.RevokeLease(leaseID, projectA, actor, ""); err != nil {
		t.Fatalf("same-project revoke err = %v, want nil", err)
	}
	if !isRevoked() {
		t.Fatal("lease not revoked by its owning project")
	}
}

// Legacy service fixtures deliberately use the same UUID for key and actor;
// real-router integration tests assert the distinct database identities.
func seedLeaseParent(t *testing.T, db *sql.DB, id, project uuid.UUID) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS agent_token_issuance(jti TEXT PRIMARY KEY,lease_id TEXT,project_id TEXT,user_id TEXT,expires_at TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{`CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY,owner_id TEXT,organization_id TEXT,deleted_at TIMESTAMP)`, `CREATE TABLE IF NOT EXISTS organization_members(organization_id TEXT,user_id TEXT,role TEXT)`} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO projects(id,owner_id) VALUES(?,?)`, project, id); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS api_keys (id TEXT PRIMARY KEY,name TEXT,hashed_key TEXT,user_id TEXT,project_id TEXT,scopes TEXT,environment TEXT,expires_at TIMESTAMP,created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO api_keys(id,name,hashed_key,user_id,project_id,scopes) VALUES(?, 'fixture', 'unused', ?, ?, '["read","write"]')`, id, id, project)
	if err != nil {
		t.Fatal(err)
	}
}
