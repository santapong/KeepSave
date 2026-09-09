package repository

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/models"
)

func newScopeTestDB(t *testing.T) (*sql.DB, Dialect) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, ddl := range []string{
		`CREATE TABLE secret_templates (
			id TEXT PRIMARY KEY, name TEXT, description TEXT, stack TEXT, keys TEXT,
			created_by TEXT, organization_id TEXT, is_global INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE organization_members (organization_id TEXT NOT NULL, user_id TEXT NOT NULL, role TEXT)`,
		`CREATE TABLE applications (
			id TEXT PRIMARY KEY, name TEXT, url TEXT, description TEXT, icon TEXT, category TEXT,
			owner_id TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	return db, NewDialect(DBTypeSQLite)
}

// TestTemplateRepo_AccessScoping proves the read IDOR (GetByIDForUser) and write
// IDOR (Update/Delete owner-binding) fixes for templates.
func TestTemplateRepo_AccessScoping(t *testing.T) {
	db, dialect := newScopeTestDB(t)
	repo := NewTemplateRepository(db, dialect)

	owner, other, orgMember := uuid.New(), uuid.New(), uuid.New()
	org := uuid.New()
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES (?,?,?)`, org.String(), orgMember.String(), "viewer"); err != nil {
		t.Fatalf("seed member: %v", err)
	}

	keys := models.JSONMap{"keys": []interface{}{}}
	priv, err := repo.Create("priv", "", "go", keys, owner, nil, false)
	if err != nil {
		t.Fatalf("create priv: %v", err)
	}
	global, err := repo.Create("glob", "", "go", keys, owner, nil, true)
	if err != nil {
		t.Fatalf("create global: %v", err)
	}
	orgTmpl, err := repo.Create("org", "", "go", keys, owner, &org, false)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}

	// --- Read access ---
	mustRead := func(id, uid uuid.UUID, want bool) {
		_, err := repo.GetByIDForUser(id, uid)
		if want && err != nil {
			t.Errorf("expected readable, got %v", err)
		}
		if !want && !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("expected ErrNoRows, got %v", err)
		}
	}
	mustRead(priv.ID, owner, true)        // owner reads own private
	mustRead(priv.ID, other, false)       // stranger cannot read private (the IDOR)
	mustRead(global.ID, other, true)      // global readable by anyone
	mustRead(orgTmpl.ID, orgMember, true) // org member reads org template
	mustRead(orgTmpl.ID, other, false)    // non-member cannot

	// --- Write access (owner-only) ---
	if _, err := repo.Update(priv.ID, "x", "", "go", keys, other); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("non-owner Update should be ErrNoRows, got %v", err)
	}
	if _, err := repo.Update(priv.ID, "x", "", "go", keys, owner); err != nil {
		t.Errorf("owner Update: %v", err)
	}
	if err := repo.Delete(priv.ID, other); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("non-owner Delete should be ErrNoRows, got %v", err)
	}
	if err := repo.Delete(priv.ID, owner); err != nil {
		t.Errorf("owner Delete: %v", err)
	}
}

// TestApplicationRepo_GetByOwner proves the application read-IDOR fix.
func TestApplicationRepo_GetByOwner(t *testing.T) {
	db, dialect := newScopeTestDB(t)
	repo := NewApplicationRepository(db, dialect)

	owner, other := uuid.New(), uuid.New()
	app := &models.Application{Name: "a", URL: "https://x", Icon: "x", Category: "c", OwnerID: owner}
	if err := repo.Create(app); err != nil {
		t.Fatalf("create app: %v", err)
	}

	if _, err := repo.GetByOwner(app.ID, owner); err != nil {
		t.Errorf("owner GetByOwner: %v", err)
	}
	if _, err := repo.GetByOwner(app.ID, other); err == nil {
		t.Error("stranger GetByOwner should fail (read IDOR)")
	}
}
