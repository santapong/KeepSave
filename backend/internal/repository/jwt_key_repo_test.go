package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santapong/KeepSave/backend/internal/auth"
)

// loadMigration013SQL reads the real SQLite jwt_keys migration so the test
// exercises the shipped DDL (CHECK constraint + partial unique index), not a
// hand-rolled copy that could drift from production.
func loadMigration013SQL(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "migrations", "sqlite", "013_jwt_keys.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading migration 013: %v", err)
	}
	return string(content)
}

// TestJWTKeyRepository_RoundTrip exercises the JWTKeyRepository against the real
// SQLite migration: list filters retired keys, SetKeyStatus persists (DB-12
// out-of-order placeholders: SET status=$2, retired_at=$3 WHERE kid=$1), and
// the partial unique index enforces a single signing key.
func TestJWTKeyRepository_RoundTrip(t *testing.T) {
	db, dialect := openSQLite(t)
	for _, stmt := range splitSQLStatements(loadMigration013SQL(t)) {
		if stmt = strings.TrimSpace(stmt); stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying migration stmt: %v\nSQL: %s", err, stmt)
		}
	}
	repo := NewJWTKeyRepository(db, dialect)

	mustInsert := func(kid, status string) {
		t.Helper()
		if err := repo.InsertKey(auth.StoredKey{
			Kid:                 kid,
			Alg:                 "RS256",
			PublicKeyDER:        []byte("pub-" + kid),
			PrivateKeyEncrypted: []byte("enc-" + kid),
			PrivateKeyNonce:     []byte("nonce-" + kid),
			Status:              status,
		}); err != nil {
			t.Fatalf("InsertKey(%s,%s): %v", kid, status, err)
		}
	}

	mustInsert("kid-sign", "signing")
	mustInsert("kid-verify", "verifying")
	mustInsert("kid-retired", "retired")

	keys, err := repo.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("ListKeys returned %d keys, want 2 (retired filtered)", len(keys))
	}
	for _, k := range keys {
		if k.Status == "retired" {
			t.Errorf("ListKeys returned a retired key: %s", k.Kid)
		}
		if k.Kid == "kid-sign" && string(k.PublicKeyDER) != "pub-kid-sign" {
			t.Errorf("public key did not round-trip: %q", k.PublicKeyDER)
		}
	}

	// The partial unique index must reject a second signing key while one is
	// already signing. (Checked before retiring kid-sign below — once it is
	// retired, a fresh signing key is legitimate.)
	if err := repo.InsertKey(auth.StoredKey{Kid: "kid-sign-2", Alg: "RS256", Status: "signing"}); err == nil {
		t.Error("inserting a second signing key succeeded; partial unique index not enforced")
	}

	// DB-12 guard: SetKeyStatus must actually persist on SQLite.
	if err := repo.SetKeyStatus("kid-sign", "retired"); err != nil {
		t.Fatalf("SetKeyStatus: %v", err)
	}
	keys, err = repo.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys after retire: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("after retiring kid-sign, ListKeys=%d, want 1 (SetKeyStatus no-op?)", len(keys))
	}
	if keys[0].Kid != "kid-verify" {
		t.Errorf("surviving key=%q, want kid-verify", keys[0].Kid)
	}
	var retiredAt *string
	if err := db.QueryRow(`SELECT retired_at FROM jwt_keys WHERE kid='kid-sign'`).Scan(&retiredAt); err != nil {
		t.Fatalf("reading retired_at: %v", err)
	}
	if retiredAt == nil {
		t.Error("retired_at not set after SetKeyStatus(retired)")
	}
}
