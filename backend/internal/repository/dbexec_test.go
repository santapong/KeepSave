package repository

import (
	"bytes"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// openSQLite returns an in-memory SQLite handle pinned to a single connection.
// ":memory:" gives each *connection* its own database, so without the cap a
// pooled second connection would see an empty schema; MaxOpenConns(1) keeps
// every statement on one database for deterministic tests.
func openSQLite(t *testing.T) (*sql.DB, Dialect) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db, NewDialect(DBTypeSQLite)
}

// TestExecQ_ReordersOutOfOrderPlaceholders is the unit-level guard for DB-12:
// a statement whose placeholders are not in 1..N order must still bind its
// args correctly on SQLite. Before ExecQ, db.Exec(Q(...), args...) bound the
// anonymous `?` positionally and silently updated nothing.
func TestExecQ_ReordersOutOfOrderPlaceholders(t *testing.T) {
	db, dialect := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE t (id TEXT PRIMARY KEY, a TEXT, b TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO t (id, a, b) VALUES ('row1','old_a','old_b')`); err != nil {
		t.Fatal(err)
	}
	res, err := ExecQ(db, dialect, `UPDATE t SET a = $2, b = $3 WHERE id = $1`, "row1", "new_a", "new_b")
	if err != nil {
		t.Fatalf("ExecQ: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("rows affected = %d, want 1 (mis-bound placeholders)", n)
	}
	var a, b string
	if err := db.QueryRow(`SELECT a, b FROM t WHERE id='row1'`).Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	if a != "new_a" || b != "new_b" {
		t.Errorf("after ExecQ a=%q b=%q, want new_a/new_b", a, b)
	}
}

// TestSecretRepository_UpdatePersists pins that SecretRepository.Update
// actually writes on a non-Postgres dialect (the DB-12 silent no-op).
func TestSecretRepository_UpdatePersists(t *testing.T) {
	db, dialect := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE secrets (
		id TEXT PRIMARY KEY, project_id TEXT, environment_id TEXT, key TEXT,
		encrypted_value BLOB, value_nonce BLOB,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	repo := NewSecretRepository(db, dialect)
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO secrets (id, project_id, environment_id, key, encrypted_value, value_nonce) VALUES (?,?,?,?,?,?)`,
		id.String(), uuid.New().String(), uuid.New().String(), "K", []byte("OLDCIPHER"), []byte("OLDNONCE")); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Update(id, []byte("NEWCIPHER"), []byte("NEWNONCE")); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var enc, nonce []byte
	if err := db.QueryRow(`SELECT encrypted_value, value_nonce FROM secrets WHERE id=?`, id.String()).Scan(&enc, &nonce); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(enc, []byte("NEWCIPHER")) || !bytes.Equal(nonce, []byte("NEWNONCE")) {
		t.Errorf("Update did not persist: enc=%q nonce=%q", enc, nonce)
	}
}

// TestProjectRepository_UpdateDEKPersists pins UpdateDEK — the method whose
// DB-12 no-op silently masked the key-rotation test (rotation appeared to work
// only because neither the DEK nor the secrets actually changed).
func TestProjectRepository_UpdateDEKPersists(t *testing.T) {
	db, dialect := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE projects (
		id TEXT PRIMARY KEY, name TEXT, description TEXT, owner_id TEXT,
		encrypted_dek BLOB, dek_nonce BLOB,
		allowed_origins TEXT NOT NULL DEFAULT '[]', embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	repo := NewProjectRepository(db, dialect)
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?,?,'',?,?,?)`,
		id.String(), "p", uuid.New().String(), []byte("OLDDEK"), []byte("OLDN")); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateDEK(id, []byte("NEWDEK"), []byte("NEWN")); err != nil {
		t.Fatalf("UpdateDEK: %v", err)
	}
	var dek, nonce []byte
	if err := db.QueryRow(`SELECT encrypted_dek, dek_nonce FROM projects WHERE id=?`, id.String()).Scan(&dek, &nonce); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dek, []byte("NEWDEK")) || !bytes.Equal(nonce, []byte("NEWN")) {
		t.Errorf("UpdateDEK did not persist: dek=%q nonce=%q", dek, nonce)
	}
}

// TestOrganizationRepository_UpdateMemberRolePersists covers an out-of-order
// placeholder with a two-column WHERE ( SET role=$3 WHERE org=$1 AND user=$2 ).
func TestOrganizationRepository_UpdateMemberRolePersists(t *testing.T) {
	db, dialect := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE organization_members (
		id TEXT PRIMARY KEY, organization_id TEXT, user_id TEXT, role TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	repo := NewOrganizationRepository(db, dialect)
	org, user := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role) VALUES (?,?,?,?)`,
		uuid.New().String(), org.String(), user.String(), "member"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateMemberRole(org, user, "admin"); err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}
	var role string
	if err := db.QueryRow(`SELECT role FROM organization_members WHERE organization_id=? AND user_id=?`, org.String(), user.String()).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Errorf("UpdateMemberRole did not persist: role=%q, want admin", role)
	}
}
