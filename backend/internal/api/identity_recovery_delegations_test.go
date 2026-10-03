package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/authority"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/identity"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
)

// Use the shipped router and versioned vault, not a fake authenticator. The
// recovery, key, lease, token and secret requests share the same stored owner.
func recoveryGrantRouter(t *testing.T, f *identityFixture) *vault.Service {
	t.Helper()
	cs, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	permission := func(required bool) vault.AuthorizeTx {
		return func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
			return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: required}}).Authorize(ctx, p, a, r)
		}
	}
	v := vault.New(f.db, cs, permission(false), f.audit)
	for _, pair := range []struct{ owner, project uuid.UUID }{{f.owner, f.project}, {f.other, f.foreign}} {
		if err = v.Enroll(context.Background(), policy.Principal{Kind: policy.Human, SubjectID: pair.owner, ActorID: pair.owner}, pair.project); err != nil {
			t.Fatal(err)
		}
	}
	v = vault.New(f.db, cs, permission(true), f.audit)
	pr := repository.NewProjectRepository(f.db, f.d)
	ss := service.NewSecretService(repository.NewSecretRepository(f.db, f.d), pr, repository.NewEnvironmentRepository(f.db, f.d), f.audit, cs)
	ss.EnableVault(v)
	ls := service.NewLeaseService(f.db, f.d, f.audit)
	ls.EnableSessions(f.sessions)
	deny := repository.NewTokenDenylistRepository(f.db, f.d)
	f.r = NewRouter(Dependencies{CoreRelease: true, DisableLocalMCP: true, CORSOrigins: "http://localhost", JWTService: f.jwt, APIKeyRepo: f.keys, ProjectRepo: pr,
		AuthHandler: &AuthHandler{}, SessionHandler: NewSessionHandler(f.sessions), SecretHandler: NewSecretHandler(ss),
		AgentHandler: NewAgentHandler(ls, service.NewAgentAnalyticsService(f.db, f.d), service.NewAgentTokenService(f.jwt, ls, deny, f.audit)), IdentityPlatformHandler: NewIdentityPlatformHandler(f.identity), DB: f.db})
	return v
}

type recoveryCredentials struct {
	key            string
	keyID, leaseID uuid.UUID
	token, jti     string
	expiry         time.Time
}

