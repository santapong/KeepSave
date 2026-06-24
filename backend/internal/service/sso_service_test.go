package service

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func newOrgRepoWithMembers(t *testing.T) (*repository.OrganizationRepository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE organization_members (
		id TEXT PRIMARY KEY,
		organization_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	return repository.NewOrganizationRepository(db, dialect), db
}

func addMember(t *testing.T, db *sql.DB, orgID, userID uuid.UUID, role string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO organization_members (id, organization_id, user_id, role) VALUES (?, ?, ?, ?)`,
		uuid.New().String(), orgID.String(), userID.String(), role,
	); err != nil {
		t.Fatalf("add member: %v", err)
	}
}

func TestRequireOrgRole(t *testing.T) {
	orgRepo, db := newOrgRepoWithMembers(t)
	org := uuid.New()
	admin := uuid.New()
	viewer := uuid.New()
	stranger := uuid.New()
	addMember(t, db, org, admin, "admin")
	addMember(t, db, org, viewer, "viewer")

	cases := []struct {
		name     string
		user     uuid.UUID
		required string
		wantErr  bool
	}{
		{"admin meets admin", admin, "admin", false},
		{"admin meets viewer", admin, "viewer", false},
		{"viewer meets viewer", viewer, "viewer", false},
		{"viewer fails admin", viewer, "admin", true},
		{"stranger fails viewer", stranger, "viewer", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireOrgRole(orgRepo, org, tc.user, tc.required)
			if tc.wantErr {
				if !errors.Is(err, ErrOrgAccessDenied) {
					t.Errorf("err = %v, want ErrOrgAccessDenied", err)
				}
			} else if err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

// TestConfigureSSO_RequiresAdmin proves the SSO management method enforces org
// admin before touching crypto/storage: a non-admin is rejected up-front, so
// nil ssoRepo/cryptoSvc are never reached (AUTH-03).
func TestConfigureSSO_RequiresAdmin(t *testing.T) {
	orgRepo, db := newOrgRepoWithMembers(t)
	org := uuid.New()
	viewer := uuid.New()
	addMember(t, db, org, viewer, "viewer")

	svc := NewSSOService(nil, orgRepo, nil, nil)

	if _, err := svc.ConfigureSSO(org, viewer, "oidc", "https://idp", "client", "secret", nil, ""); !errors.Is(err, ErrOrgAccessDenied) {
		t.Fatalf("ConfigureSSO as viewer err = %v, want ErrOrgAccessDenied", err)
	}
	stranger := uuid.New()
	if _, err := svc.ConfigureSSO(org, stranger, "oidc", "https://idp", "client", "secret", nil, ""); !errors.Is(err, ErrOrgAccessDenied) {
		t.Fatalf("ConfigureSSO as non-member err = %v, want ErrOrgAccessDenied", err)
	}
}
