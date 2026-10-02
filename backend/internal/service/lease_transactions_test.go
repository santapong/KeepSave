package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// This fixture uses the shipped migrations, distinct user/key identities and
// production-style human session checks on SQLite and the disposable PG target.
type grantFixture struct {
	t        *testing.T
	db       *sql.DB
	d        repository.Dialect
	leases   *LeaseService
	tokens   *AgentTokenService
	sessions *SessionService
	jwt      *auth.JWTService
	human    policy.Principal
	key      policy.Principal
	project  uuid.UUID
	org      uuid.UUID
}

func newGrantFixture(t *testing.T) *grantFixture {
	t.Helper()
	login, sessions, jwt, db, d := identityFixture(t)
	registered, err := login.Register("grant-"+uuid.NewString()+"@example.invalid", identityPassword)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := jwt.ValidateToken(registered.Token)
	if err != nil {
		t.Fatal(err)
	}
	project, err := repository.NewProjectRepository(db, d).Create("Grant fixture", "", registered.User.ID, []byte("synthetic-encrypted-key"), []byte("synthetic-nonce"))
	if err != nil {
		t.Fatal(err)
	}
	_, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	key, err := repository.NewAPIKeyRepository(db, d).Create("Parent", hash, registered.User.ID, project.ID, []string{"read"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ls := NewLeaseService(db, d, login.auditRepo)
	ls.EnableSessions(sessions)
	deny := repository.NewTokenDenylistRepository(db, d)
	jwt.EnableDenylist(deny)
	return &grantFixture{t: t, db: db, d: d, leases: ls, tokens: NewAgentTokenService(jwt, ls, deny, login.auditRepo), sessions: sessions, jwt: jwt,
		human: policy.Principal{Kind: policy.Human, SubjectID: registered.User.ID, ActorID: registered.User.ID, SessionID: uuid.MustParse(claims.SessionID)},
		key:   policy.Principal{Kind: policy.APIKey, SubjectID: key.ID, ActorID: registered.User.ID}, project: project.ID}
}

func (f *grantFixture) lease() *models.SecretLease {
	f.t.Helper()
	lease, err := f.leases.CreateLeaseAuthorized(context.Background(), f.key, f.project, "alpha", []string{"DB_URL"}, 10*time.Minute, "")
	if err != nil {
		f.t.Fatal(err)
	}
	return lease
}

func (f *grantFixture) mint(lease uuid.UUID) (*MintedAgentToken, string) {
	f.t.Helper()
	token, err := f.tokens.MintTokenAuthorized(context.Background(), f.human, f.project, lease, time.Minute, "")
	if err != nil {
		f.t.Fatal(err)
	}
	claims, err := f.jwt.ValidateToken(token.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	return token, claims.ID
}

func (f *grantFixture) organization() {
	f.t.Helper()
	f.org = uuid.New()
	for _, operation := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations(id,name,slug,owner_id) VALUES($1,'Grant org',$2,$3)`, []any{f.org, uuid.NewString(), f.human.SubjectID}},
		{`INSERT INTO organization_members(id,organization_id,user_id,role) VALUES($1,$2,$3,'editor')`, []any{uuid.New(), f.org, f.human.SubjectID}},
		{`UPDATE projects SET organization_id=$1 WHERE id=$2`, []any{f.org, f.project}},
	} {
		if _, err := f.db.Exec(repository.Q(f.d, operation.query), operation.args...); err != nil {
			f.t.Fatal(err)
		}
	}
}

// Include revocation state and the persisted audit head, not only row counts:
// all of these values must remain unchanged when a required write fails.
func (f *grantFixture) snapshot() map[string]string {
	f.t.Helper()
	queries := map[string]string{
		"leases": `SELECT COUNT(*) FROM secret_leases`, "revoked_leases": `SELECT COUNT(*) FROM secret_leases WHERE revoked=` + f.d.BoolLiteral(true),
		"issuance": `SELECT COUNT(*) FROM agent_token_issuance`, "denylist": `SELECT COUNT(*) FROM token_denylist`,
		"audit": `SELECT COUNT(*) FROM audit_log`, "head": `SELECT entry_hash FROM audit_chain_head WHERE id=1`,
		"revision": `SELECT revision FROM audit_chain_head WHERE id=1`,
	}
	if f.d.DBType() == repository.DBTypePostgres {
		queries["outbox"] = `SELECT COUNT(*) FROM outbox_jobs`
	}
	result := map[string]string{}
	for key, query := range queries {
		var value string
		if err := f.db.QueryRow(query).Scan(&value); err != nil {
			f.t.Fatal(err)
		}
		result[key] = value
	}
	return result
}

func TestIdentityGrantRequiredWritesAreAtomic(t *testing.T) {
	for _, sink := range []string{"audit", "outbox"} {
		for _, action := range []string{"lease.created", "lease.revoked", "agent.token.minted", "agent.token.revoked"} {
			t.Run(sink+"/"+action, func(t *testing.T) {
				f := newGrantFixture(t)
				if sink == "outbox" && f.d.DBType() != repository.DBTypePostgres {
					t.Skip("PostgreSQL platform outbox contract")
				}
				var lease *models.SecretLease
				var minted *MintedAgentToken
				var jti string
				if action != "lease.created" {
					lease = f.lease()
				}
				if action == "agent.token.revoked" {
					minted, jti = f.mint(lease.ID)
				}
				ddl := identityAuditFailureDDL(f.d, action)
				if sink == "outbox" {
					ddl = fmt.Sprintf(`CREATE FUNCTION grant_outbox_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='identity.event' AND NEW.payload->>'action'='%s' THEN RAISE EXCEPTION 'synthetic outbox failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_grant_outbox BEFORE INSERT ON outbox_jobs FOR EACH ROW EXECUTE FUNCTION grant_outbox_failure();`, action)
				}
				if _, err := f.db.Exec(ddl); err != nil {
					t.Fatal(err)
				}
				before := f.snapshot()
				var err error
				switch action {
				case "lease.created":
					var result *models.SecretLease
					result, err = f.leases.CreateLeaseAuthorized(context.Background(), f.key, f.project, "alpha", []string{"DB_URL"}, time.Minute, "")
					if result != nil {
						t.Fatal("failed transaction returned a grant")
					}
				case "lease.revoked":
					err = f.leases.RevokeLeaseAuthorized(context.Background(), f.human, lease.ID, f.project, "")
				case "agent.token.minted":
					var result *MintedAgentToken
					result, err = f.tokens.MintTokenAuthorized(context.Background(), f.human, f.project, lease.ID, time.Minute, "")
					if result != nil {
						t.Fatal("uncommitted token escaped")
					}
				case "agent.token.revoked":
					err = f.tokens.RevokeTokenAuthorized(context.Background(), f.human, f.project, jti, "")
				}
				if err == nil {
					t.Fatal("required write failed but mutation reported success")
				}
				if after := f.snapshot(); !reflect.DeepEqual(before, after) {
					t.Fatalf("partial mutation escaped rollback: before=%v after=%v", before, after)
				}
				if minted != nil {
					if _, err = f.jwt.ValidateToken(minted.Token); err != nil {
						t.Fatal("failed revocation changed effective token state", err)
					}
				}
			})
		}
	}
}

