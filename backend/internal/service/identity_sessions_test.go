package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/migrations"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func identityDatabase(t *testing.T, source fs.FS) (*sql.DB, repository.Dialect) {
	t.Helper()
	dsn := "sqlite://" + filepath.Join(t.TempDir(), "identity.db")
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") == "1" {
		dsn = "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
		admin, _, err := repository.NewDB(dsn)
		if err != nil {
			t.Fatal(err)
		}
		schema := "identity_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = admin.Close() })
		dsn += "&search_path=" + schema + ",public"
	}
	db, d, err := repository.NewDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = repository.RunMigrationsFS(db, d, source); err != nil {
		t.Fatal(err)
	}
	return db, d
}
func identityFixture(t *testing.T) (*AuthService, *SessionService, *auth.JWTService, *sql.DB, repository.Dialect) {
	t.Helper()
	db, d := identityDatabase(t, migrations.FS)
	a := repository.NewAuditRepository(db, d)
	a.SetChainKey([]byte("synthetic-identity-audit-key-only"))
	j := auth.NewJWTService("synthetic-session-signing-key-only")
	sessions := NewSessionService(db, d, j, a)
	j.EnableHumanSessions(sessions)
	s := NewAuthService(repository.NewUserRepository(db, d), repository.NewAuthAttemptsRepository(db, d), a, j)
	s.EnableSessions(sessions)
	return s, sessions, j, db, d
}

const identityPassword = "Synthetic-Only1!"

func TestIdentityCanonicalPasswordsAndAuthoritativeRevocation(t *testing.T) {
	s, sessions, j, db, d := identityFixture(t)
	ctx := context.Background()
	first, err := s.RegisterContext(ctx, " Owner@Example.invalid ", identityPassword, "127.0.0.1", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Register("owner@example.invalid", identityPassword); !errors.Is(err, repository.ErrUserExists) {
		t.Fatal("canonical duplicate accepted", err)
	}
	second, err := s.Login("OWNER@example.INVALID", identityPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	a, err := j.ValidateToken(first.Token)
	if err != nil {
		t.Fatal(err)
	}
	b, err := j.ValidateToken(second.Token)
	if err != nil {
		t.Fatal(err)
	}
	if a.SessionID == b.SessionID || a.ID != a.SessionID || a.TokenType != "human" {
		t.Fatal("session fixation or invalid claims")
	}
	if a.ExpiresAt.Time.After(time.Now().Add(24 * time.Hour)) {
		t.Fatal("overlong session")
	}
	current := uuid.MustParse(b.SessionID)
	old := uuid.MustParse(a.SessionID)
	listed, err := sessions.List(ctx, first.User.ID, current)
	if err != nil || len(listed) != 2 {
		t.Fatal("session list", err)
	}
	payload, _ := json.Marshal(listed)
	if strings.Contains(string(payload), first.Token) || strings.Contains(string(payload), "token_hash") {
		t.Fatal("session credentials exposed")
	}
	foreign, err := s.Register("foreign@example.invalid", identityPassword)
	if err != nil {
		t.Fatal(err)
	}
	fc, _ := j.ValidateToken(foreign.Token)
	if err = sessions.Revoke(ctx, first.User.ID, current, uuid.MustParse(fc.SessionID), ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("cross-account revocation", err)
	}
	if err = sessions.Revoke(ctx, first.User.ID, current, old, ""); err != nil {
		t.Fatal(err)
	}
	independent := auth.NewJWTService("synthetic-session-signing-key-only")
	independent.EnableHumanSessions(NewSessionService(db, d, independent, s.auditRepo))
	if _, err = independent.ValidateToken(first.Token); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatal("other instance accepted revoked session", err)
	}
	if _, err = j.ValidateToken(second.Token); err != nil {
		t.Fatal("revoked sibling", err)
	}
	var count int
	if err = db.QueryRow(repository.Q(d, `SELECT COUNT(*) FROM audit_log WHERE action='auth.session_revoked'`)).Scan(&count); err != nil || count != 1 {
		t.Fatal("missing revoke audit", err)
	}
	legacy, _ := j.GenerateToken(first.User.ID, first.User.Email)
	if _, err = j.ValidateToken(legacy); err == nil {
		t.Fatal("legacy token survived cutover")
	}
	db.Close()
	if _, err = j.ValidateToken(second.Token); !errors.Is(err, auth.ErrSessionUnavailable) {
		t.Fatal("unavailable authority allowed", err)
	}
}

func TestIdentityRegisterAuditFailureRollsBackUserAndSession(t *testing.T) {
	s, _, _, db, d := identityFixture(t)
	if _, err := db.Exec(identityAuditFailureDDL(d, "auth.register")); err != nil {
		t.Fatal(err)
	}
	if result, err := s.Register("rollback@example.invalid", identityPassword); err == nil || result != nil {
		t.Fatal("issued session despite audit failure")
	}
	tables := []string{"users", "session_tokens", "audit_log"}
	if d.DBType() == repository.DBTypePostgres {
		tables = append(tables, "outbox_jobs")
	}
	for _, table := range tables {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial registration persisted", table, err)
		}
	}
}

