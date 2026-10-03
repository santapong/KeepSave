package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/santapong/KeepSave/backend/migrations"
)

func TestSQLiteEmbeddedMigrationsAndPersistence(t *testing.T) {
	url := "sqlite://" + filepath.Join(t.TempDir(), "vault.db")
	db, dialect, err := NewDB(url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrationsFS(db, dialect, migrations.FS); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrationsFS(db, dialect, migrations.FS); err != nil {
		t.Fatalf("migration rerun: %v", err)
	}
	users := NewUserRepository(db, dialect)
	user, err := users.Create("persistence@example.invalid", "not-a-password-hash")
	if err != nil {
		t.Fatal(err)
	}
	if user.CreatedAt.IsZero() {
		t.Fatal("missing persisted timestamp")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, dialect, err = NewDB(url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	loaded, err := NewUserRepository(db, dialect).GetByEmail(user.Email)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != user.ID || !loaded.CreatedAt.Equal(user.CreatedAt) {
		t.Fatal("persisted row changed after reopening")
	}
	// Force the pool to replace its physical connection: connection-local
	// foreign key enforcement must survive, not just the initial startup.
	db.SetMaxIdleConns(0)
	connection, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	var enabled, timeout int
	if err := connection.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatal("replacement connection lost foreign keys")
	}
	if err := connection.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout); err != nil {
		t.Fatal(err)
	}
	if timeout != 5000 {
		t.Fatalf("unexpected busy timeout: %d", timeout)
	}
	var integrity string
	if err := connection.QueryRowContext(context.Background(), "PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatal(err)
	}
	if integrity != "ok" {
		t.Fatal("database integrity check failed")
	}
}

func TestDBTimestampCompatibility(t *testing.T) {
	want := time.Date(2026, 9, 8, 10, 20, 30, 0, time.UTC)
	for _, value := range []interface{}{want, "2026-09-08 10:20:30", "2026-09-08T10:20:30Z", []byte("2026-09-08 10:20:30+00:00")} {
		var got time.Time
		if err := dbTime(&got).Scan(value); err != nil {
			t.Fatal(err)
		}
		if !got.Equal(want) {
			t.Fatal("timestamp changed during decoding")
		}
	}
	var optional *time.Time
	if err := dbTime(&optional).Scan(nil); err != nil || optional != nil {
		t.Fatal("NULL timestamp not preserved")
	}
	var required time.Time
	if err := dbTime(&required).Scan(nil); err == nil {
		t.Fatal("required NULL timestamp accepted")
	}
	if err := dbTime(&required).Scan("not-a-timestamp"); err == nil {
		t.Fatal("malformed timestamp accepted")
	}
}

func TestPoolRejectsInvalidBounds(t *testing.T) {
	for _, pool := range []PoolOptions{{0, 0}, {1, 2}, {1, -1}} {
		if db, _, err := NewDBWithPool(":memory:", pool); err == nil {
			db.Close()
			t.Fatal("invalid pool accepted")
		}
	}
}

func TestSQLiteRejectsMalformedReadOnlyOption(t *testing.T) {
	if _, err := sqliteDSN("file:test.db?mode=ro%ZZ"); err == nil {
		t.Fatal("malformed read-only option silently removed")
	}
}