func TestIdentityGrantCurrentAuthorityIsRequired(t *testing.T) {
	for _, change := range []string{"scope", "environment", "expiry", "parent_deleted", "role", "membership", "project"} {
		t.Run(change, func(t *testing.T) {
			f := newGrantFixture(t)
			f.organization()
			lease := f.lease()
			var query string
			var args []any
			switch change {
			case "scope":
				scopes, err := f.d.ArrayParam([]string{"read:OTHER"})
				if err != nil {
					t.Fatal(err)
				}
				query, args = `UPDATE api_keys SET scopes=$1 WHERE id=$2`, []any{scopes, f.key.SubjectID}
			case "environment":
				query, args = `UPDATE api_keys SET environment='prod' WHERE id=$1`, []any{f.key.SubjectID}
			case "expiry":
				query, args = `UPDATE api_keys SET expires_at=$1 WHERE id=$2`, []any{time.Now().Add(-time.Hour), f.key.SubjectID}
			case "parent_deleted":
				query, args = `DELETE FROM api_keys WHERE id=$1`, []any{f.key.SubjectID}
			case "role":
				query, args = `UPDATE organization_members SET role='viewer' WHERE organization_id=$1`, []any{f.org}
			case "membership":
				query, args = `DELETE FROM organization_members WHERE organization_id=$1`, []any{f.org}
			case "project":
				query, args = `UPDATE projects SET deleted_at=$1 WHERE id=$2`, []any{time.Now(), f.project}
			}
			if _, err := f.db.Exec(repository.Q(f.d, query), args...); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot()
			if granted, err := f.leases.CreateLeaseAuthorized(context.Background(), f.key, f.project, "alpha", []string{"DB_URL"}, time.Minute, ""); err == nil || granted != nil {
				t.Fatal("current parent/role was not checked at lease admission")
			}
			if token, err := f.tokens.MintTokenAuthorized(context.Background(), f.human, f.project, lease.ID, time.Minute, ""); err == nil || token != nil {
				t.Fatal("human mint bypassed current parent/role")
			}
			if token, err := f.tokens.MintTokenAuthorized(context.Background(), f.key, f.project, lease.ID, time.Minute, ""); err == nil || token != nil {
				t.Fatal("API key mint bypassed current parent/role")
			}
			if after := f.snapshot(); !reflect.DeepEqual(before, after) {
				t.Fatal("denied admission mutated grants or success records")
			}
		})
	}
}

