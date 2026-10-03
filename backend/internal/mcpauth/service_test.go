package mcpauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/migrations"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func config() Config {
	return Config{Issuer: "https://app.keepsave.example", AppURL: "https://app.keepsave.example", Resource: "https://app.keepsave.example/mcp"}
}

type fixture struct {
	s, s2     *Service
	db        *sql.DB
	user, sid uuid.UUID
}

func pgFixture(t *testing.T) *fixture {
	t.Helper()
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") != "1" {
		t.Skip("requires fixed disposable PostgreSQL target")
	}
	dsn := "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
	admin, _, e := repository.NewDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "mcp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, e = admin.Exec("CREATE SCHEMA " + schema); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	dsn += "&search_path=" + schema + ",public"
	db, d, e := repository.NewDB(dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(24)
	if e = repository.RunMigrationsFS(db, d, migrations.FS); e != nil {
		t.Fatal(e)
	}
	u, e := repository.NewUserRepository(db, d).Create(uuid.NewString()+"@example.invalid", "!")
	if e != nil {
		t.Fatal(e)
	}
	sid := uuid.New()
	_, e = db.Exec(`INSERT INTO session_tokens(id,user_id,token_hash,expires_at,revoked) VALUES($1,$2,$3,$4,FALSE)`, sid, u.ID, hash("synthetic-human-session"), time.Now().Add(24*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey([]byte("synthetic-mcp-chain-key"))
	s, e := New(db, d, audit, authority.Postgres(db), config())
	if e != nil {
		t.Fatal(e)
	}
	s2, e := New(db, d, audit, authority.Postgres(db), config())
	if e != nil {
		t.Fatal(e)
	}
	return &fixture{s, s2, db, u.ID, sid}
}
func (f *fixture) code(t *testing.T, client string) (string, TokenRequest) {
	t.Helper()
	verifier := strings.Repeat("a", 64)
	b := sha256.Sum256([]byte(verifier))
	callback := "http://127.0.0.1:17701/callback"
	if client == "keepsave-hermes-linux-v1" {
		callback = "http://127.0.0.1:17702/callback"
	}
	values := url.Values{"client_id": {client}, "redirect_uri": {callback}, "response_type": {"code"}, "resource": {config().Resource}, "scope": {Scope + " offline_access"}, "state": {strings.Repeat("s", 32)}, "code_challenge": {base64.RawURLEncoding.EncodeToString(b[:])}, "code_challenge_method": {"S256"}}
	req, e := f.s.BeginAuthorization(context.Background(), values)
	if e != nil {
		t.Fatal(e)
	}
	preview, e := f.s.Preview(context.Background(), f.user, f.sid, req.ID)
	if e != nil || preview.ClientID != client {
		t.Fatalf("preview denied: %v", e)
	}
	redirect, e := f.s.Decide(context.Background(), f.user, f.sid, req.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(redirect)
	if e != nil {
		t.Fatal(e)
	}
	if u.Query().Get("iss") != config().Issuer || u.Query().Get("state") != values.Get("state") {
		t.Fatal("callback not issuer/state bound")
	}
	code := u.Query().Get("code")
	return code, TokenRequest{GrantType: "authorization_code", ClientID: client, Code: code, RedirectURI: callback, Verifier: verifier, Resource: config().Resource}
}
func TestCodeAtomicAcrossServicesAndOpaqueStorage(t *testing.T) {
	f := pgFixture(t)
	ctx := context.Background()
	code, r := f.code(t, "keepsave-codex-linux-v1")
	var won atomic.Int32
	var winner TokenResponse
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := f.s
			if i%2 == 1 {
				s = f.s2
			}
			v, e := s.Exchange(ctx, r)
			if e == nil {
				won.Add(1)
				mu.Lock()
				winner = v
				mu.Unlock()
			} else if !errors.Is(e, ErrDenied) {
				t.Errorf("unexpected exchange error: %v", e)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("successful redemptions %d", won.Load())
	}
	p, e := f.s2.Validate(ctx, winner.AccessToken)
	if e != nil || p.UserID != f.user || p.SessionID != f.sid || p.ClientID != r.ClientID {
		t.Fatalf("validation: %v", e)
	}
	var codeHash, accessHash, refreshHash string
	f.db.QueryRow(`SELECT code_hash FROM mcp_oauth_codes`).Scan(&codeHash)
	f.db.QueryRow(`SELECT access_hash,refresh_hash FROM mcp_oauth_tokens`).Scan(&accessHash, &refreshHash)
	if codeHash != hash(code) || accessHash != hash(winner.AccessToken) || refreshHash != hash(winner.RefreshToken) || strings.Contains(codeHash, code) {
		t.Fatal("opaque storage mismatch")
	}
	if winner.ExpiresIn < 590 || winner.ExpiresIn > 600 {
		t.Fatal("access deadline mismatch")
	}
}
func TestRefreshReplayRevokesFamilyAcrossServices(t *testing.T) {
	f := pgFixture(t)
	ctx := context.Background()
	_, r := f.code(t, "keepsave-hermes-linux-v1")
	initial, e := f.s.Exchange(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	refresh := TokenRequest{GrantType: "refresh_token", ClientID: r.ClientID, Resource: r.Resource, RefreshToken: initial.RefreshToken}
	var won atomic.Int32
	var winner TokenResponse
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := f.s
			if i%2 == 1 {
				s = f.s2
			}
			v, e := s.Exchange(ctx, refresh)
			if e == nil {
				won.Add(1)
				mu.Lock()
				winner = v
				mu.Unlock()
			} else if !errors.Is(e, ErrDenied) {
				t.Errorf("refresh: %v", e)
			}
		}(i)
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("rotations %d", won.Load())
	}
	if _, e = f.s2.Validate(ctx, winner.AccessToken); !errors.Is(e, ErrDenied) {
		t.Fatalf("replay successor survived: %v", e)
	}
	if _, e = f.s.Validate(ctx, initial.AccessToken); !errors.Is(e, ErrDenied) {
		t.Fatal("old access survived rotation")
	}
	var n int
	if e = f.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action='mcp.refresh_replay'`).Scan(&n); e != nil || n != 1 {
		t.Fatalf("replay audit: %d %v", n, e)
	}
}
func TestOAuthResourceCallbackPKCEExpiryAndParentRevocation(t *testing.T) {
	f := pgFixture(t)
	ctx := context.Background()
	_, r := f.code(t, "keepsave-codex-linux-v1")
	for _, mutate := range []func(*TokenRequest){func(v *TokenRequest) { v.Resource = "https://other.example/mcp" }, func(v *TokenRequest) { v.ClientID = "keepsave-hermes-linux-v1" }, func(v *TokenRequest) { v.RedirectURI = "http://127.0.0.1:17702/callback" }, func(v *TokenRequest) { v.Verifier = strings.Repeat("b", 64) }} {
		bad := r
		mutate(&bad)
		if _, e := f.s.Exchange(ctx, bad); e == nil {
			t.Fatal("bad request allowed")
		}
	}
	issued, e := f.s.Exchange(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Revoke(ctx, issued.RefreshToken, r.ClientID); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s2.Validate(ctx, issued.AccessToken); e == nil {
		t.Fatal("revoked family survived")
	}
	_, r = f.code(t, "keepsave-codex-linux-v1")
	if _, e = f.db.Exec(`UPDATE mcp_oauth_codes SET expires_at=NOW()-INTERVAL '1 second' WHERE code_hash=$1`, hash(r.Code)); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Exchange(ctx, r); e == nil {
		t.Fatal("expired code allowed")
	}
	_, r = f.code(t, "keepsave-codex-linux-v1")
	issued, e = f.s.Exchange(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE session_tokens SET revoked=TRUE WHERE id=$1`, f.sid); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s2.Validate(ctx, issued.AccessToken); e == nil {
		t.Fatal("revoked parent survived")
	}
	if _, e = f.s.Exchange(ctx, TokenRequest{GrantType: "refresh_token", ClientID: r.ClientID, Resource: r.Resource, RefreshToken: issued.RefreshToken}); e == nil {
		t.Fatal("revoked parent refreshed")
	}
}
func TestAuditFailureRollsBackCodeConsumption(t *testing.T) {
	f := pgFixture(t)
	_, r := f.code(t, "keepsave-codex-linux-v1")
	if _, e := f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT synthetic_mcp_audit_failure CHECK (action != 'mcp.token_issued') NOT VALID`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.Exchange(context.Background(), r); !errors.Is(e, ErrUnavailable) {
		t.Fatalf("audit failure: %v", e)
	}
	var consumed sql.NullTime
	if e := f.db.QueryRow(`SELECT consumed_at FROM mcp_oauth_codes WHERE code_hash=$1`, hash(r.Code)).Scan(&consumed); e != nil || consumed.Valid {
		t.Fatal("failed audit consumed code")
	}
	var n int
	f.db.QueryRow(`SELECT COUNT(*) FROM mcp_oauth_tokens`).Scan(&n)
	if n != 0 {
		t.Fatal("failed audit minted token")
	}
	if _, e := f.db.Exec(`ALTER TABLE audit_log DROP CONSTRAINT synthetic_mcp_audit_failure`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s2.Exchange(context.Background(), r); e != nil {
		t.Fatal(e)
	}
}
func TestCanonicalConfigAndOrigin(t *testing.T) {
	cfg := config()
	for _, bad := range []string{"http://app.keepsave.example/mcp", "https://user:secret@app.keepsave.example/mcp", "https://other.example/mcp", "https://app.keepsave.example/mcp?x=y", "https://app.keepsave.example/mcp/"} {
		c := cfg
		c.Resource = bad
		if c.Validate() == nil {
			t.Fatalf("invalid resource allowed: %s", bad)
		}
	}
	for origin, want := range map[string]bool{"": true, cfg.AppURL: true, "https://evil.example": false, "null": false} {
		r := httptest.NewRequest("POST", "https://app.keepsave.example/mcp", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if OriginAllowed(cfg, r) != want {
			t.Fatalf("origin %s", origin)
		}
	}
	r := httptest.NewRequest("POST", cfg.Resource, nil)
	r.Header.Add("Origin", cfg.AppURL)
	r.Header.Add("Origin", cfg.AppURL)
	if OriginAllowed(cfg, r) {
		t.Fatal("ambiguous origin allowed")
	}
}
func TestDelegationReauthorizationRejectsRotatedToken(t *testing.T) {
	f := pgFixture(t)
	ctx := context.Background()
	_, r := f.code(t, "keepsave-codex-linux-v1")
	issued, e := f.s.Exchange(ctx, r)
	if e != nil {
		t.Fatal(e)
	}
	principal, e := f.s.Validate(ctx, issued.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	p := policy.Principal{Kind: policy.OAuthDelegation, SubjectID: principal.UserID, ActorID: principal.UserID, SessionID: principal.SessionID, ParentGrantID: principal.FamilyID, TokenID: principal.TokenID.String(), ExpiresAt: principal.ExpiresAt}
	check := func(want bool) {
		t.Helper()
		tx, e := f.db.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		if e = authority.Postgres(f.db).RequireActiveTx(ctx, tx, f.user, f.sid); e != nil {
			t.Fatal(e)
		}
		e = f.s.RequireGrantTx(ctx, tx, p, principal.ClientID)
		if (e == nil) != want {
			t.Fatalf("grant reauthorization %v expected %v", e, want)
		}
	}
	check(true)
	_, e = f.s2.Exchange(ctx, TokenRequest{GrantType: "refresh_token", ClientID: r.ClientID, Resource: r.Resource, RefreshToken: issued.RefreshToken})
	if e != nil {
		t.Fatal(e)
	}
	check(false)
}
func TestPublicMetadataAndAmbiguousOAuthDenial(t *testing.T) {
	s := &Service{cfg: config()}
	m := s.AuthorizationMetadata()
	if _, ok := m["registration_endpoint"]; ok {
		t.Fatal("dynamic registration advertised")
	}
	if m["authorization_response_iss_parameter_supported"] != true || m["issuer"] != config().Issuer {
		t.Fatal("issuer binding absent")
	}
	if s.ProtectedMetadata()["resource"] != config().Resource {
		t.Fatal("resource metadata mismatch")
	}
	_, e := s.BeginAuthorization(context.Background(), url.Values{"resource": {config().Resource, "https://evil.example/mcp"}})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal("ambiguous request not rejected before storage")
	}
	for _, scope := range []string{"", "write", Scope + " write", Scope + " " + Scope, "offline_access"} {
		if _, e = canonicalScope(scope); e == nil {
			t.Fatal("invalid scope accepted")
		}
	}
}
