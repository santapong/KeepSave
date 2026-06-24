package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// TestSplitSQLStatements_KeepsTriggerIntact guards the migration-runner change
// that made the SQLite statement splitter trigger-aware: a CREATE TRIGGER whose
// BEGIN...END body carries inner semicolons must come out as ONE statement, not
// be cut into broken fragments.
func TestSplitSQLStatements_KeepsTriggerIntact(t *testing.T) {
	sql := `-- a comment
CREATE TRIGGER t BEFORE UPDATE OF x ON tbl
FOR EACH ROW WHEN NEW.x IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'no; really');
END;
INSERT INTO tbl (x) VALUES (1);`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2:\n%q", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "CREATE TRIGGER") || !strings.Contains(stmts[0], "END") {
		t.Errorf("trigger statement was split: %q", stmts[0])
	}
	if !strings.HasPrefix(stmts[1], "INSERT") {
		t.Errorf("second statement = %q, want INSERT", stmts[1])
	}
}

// TestSelfApprovalTrigger_SQLite applies the real migration file through the
// runner's splitter and asserts the trigger blocks setting approved_by equal to
// requested_by (ADR-0007/0017, P-10) while allowing a distinct approver.
func TestSelfApprovalTrigger_SQLite(t *testing.T) {
	db, _ := openSQLite(t)
	if _, err := db.Exec(`CREATE TABLE promotion_requests (
		id TEXT PRIMARY KEY,
		requested_by TEXT NOT NULL,
		approved_by TEXT,
		status TEXT NOT NULL DEFAULT 'pending'
	)`); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("../../migrations/sqlite/011_promotion_self_approval_trigger.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for _, stmt := range splitSQLStatements(string(content)) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying trigger stmt %q: %v", stmt, err)
		}
	}

	requester := uuid.New().String()
	id := uuid.New().String()
	if _, err := db.Exec(`INSERT INTO promotion_requests (id, requested_by, status) VALUES (?, ?, 'pending')`, id, requester); err != nil {
		t.Fatal(err)
	}

	// Self-approval must be rejected by the trigger.
	if _, err := db.Exec(`UPDATE promotion_requests SET status='completed', approved_by=? WHERE id=?`, requester, id); err == nil {
		t.Error("self-approval UPDATE succeeded; trigger did not fire")
	} else if !strings.Contains(err.Error(), "requester cannot approve") {
		t.Errorf("unexpected error %q, want self-approval abort", err)
	}

	// A distinct approver is allowed.
	approver := uuid.New().String()
	if _, err := db.Exec(`UPDATE promotion_requests SET status='completed', approved_by=? WHERE id=?`, approver, id); err != nil {
		t.Errorf("distinct-approver UPDATE failed: %v", err)
	}
}