func TestIdentityGrantStrictSessionsLineageAndRevocation(t *testing.T) {
	f := newGrantFixture(t)
	ctx := context.Background()
	lease := f.lease()
	_, jti := f.mint(lease.ID)
	_, hash, _ := auth.GenerateAPIKey()
	sibling, err := repository.NewAPIKeyRepository(f.db, f.d).Create("Sibling", hash, f.human.SubjectID, f.project, []string{"read"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := policy.Principal{Kind: policy.APIKey, SubjectID: sibling.ID, ActorID: f.human.SubjectID}
	before := f.snapshot()
	if err = f.leases.RevokeLeaseAuthorized(ctx, p, lease.ID, f.project, ""); !errors.Is(err, ErrLeaseNotFound) {
		t.Fatal("sibling key revoked lease", err)
	}
	if err = f.tokens.RevokeTokenAuthorized(ctx, p, f.project, jti, ""); !errors.Is(err, ErrAgentTokenNotFound) {
		t.Fatal("sibling key revoked token", err)
	}
	for _, session := range []uuid.UUID{uuid.Nil, uuid.New()} {
		p = f.human
		p.SessionID = session
		if token, err := f.tokens.MintTokenAuthorized(ctx, p, f.project, lease.ID, time.Minute, ""); !errors.Is(err, auth.ErrSessionInvalid) || token != nil {
			t.Fatal("missing/foreign session minted token", err)
		}
		if err = f.leases.RevokeLeaseAuthorized(ctx, p, lease.ID, f.project, ""); !errors.Is(err, auth.ErrSessionInvalid) {
			t.Fatal("missing/foreign session revoked lease", err)
		}
		if err = f.tokens.RevokeTokenAuthorized(ctx, p, f.project, jti, ""); !errors.Is(err, auth.ErrSessionInvalid) {
			t.Fatal("missing/foreign session revoked token", err)
		}
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("lineage/session denial mutated records")
	}
	if err = f.sessions.Revoke(ctx, f.human.SubjectID, f.human.SessionID, f.human.SessionID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.tokens.MintTokenAuthorized(ctx, f.human, f.project, lease.ID, time.Minute, ""); !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatal("revoked session reused for token admission", err)
	}
	// The key is a separate identity; logging out its owner does not revoke it.
	if _, err = f.tokens.MintTokenAuthorized(ctx, f.key, f.project, lease.ID, time.Minute, ""); err != nil {
		t.Fatal("API key incorrectly inherited human session status", err)
	}
}

func TestIdentityGrantEmptySelectionAndExpiredRevocation(t *testing.T) {
	f := newGrantFixture(t)
	ctx := context.Background()
	lease, err := f.leases.CreateLeaseAuthorized(ctx, f.key, f.project, "alpha", nil, time.Minute, "")
	if err != nil || !reflect.DeepEqual([]string(lease.SecretKeys), []string{"*"}) {
		t.Fatal("unbounded-read empty selection lost legacy semantics", err)
	}
	_, jti := f.mint(lease.ID)
	expires := time.Now().Add(-time.Hour)
	if _, err = f.db.Exec(repository.Q(f.d, `UPDATE api_keys SET expires_at=$1 WHERE id=$2`), expires, f.key.SubjectID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(repository.Q(f.d, `UPDATE secret_leases SET expires_at=$1 WHERE id=$2`), expires, lease.ID); err != nil {
		t.Fatal(err)
	}
	if err = f.tokens.RevokeTokenAuthorized(ctx, f.human, f.project, jti, ""); err != nil {
		t.Fatal("human could not revoke expired lineage", err)
	}
	if err = f.leases.RevokeLeaseAuthorized(ctx, f.human, lease.ID, f.project, ""); err != nil {
		t.Fatal("human could not revoke expired lease", err)
	}
	// Repeating a committed revocation creates no duplicate audit/outbox entry.
	before := f.snapshot()
	if err = f.tokens.RevokeTokenAuthorized(ctx, f.human, f.project, jti, ""); err != nil {
		t.Fatal(err)
	}
	if err = f.leases.RevokeLeaseAuthorized(ctx, f.human, lease.ID, f.project, ""); err != nil {
		t.Fatal(err)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("idempotent revocation duplicated success records")
	}
}
