package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/migrations"
	"os"
	"testing"
)

// Opt-in integration check for the disposable Docker service documented in the
// validation report. No configurable DSN: this cannot target a real vault.
func TestSocialAuthPostgresIntegration(t *testing.T) {
	if os.Getenv("KEEPSAVE_SOCIAL_POSTGRES_TEST") != "1" {
		t.Skip("requires isolated keepsave-social-postgres-test Docker service")
	}
	db, d, err := NewDB("postgres://keepsave_social_test:local-test-only@keepsave-social-postgres-test:5432/keepsave_social_test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = RunMigrationsFS(db, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	repo := NewSocialAuthRepository(db, d)
	ctx := context.Background()
	user, err := NewUserRepository(db, d).Create(uuid.NewString()+"@example.invalid", "!")
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []*uuid.UUID{nil, &user.ID} {
		hash := uuid.NewString()
		challenge := "abcdefghijklmnopqrstuvwxyz01234567890123456"
		if err = repo.CreateFlow(ctx, hash, "github", challenge, owner); err != nil {
			t.Fatal(err)
		}
		if err = repo.ConsumeFlow(ctx, hash, "github", challenge, owner); err != nil {
			t.Fatal(err)
		}
		if err = repo.ConsumeFlow(ctx, hash, "github", challenge, owner); err == nil {
			t.Fatal("replayed flow")
		}
	}
	subject := uuid.NewString()
	created, err := repo.Resolve(ctx, "google", subject, uuid.NewString()+"@example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := repo.Resolve(ctx, "google", subject, "changed@example.invalid", nil)
	if err != nil || again.ID != created.ID {
		t.Fatal("stable identity resolution failed", err)
	}
	if _, err = repo.Resolve(ctx, "google", subject, "changed@example.invalid", &user.ID); err != ErrSocialConflict {
		t.Fatal("identity moved")
	}
}
