package repository

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func newAuditTestDB(t *testing.T) (*sql.DB, Dialect) {
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
		created_at TIMESTAMP NOT NULL
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db, NewDialect(DBTypeSQLite)
}

func insertAuditRow(t *testing.T, db *sql.DB, created time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO audit_log (action, environment, details, ip_address, created_at) VALUES ('test', 'alpha', '{}', '127.0.0.1', ?)`,
		created,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestAuditRepository_DeleteOlderThan_Happy(t *testing.T) {
	db, dialect := newAuditTestDB(t)
	repo := NewAuditRepository(db, dialect)

	now := time.Now().UTC()
	// 2 rows older than the 365-day retention window, 1 inside.
	insertAuditRow(t, db, now.AddDate(0, 0, -400))
	insertAuditRow(t, db, now.AddDate(0, 0, -800))
	insertAuditRow(t, db, now.AddDate(0, 0, -10))

	deleted, err := repo.DeleteOlderThan(365)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want 1", remaining)
	}
}

func TestAuditRepository_DeleteOlderThan_RejectsNonPositive(t *testing.T) {
	db, dialect := newAuditTestDB(t)
	repo := NewAuditRepository(db, dialect)

	for _, days := range []int{0, -1, -365} {
		if _, err := repo.DeleteOlderThan(days); err == nil {
			t.Errorf("DeleteOlderThan(%d): expected error, got nil", days)
		}
	}
}

func TestAuditRepository_DeleteOlderThan_EmptyTable(t *testing.T) {
	db, dialect := newAuditTestDB(t)
	repo := NewAuditRepository(db, dialect)

	deleted, err := repo.DeleteOlderThan(30)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 on empty table", deleted)
	}
}

func TestAuditRepository_DeleteOlderThan_BoundaryExact(t *testing.T) {
	db, dialect := newAuditTestDB(t)
	repo := NewAuditRepository(db, dialect)

	// A row whose age equals the retention window should be kept (strict <).
	// Insert slightly inside and slightly outside the boundary.
	now := time.Now().UTC()
	insertAuditRow(t, db, now.AddDate(0, 0, -30).Add(1*time.Minute)) // inside
	insertAuditRow(t, db, now.AddDate(0, 0, -30).Add(-1*time.Minute)) // outside

	deleted, err := repo.DeleteOlderThan(30)
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}