func recoveryCredentialsFor(t *testing.T, f *identityFixture, user, project uuid.UUID, browser string) recoveryCredentials {
	t.Helper()
	raw, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(time.Hour)
	k, err := f.keys.Create("synthetic recovery key", hash, user, project, []string{"read"}, nil, &expiry)
	if err != nil {
		t.Fatal(err)
	}
	if k.ID == user {
		t.Fatal("fixture conflates credential and represented user")
	}
	prefix := "/api/v1/projects/" + project.String()
	w := f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, raw, "")
	assertCoreResponse(t, "/projects/{id}/leases", "POST", w, 201)
	var created struct {
		Lease struct{ ID uuid.UUID } `json:"lease"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	w = f.request("POST", prefix+"/agent-token", fmt.Sprintf(`{"lease_id":"%s","duration_minutes":1}`, created.Lease.ID), "", browser)
	assertCoreResponse(t, "/projects/{id}/agent-token", "POST", w, 201)
	var minted struct{ Token string }
	if err = json.Unmarshal(w.Body.Bytes(), &minted); err != nil {
		t.Fatal(err)
	}
	c, err := f.jwt.ValidateToken(minted.Token)
	if err != nil || c.UserID != user || c.UserID == k.ID {
		t.Fatal("mint did not retain represented human identity", err)
	}
	return recoveryCredentials{raw, k.ID, created.Lease.ID, minted.Token, c.ID, c.ExpiresAt.Time}
}

func recoveryProofFor(t *testing.T, f *identityFixture) (uuid.UUID, string) {
	t.Helper()
	if _, err := f.db.Exec(`INSERT INTO identity_verified_contacts(user_id,contact,verified_at) VALUES($1,'recovery-grants@example.invalid',NOW())`, f.owner); err != nil {
		t.Fatal(err)
	}
	w := f.request("POST", "/api/v1/auth/recovery/request", `{"contact":"recovery-grants@example.invalid"}`, "", "")
	if w.Code != 202 {
		t.Fatal("recovery request", w.Code)
	}
	id := identityRequestID(t, w.Body.String())
	return id, f.deliver(t, id, &syntheticProofMail{outcome: identity.DeliveryOutcome{State: "sent", Reason: "smtp_accepted"}})
}

func TestPlatformIdentityRecoveryRevokesRetainedDelegationsAndRollsBack(t *testing.T) {
	f := newIdentityFixture(t)
	recoveryGrantRouter(t, f)
	oldHash, err := auth.HashPassword("Synthetic-Old-Password1!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE users SET password_hash=$1 WHERE id=$2`, oldHash, f.owner); err != nil {
		t.Fatal(err)
	}
	own := recoveryCredentialsFor(t, f, f.owner, f.project, f.token)
	otherBrowser := f.session(t, f.other, "password")
	other := recoveryCredentialsFor(t, f, f.other, f.foreign, otherBrowser)
	path := "/api/v1/projects/" + f.project.String() + "/secrets?environment=alpha"
	for _, credential := range []struct{ key, token string }{{own.key, ""}, {"", own.token}} {
		if w := f.request("GET", path, "", credential.key, credential.token); w.Code != 200 {
			t.Fatal("previously usable delegated read", w.Code)
		}
	}
	id, raw := recoveryProofFor(t, f)
	body := fmt.Sprintf(`{"id":"%s","proof":"%s","password":"Synthetic-New-Password1!"}`, id, raw)
	if _, err = f.db.Exec(grantHTTPAuditFailureDDL(f.d, "identity.proof_consumed")); err != nil {
		t.Fatal(err)
	}
	before := f.countSnapshot()
	if w := f.request("POST", "/api/v1/auth/recovery/confirm", body, "", ""); w.Code != 500 {
		t.Fatal("required audit failure ignored", w.Code)
	}
	if after := f.countSnapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("recovery escaped required audit/outbox rollback")
	}
	var storedHash string
	if err = f.db.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, f.owner).Scan(&storedHash); err != nil || storedHash != oldHash {
		t.Fatal("failed recovery changed password", err)
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proofs WHERE id=$1 AND NOT consumed AND NOT revoked`, id) != 1 ||
		identityCount(t, f.db, `SELECT COUNT(*) FROM api_keys WHERE id=$1 AND expires_at>NOW()`, own.keyID) != 1 ||
		identityCount(t, f.db, `SELECT COUNT(*) FROM secret_leases WHERE id=$1 AND NOT revoked`, own.leaseID) != 1 {
		t.Fatal("failed recovery changed proof or parent authority")
	}
	for _, credential := range []struct{ key, token string }{{own.key, ""}, {"", own.token}} {
		if w := f.request("GET", path, "", credential.key, credential.token); w.Code != 200 {
			t.Fatal("rollback invalidated credential", w.Code)
		}
	}
	if _, err = f.db.Exec(`DROP TRIGGER fail_grant_http_audit ON audit_log; DROP FUNCTION grant_http_audit_failure()`); err != nil {
		t.Fatal(err)
	}
	if w := f.request("POST", "/api/v1/auth/recovery/confirm", body, "", ""); w.Code != 204 {
		t.Fatal("successful recovery", w.Code)
	}
	for _, credential := range []struct{ key, token string }{{own.key, ""}, {"", own.token}, {"", f.token}} {
		if w := f.request("GET", path, "", credential.key, credential.token); w.Code != 401 {
			t.Fatal("recovered authority remained usable", w.Code)
		}
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM api_keys WHERE id=$1 AND user_id=$2 AND expires_at<=NOW()`, own.keyID, f.owner) != 1 ||
		identityCount(t, f.db, `SELECT COUNT(*) FROM secret_leases WHERE id=$1 AND api_key_id=$2 AND revoked AND revoked_at IS NOT NULL`, own.leaseID, own.keyID) != 1 ||
		identityCount(t, f.db, `SELECT COUNT(*) FROM agent_token_issuance WHERE jti=$1 AND user_id=$2 AND lease_id=$3`, own.jti, f.owner, own.leaseID) != 1 ||
		identityCount(t, f.db, `SELECT COUNT(*) FROM token_denylist WHERE jti=$1 AND lease_id=$2 AND expires_at>=$3 AND reason='password_recovery'`, own.jti, own.leaseID, own.expiry) != 1 {
		t.Fatal("recovery deleted or misattributed delegation identities")
	}
	if err = f.db.QueryRow(`SELECT password_hash FROM users WHERE id=$1`, f.owner).Scan(&storedHash); err != nil || auth.CheckPassword("Synthetic-New-Password1!", storedHash) != nil {
		t.Fatal("new password did not commit", err)
	}
	otherPath := "/api/v1/projects/" + f.foreign.String() + "/secrets?environment=alpha"
	for _, credential := range []struct{ key, token string }{{other.key, ""}, {"", other.token}, {"", otherBrowser}} {
		if w := f.request("GET", otherPath, "", credential.key, credential.token); w.Code != 200 {
			t.Fatal("recovery revoked unrelated user", w.Code)
		}
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM token_denylist WHERE jti=$1`, other.jti) != 0 {
		t.Fatal("foreign delegation denylisted")
	}
	if w := f.request("POST", "/api/v1/auth/recovery/confirm", body, "", ""); w.Code != 400 {
		t.Fatal("recovery replay accepted", w.Code)
	}
}

func TestPlatformIdentityRecoveryRequiresDelegationPort(t *testing.T) {
	f := newIdentityFixture(t)
	id, raw := recoveryProofFor(t, f)
	f.identity.EnableRecoveryRevocation(nil)
	w := f.request("POST", "/api/v1/auth/recovery/confirm", fmt.Sprintf(`{"id":"%s","proof":"%s","password":"Synthetic-New-Password1!"}`, id, raw), "", "")
	if w.Code != 503 || identityCount(t, f.db, `SELECT COUNT(*) FROM identity_proofs WHERE id=$1 AND NOT consumed`, id) != 1 {
		t.Fatal("unwired recovery failed open or consumed proof", w.Code)
	}
}

func TestPlatformIdentityRecoverySerializesLegacyMintAndUse(t *testing.T) {
	f := newIdentityFixture(t)
	recoveryGrantRouter(t, f)
	own := recoveryCredentialsFor(t, f, f.owner, f.project, f.token)
	id, raw := recoveryProofFor(t, f)
	guard := &identityPausingGuard{Guard: authority.Guard{DB: f.db, Dialect: f.d}, entered: make(chan struct{}), release: make(chan struct{})}
	paused := identity.New(f.db, identity.Config{Enabled: true, PostgreSQL: true}, f.audit, nil, guard)
	paused.EnableRecoveryRevocation(f.sessions)
	var release sync.Once
	unblock := func() { release.Do(func() { close(guard.release) }) }
	defer unblock()
	reset := make(chan error, 1)
	go func() { reset <- paused.ResetPassword(context.Background(), id, raw, "Synthetic-New-Password1!") }()
	select {
	case <-guard.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not acquire subject barrier")
	}
	prefix := "/api/v1/projects/" + f.project.String()
	lease, token, read := make(chan int, 1), make(chan int, 1), make(chan int, 1)
	go func() {
		lease <- f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":1}`, own.key, "").Code
	}()
	go func() {
		token <- f.request("POST", prefix+"/agent-token", fmt.Sprintf(`{"lease_id":"%s","duration_minutes":1}`, own.leaseID), own.key, "").Code
	}()
	go func() { read <- f.request("GET", prefix+"/secrets?environment=alpha", "", own.key, "").Code }()
	for _, ch := range []chan int{lease, token, read} {
		select {
		case code := <-ch:
			t.Fatalf("legacy operation bypassed recovery barrier: %d", code)
		case <-time.After(100 * time.Millisecond):
		}
	}
	unblock()
	select {
	case err := <-reset:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("recovery did not commit")
	}
	for _, ch := range []chan int{lease, token, read} {
		select {
		case code := <-ch:
			if code != 401 && code != 403 && code != 404 {
				t.Fatal("operation did not deny after recovery commit", code)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("blocked legacy operation did not settle")
		}
	}
	if identityCount(t, f.db, `SELECT COUNT(*) FROM secret_leases WHERE api_key_id=$1`, own.keyID) != 1 || identityCount(t, f.db, `SELECT COUNT(*) FROM agent_token_issuance WHERE user_id=$1`, f.owner) != 1 {
		t.Fatal("recovery race minted new authority")
	}
}
