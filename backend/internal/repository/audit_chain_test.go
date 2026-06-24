package repository

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// newChainAuditRepo builds a prod-shaped audit_log (TEXT id + chain columns) and
// a keyed AuditRepository, so the ADR-0019 chain runs end-to-end on SQLite.
func newChainAuditRepo(t *testing.T) (*AuditRepository, *sql.DB) {
	t.Helper()
	db, dialect := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE audit_log (
		id TEXT PRIMARY KEY,
		user_id TEXT, project_id TEXT, action TEXT, environment TEXT,
		details TEXT, ip_address TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		prev_hash TEXT, entry_hash TEXT
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	repo := NewAuditRepository(db, dialect)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	repo.SetChainKey(key)
	return repo, db
}

func auditRow(t *testing.T, repo *AuditRepository, action string, n int) {
	t.Helper()
	u, p := uuid.New(), uuid.New()
	if err := repo.Create(&u, &p, action, "alpha", models.JSONMap{"n": n, "k": "v"}, "127.0.0.1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestAuditChain_LinksAndVerifies(t *testing.T) {
	repo, db := newChainAuditRepo(t)
	auditRow(t, repo, "secret.created", 0)
	auditRow(t, repo, "secret.updated", 1)
	auditRow(t, repo, "secret.deleted", 2)

	rows, err := db.Query(`SELECT prev_hash, entry_hash FROM audit_log ORDER BY rowid ASC`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var prevs, entries []string
	for rows.Next() {
		var p, e string
		if err := rows.Scan(&p, &e); err != nil {
			t.Fatal(err)
		}
		if e == "" {
			t.Error("entry_hash empty on a chained row")
		}
		prevs = append(prevs, p)
		entries = append(entries, e)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d rows, want 3", len(entries))
	}
	if prevs[0] != "" {
		t.Errorf("genesis prev_hash = %q, want empty", prevs[0])
	}
	if prevs[1] != entries[0] || prevs[2] != entries[1] {
		t.Error("chain links are not contiguous")
	}

	if broken, err := repo.VerifyChain(); err != nil || broken != nil {
		t.Errorf("VerifyChain = %v, %v; want nil, nil", broken, err)
	}
}

func TestAuditChain_DetectsTamperedDetails(t *testing.T) {
	repo, db := newChainAuditRepo(t)
	auditRow(t, repo, "secret.created", 0)
	auditRow(t, repo, "target", 1)
	auditRow(t, repo, "secret.deleted", 2)

	var id string
	if err := db.QueryRow(`SELECT id FROM audit_log WHERE action='target'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	// Tamper: change the stored details without touching entry_hash.
	if _, err := db.Exec(`UPDATE audit_log SET details='{"n":99,"k":"HACKED"}' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}

	broken, err := repo.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain err: %v", err)
	}
	if broken == nil || broken.String() != id {
		t.Errorf("VerifyChain broken = %v, want %s", broken, id)
	}
}

func TestAuditChain_DetectsDeletion(t *testing.T) {
	repo, db := newChainAuditRepo(t)
	auditRow(t, repo, "secret.created", 0)
	auditRow(t, repo, "middle", 1)
	auditRow(t, repo, "secret.deleted", 2)

	if _, err := db.Exec(`DELETE FROM audit_log WHERE action='middle'`); err != nil {
		t.Fatal(err)
	}
	broken, err := repo.VerifyChain()
	if err != nil {
		t.Fatalf("VerifyChain err: %v", err)
	}
	if broken == nil {
		t.Error("VerifyChain did not detect the deleted row")
	}
}

func TestAuditChain_DeleteOlderThanDisabledWhenKeyed(t *testing.T) {
	repo, _ := newChainAuditRepo(t)
	if _, err := repo.DeleteOlderThan(30); err == nil {
		t.Error("DeleteOlderThan should error while the chain is active")
	}
}

// TestAuditChain_TipResumesAcrossRepos proves a fresh repo over the same DB
// picks up the existing chain tip, so appends stay contiguous across restarts.
func TestAuditChain_TipResumesAcrossRepos(t *testing.T) {
	repoA, db := newChainAuditRepo(t)
	auditRow(t, repoA, "a", 0)
	auditRow(t, repoA, "b", 1)

	repoB := NewAuditRepository(db, NewDialect(DBTypeSQLite))
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	repoB.SetChainKey(key)
	auditRow(t, repoB, "c", 2)

	if broken, err := repoB.VerifyChain(); err != nil || broken != nil {
		t.Errorf("VerifyChain after resume = %v, %v; want nil, nil", broken, err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entry_hash IS NOT NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("chained rows = %d, want 3", n)
	}
}