func TestIdentitySocialAuditRollbackAndSessionBoundLinks(t *testing.T) {
	authSvc, sessions, j, db, d := identityFixture(t)
	cfg := config.SocialAuth{Origin: "http://127.0.0.1:4651", GitHub: config.SocialProvider{ClientID: "github-client", ClientSecret: "test-secret"}, Google: config.SocialProvider{ClientID: "google-client", ClientSecret: "test-secret"}}
	social := NewSocialAuthService(cfg, repository.NewSocialAuthRepository(db, d), authSvc.auditRepo, j)
	social.EnableSessions(sessions)
	githubMock(t, social, true)
	account, err := authSvc.Register("existing@example.invalid", identityPassword)
	if err != nil {
		t.Fatal(err)
	}
	second, err := authSvc.Login("existing@example.invalid", identityPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := social.jwt.ValidateToken(account.Token)
	otherClaims, _ := social.jwt.ValidateToken(second.Token)
	sid := uuid.MustParse(claims.SessionID)
	other := uuid.MustParse(otherClaims.SessionID)
	ctx := context.Background()
	if _, err = social.Start(ctx, "github", testChallenge(), "", &account.User.ID); !errors.Is(err, repository.ErrRecentAuthentication) {
		t.Fatal("unbound link accepted", err)
	}
	start, err := social.StartWithSession(ctx, "github", testChallenge(), "", &account.User.ID, &sid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = social.CompleteWithSession(ctx, "github", "code", start.State, testVerifier, "", &account.User.ID, &other, ""); !errors.Is(err, repository.ErrSocialFlow) {
		t.Fatal("link moved to another session", err)
	}
	if _, err = social.CompleteWithSession(ctx, "github", "code", start.State, testVerifier, "", &account.User.ID, &sid, ""); err != nil {
		t.Fatal(err)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='auth.social_linked'`).Scan(&count)
	if count != 1 {
		t.Fatal("missing link audit")
	}
	if _, err = db.Exec(repository.Q(d, `UPDATE session_tokens SET created_at=$1 WHERE id=$2`), time.Now().Add(-11*time.Minute), other); err != nil {
		t.Fatal(err)
	}
	if _, err = social.StartWithSession(ctx, "google", testChallenge(), "", &account.User.ID, &other); !errors.Is(err, repository.ErrRecentAuthentication) {
		t.Fatal("old authentication accepted", err)
	}
	if _, err = db.Exec(identityAuditFailureDDL(d, "auth.login")); err != nil {
		t.Fatal(err)
	}
	// Existing linked identity cannot receive a new successful session without its outcome audit.
	begin, err := social.Start(ctx, "github", testChallenge(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := social.Complete(ctx, "github", "code", begin.State, testVerifier, "", nil); err == nil || result != nil {
		t.Fatal("social session escaped audit rollback")
	}
	db.QueryRow(`SELECT COUNT(*) FROM session_tokens`).Scan(&count)
	if count != 2 {
		t.Fatal("partial social session persisted")
	}
}

func TestIdentityOperatorGrantDoesNotTrustEmail(t *testing.T) {
	s, _, _, db, d := identityFixture(t)
	account, err := s.Register("configured-operator@example.invalid", identityPassword)
	if err != nil {
		t.Fatal(err)
	}
	r := repository.NewPlatformAdminRepository(db, d, s.auditRepo)
	ctx := context.Background()
	if allowed, err := r.IsPlatformAdmin(ctx, account.User.ID); err != nil || allowed {
		t.Fatal("signup received global administrator authority", err)
	}
	if err = r.SetGrant(ctx, account.User.ID, true, "synthetic-operator", "reviewed test account"); err != nil {
		t.Fatal(err)
	}
	if allowed, err := r.IsPlatformAdmin(ctx, account.User.ID); err != nil || !allowed {
		t.Fatal("stored grant not honored", err)
	}
	if err = r.SetGrant(ctx, account.User.ID, false, "synthetic-operator", "test revocation"); err != nil {
		t.Fatal(err)
	}
	if allowed, err := r.IsPlatformAdmin(ctx, account.User.ID); err != nil || allowed {
		t.Fatal("revoked grant honored", err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action IN ('platform.admin_granted','platform.admin_revoked')`).Scan(&count); err != nil || count != 2 {
		t.Fatal("missing operator audit", err)
	}
}

func identityAuditFailureDDL(d repository.Dialect, action string) string {
	if d.DBType() == repository.DBTypePostgres {
		return "CREATE FUNCTION identity_audit_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='" + action + "' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_identity_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION identity_audit_failure();"
	}
	return "CREATE TRIGGER fail_identity_audit BEFORE INSERT ON audit_log WHEN NEW.action='" + action + "' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END"
}

func TestIdentityCanonicalMigrationCollisionFailsClosed(t *testing.T) {
	// Use the actual shipped predecessor migrations, not a hand-built users table.
	predecessors := fstest.MapFS{}
	err := fs.WalkDir(migrations.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".sql") || entry.Name() >= "020" {
			return nil
		}
		data, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		predecessors[path] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db, d := identityDatabase(t, predecessors)
	for _, email := range []string{"Collision@example.invalid", " collision@example.invalid "} {
		if _, err = db.Exec(repository.Q(d, `INSERT INTO users(id,email,password_hash) VALUES($1,$2,'!')`), uuid.New(), email); err != nil {
			t.Fatal(err)
		}
	}
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err == nil {
		t.Fatal("canonical collision migration succeeded")
	}
	var users, applied int
	if err = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil || users != 2 {
		t.Fatal("migration discarded identities", err)
	}
	if err = db.QueryRow(repository.Q(d, `SELECT COUNT(*) FROM schema_migrations WHERE version='020_identity_sessions'`)).Scan(&applied); err != nil || applied != 0 {
		t.Fatal("failed migration was recorded", err)
	}
}

func TestIdentityFailedLogoutPreservesAuthorityAndAudit(t *testing.T) {
	s, sessions, j, db, d := identityFixture(t)
	response, err := s.Register("logout@example.invalid", identityPassword)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ValidateToken(response.Token)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(claims.SessionID)
	if _, err = db.Exec(identityAuditFailureDDL(d, "auth.session_revoked")); err != nil {
		t.Fatal(err)
	}
	if err = sessions.Revoke(context.Background(), response.User.ID, id, id, ""); err == nil {
		t.Fatal("failed audit reported logout success")
	}
	if _, err = j.ValidateToken(response.Token); err != nil {
		t.Fatal("uncommitted revocation changed session", err)
	}
	var revoked int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='auth.session_revoked'`).Scan(&revoked); err != nil || revoked != 0 {
		t.Fatal("false audit outcome", err)
	}
	if d.DBType() == repository.DBTypePostgres {
		if err = db.QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind='identity.event' AND payload->>'action'='auth.session_revoked'`).Scan(&revoked); err != nil || revoked != 0 {
			t.Fatal("partial identity outbox", err)
		}
	}
}
