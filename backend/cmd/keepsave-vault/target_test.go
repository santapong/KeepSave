package main

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func TestPlatformRecoveryTargetPreflight(t *testing.T) {
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("isolated synthetic PostgreSQL harness required")
	}
	const fixture = "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
	admin, _, err := repository.NewDB(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := "recovery_preflight_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(`DROP DATABASE ` + name); e != nil {
			t.Error(e)
		}
	}()
	u, _ := url.Parse(fixture)
	u.Path = "/" + name
	db, _, err := repository.NewDB(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := lockFreshRecoveryTarget(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if unexpected, err := lockFreshRecoveryTarget(ctx, db); err == nil {
		unexpected()
		t.Fatal("concurrent operator recovery admitted")
	}
	release()
	if _, err = db.Exec(`CREATE TABLE occupied_target(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if unexpected, err := lockFreshRecoveryTarget(ctx, db); err == nil {
		unexpected()
		t.Fatal("existing schema admitted before migrations")
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("refused target was mutated", err, count)
	}
}
