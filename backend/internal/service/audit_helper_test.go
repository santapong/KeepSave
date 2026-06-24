package service

import (
	"database/sql"
	"sync"
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

// TestEmitAudit_ObserverRecordsOutcome pins A-06: every emit notifies the
// observer with the action and whether the write succeeded, so the audit
// metrics can count attempts and failures.
func TestEmitAudit_ObserverRecordsOutcome(t *testing.T) {
	type rec struct {
		action string
		ok     bool
	}
	var mu sync.Mutex
	var got []rec
	SetAuditObserver(func(action string, ok bool) {
		mu.Lock()
		got = append(got, rec{action, ok})
		mu.Unlock()
	})
	t.Cleanup(func() { SetAuditObserver(nil) })

	// Success: a working repo.
	okRepo, _ := newAuditTestRepo(t)
	emitAudit(okRepo, nil, nil, "secret.created", "", nil, "")

	// Failure: a repo whose DB has no audit_log table, so Create errors.
	failDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = failDB.Close() })
	failRepo := repository.NewAuditRepository(failDB, repository.NewDialect(repository.DBTypeSQLite))
	emitAudit(failRepo, nil, nil, "secret.deleted", "", nil, "")

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("observer calls = %d, want 2: %+v", len(got), got)
	}
	var sawOK, sawFail bool
	for _, r := range got {
		if r.action == "secret.created" && r.ok {
			sawOK = true
		}
		if r.action == "secret.deleted" && !r.ok {
			sawFail = true
		}
	}
	if !sawOK {
		t.Error("missing success observation for secret.created")
	}
	if !sawFail {
		t.Error("missing failure observation for secret.deleted")
	}
}
