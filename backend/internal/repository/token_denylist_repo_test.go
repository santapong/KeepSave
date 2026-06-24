package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadSQLiteMigration reads a real SQLite migration file so tests exercise the
// shipped DDL rather than a hand-rolled copy.
func loadSQLiteMigration(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "migrations", "sqlite", name))
	if err != nil {
		t.Fatalf("reading migration %s: %v", name, err)
	}
	return string(content)
}

// TestTokenDenylistRepository_CacheAndPrune covers the boot-seed/refresh path
// (a row revoked by "another instance" becomes visible only after RefreshCache)
// and DeleteExpired pruning, against the real 014 migration DDL.
func TestTokenDenylistRepository_CacheAndPrune(t *testing.T) {
	db, dialect := openSQLite(t)
	for _, stmt := range splitSQLStatements(loadSQLiteMigration(t, "014_token_denylist.sql")) {
		if stmt = strings.TrimSpace(stmt); stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying migration stmt: %v\nSQL: %s", err, stmt)
		}
	}

	// Simulate another instance writing a revocation directly to the table.
	if _, err := db.Exec(
		`INSERT INTO token_denylist (jti, expires_at) VALUES (?, ?)`,
		"other-instance-jti", time.Now().Add(time.Hour).UTC(),
	); err != nil {
		t.Fatalf("seed denylist: %v", err)
	}

	repo := NewTokenDenylistRepository(db, dialect)
	// A fresh repo has an empty cache: the externally-written jti is not yet seen.
	if revoked, err := repo.IsTokenRevoked("other-instance-jti", nil); err != nil || revoked {
		t.Fatalf("pre-refresh revoked=%v err=%v, want false/nil", revoked, err)
	}
	if err := repo.RefreshCache(); err != nil {
		t.Fatalf("RefreshCache: %v", err)
	}
	if revoked, err := repo.IsTokenRevoked("other-instance-jti", nil); err != nil || !revoked {
		t.Fatalf("post-refresh revoked=%v err=%v, want true/nil", revoked, err)
	}

	// Write-through: Revoke is visible immediately without a refresh.
	if err := repo.Revoke("local-jti", nil, time.Now().Add(time.Hour), "test"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if revoked, _ := repo.IsTokenRevoked("local-jti", nil); !revoked {
		t.Error("write-through revoke not visible")
	}

	// An already-expired row prunes and drops out of the refreshed cache.
	if err := repo.Revoke("expired-jti", nil, time.Now().Add(-time.Hour), "test"); err != nil {
		t.Fatalf("Revoke expired: %v", err)
	}
	n, err := repo.DeleteExpired()
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("DeleteExpired removed %d rows, want 1", n)
	}
	if err := repo.RefreshCache(); err != nil {
		t.Fatalf("RefreshCache after prune: %v", err)
	}
	if revoked, _ := repo.IsTokenRevoked("expired-jti", nil); revoked {
		t.Error("expired jti still cached after prune+refresh")
	}
	// The live ones survive.
	if revoked, _ := repo.IsTokenRevoked("local-jti", nil); !revoked {
		t.Error("live jti dropped by prune")
	}
}
