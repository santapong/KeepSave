package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

type platformGrantFixture struct {
	*platformFixture
	sessions *service.SessionService
	token    string
}

func newPlatformGrantFixture(t *testing.T) *platformGrantFixture {
	t.Helper()
	f := newPlatformFixture(t)
	ar := repository.NewAuditRepository(f.db, f.d)
	ar.SetChainKey([]byte("synthetic-grant-http-audit-key"))
	jwt := auth.NewJWTService("synthetic-grant-http-signing-key")
	sessions := service.NewSessionService(f.db, f.d, jwt, ar)
	jwt.EnableHumanSessions(sessions)
	deny := repository.NewTokenDenylistRepository(f.db, f.d)
	jwt.EnableDenylist(deny)
	ls := service.NewLeaseService(f.db, f.d, ar)
	ls.EnableSessions(sessions)
	pr := repository.NewProjectRepository(f.db, f.d)
	f.r = NewRouter(Dependencies{CoreRelease: true, DisableLocalMCP: true, CORSOrigins: "http://localhost", JWTService: jwt, APIKeyRepo: f.keys, ProjectRepo: pr,
		AuthHandler: &AuthHandler{}, SessionHandler: NewSessionHandler(sessions), AgentHandler: NewAgentHandler(ls, service.NewAgentAnalyticsService(f.db, f.d), service.NewAgentTokenService(jwt, ls, deny, ar)), DB: f.db})
	f.jwt = jwt
	u, err := repository.NewUserRepository(f.db, f.d).GetByID(f.owner)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	token, err := sessions.IssueTx(context.Background(), tx, u, "fixture", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return &platformGrantFixture{f, sessions, token}
}

func (f *platformGrantFixture) request(method, path, body, key, token string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec
}

func (f *platformGrantFixture) countSnapshot() map[string]string {
	f.t.Helper()
	queries := map[string]string{
		"leases": `SELECT COUNT(*) FROM secret_leases`, "revoked": `SELECT COUNT(*) FROM secret_leases WHERE revoked=` + f.d.BoolLiteral(true),
		"issuance": `SELECT COUNT(*) FROM agent_token_issuance`, "denylist": `SELECT COUNT(*) FROM token_denylist`,
		"audit": `SELECT COUNT(*) FROM audit_log`, "head": `SELECT entry_hash FROM audit_chain_head WHERE id=1`, "revision": `SELECT revision FROM audit_chain_head WHERE id=1`,
	}
	if f.d.DBType() == repository.DBTypePostgres {
		queries["outbox"] = `SELECT COUNT(*) FROM outbox_jobs`
	}
	result := map[string]string{}
	for name, query := range queries {
		var value string
		if err := f.db.QueryRow(query).Scan(&value); err != nil {
			f.t.Fatal(err)
		}
		result[name] = value
	}
	return result
}

func grantHTTPAuditFailureDDL(d repository.Dialect, action string) string {
	if d.DBType() == repository.DBTypePostgres {
		return "CREATE FUNCTION grant_http_audit_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='" + action + "' THEN RAISE EXCEPTION 'synthetic grant audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_grant_http_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION grant_http_audit_failure();"
	}
	return "CREATE TRIGGER fail_grant_http_audit BEFORE INSERT ON audit_log WHEN NEW.action='" + action + "' BEGIN SELECT RAISE(ABORT,'synthetic grant audit failure'); END"
}

func TestPlatformGrantHandlersRollbackRequiredAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, action := range []string{"lease.created", "lease.revoked", "agent.token.minted", "agent.token.revoked"} {
		t.Run(action, func(t *testing.T) {
			f := newPlatformGrantFixture(t)
			key, _ := f.key([]string{"read"}, nil, nil)
			prefix := "/api/v1/projects/" + f.project.String()
			var leaseID, jti string
			if action != "lease.created" {
				created := f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, "")
				assertCoreResponse(t, "/projects/{id}/leases", "POST", created, 201)
				var response struct {
					Lease struct{ ID string } `json:"lease"`
				}
				if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				leaseID = response.Lease.ID
			}
			if action == "agent.token.revoked" {
				minted := f.request("POST", prefix+"/agent-token", `{"lease_id":"`+leaseID+`","duration_minutes":1}`, "", f.token)
				assertCoreResponse(t, "/projects/{id}/agent-token", "POST", minted, 201)
				var response struct{ Token string }
				if err := json.Unmarshal(minted.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				claims, err := f.jwt.ValidateToken(response.Token)
				if err != nil {
					t.Fatal(err)
				}
				jti = claims.ID
			}
			if _, err := f.db.Exec(grantHTTPAuditFailureDDL(f.d, action)); err != nil {
				t.Fatal(err)
			}
			before := f.countSnapshot()
			var rec *httptest.ResponseRecorder
			var contractPath, method string
			switch action {
			case "lease.created":
				rec = f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, "")
				contractPath, method = "/projects/{id}/leases", "POST"
			case "lease.revoked":
				rec = f.request("DELETE", prefix+"/leases/"+leaseID, "", "", f.token)
				contractPath, method = "/projects/{id}/leases/{leaseId}", "DELETE"
			case "agent.token.minted":
				rec = f.request("POST", prefix+"/agent-token", `{"lease_id":"`+leaseID+`","duration_minutes":1}`, "", f.token)
				contractPath, method = "/projects/{id}/agent-token", "POST"
			case "agent.token.revoked":
				rec = f.request("POST", prefix+"/agent-token/revoke", `{"jti":"`+jti+`"}`, "", f.token)
				contractPath, method = "/projects/{id}/agent-token/revoke", "POST"
			}
			assertCoreResponse(t, contractPath, method, rec, 500)
			if rec.Code != 500 || strings.Contains(rec.Body.String(), `"token":`) || strings.Contains(rec.Body.String(), `"lease":`) || strings.Contains(rec.Body.String(), "synthetic grant audit failure") {
				t.Fatal("failed transaction disclosed success or database details", rec.Code)
			}
			if after := f.countSnapshot(); !reflect.DeepEqual(before, after) {
				t.Fatal("handler escaped grant/audit rollback")
			}
		})
	}
}

func TestPlatformGrantHumanSessionCutover(t *testing.T) {
	f := newPlatformGrantFixture(t)
	key, _ := f.key([]string{"read"}, nil, nil)
	prefix := "/api/v1/projects/" + f.project.String()
	created := f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, "")
	assertCoreResponse(t, "/projects/{id}/leases", "POST", created, 201)
	var response struct {
		Lease struct{ ID string } `json:"lease"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	body := `{"lease_id":"` + response.Lease.ID + `","duration_minutes":1}`
	legacy, err := f.jwt.GenerateToken(f.owner, "fixture@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	assertCoreResponse(t, "/projects/{id}/agent-token", "POST", f.request("POST", prefix+"/agent-token", body, "", legacy), 401)
	assertCoreResponse(t, "/projects/{id}/agent-token", "POST", f.request("POST", prefix+"/agent-token", body, "", f.token), 201)
	claims, err := f.jwt.ValidateToken(f.token)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(claims.SessionID)
	if err = f.sessions.Revoke(context.Background(), f.owner, id, id, ""); err != nil {
		t.Fatal(err)
	}
	before := f.countSnapshot()
	assertCoreResponse(t, "/projects/{id}/agent-token", "POST", f.request("POST", prefix+"/agent-token", body, "", f.token), 401)
	assertCoreResponse(t, "/projects/{id}/leases/{leaseId}", "DELETE", f.request("DELETE", prefix+"/leases/"+response.Lease.ID, "", "", f.token), 401)
	if after := f.countSnapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("revoked browser session changed state")
	}
}

func TestPlatformGrantCurrentParentDeniesHumanMint(t *testing.T) {
	for _, change := range []string{"scope", "environment", "expiry", "role", "membership"} {
		t.Run(change, func(t *testing.T) {
			f := newPlatformGrantFixture(t)
			org := uuid.New()
			for _, operation := range []struct {
				query string
				args  []any
			}{
				{`INSERT INTO organizations(id,name,slug,owner_id) VALUES($1,'Grant org',$2,$3)`, []any{org, uuid.NewString(), f.owner}},
				{`INSERT INTO organization_members(id,organization_id,user_id,role) VALUES($1,$2,$3,'editor')`, []any{uuid.New(), org, f.owner}},
				{`UPDATE projects SET organization_id=$1 WHERE id=$2`, []any{org, f.project}},
			} {
				if _, err := f.db.Exec(repository.Q(f.d, operation.query), operation.args...); err != nil {
					t.Fatal(err)
				}
			}
			key, keyID := f.key([]string{"read"}, nil, nil)
			prefix := "/api/v1/projects/" + f.project.String()
			created := f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, "")
			assertCoreResponse(t, "/projects/{id}/leases", "POST", created, 201)
			var response struct {
				Lease struct{ ID string } `json:"lease"`
			}
			if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			var query string
			var args []any
			switch change {
			case "scope":
				scopes, _ := f.d.ArrayParam([]string{"read:OTHER"})
				query, args = `UPDATE api_keys SET scopes=$1 WHERE id=$2`, []any{scopes, keyID}
			case "environment":
				query, args = `UPDATE api_keys SET environment='prod' WHERE id=$1`, []any{keyID}
			case "expiry":
				query, args = `UPDATE api_keys SET expires_at=$1 WHERE id=$2`, []any{time.Now().Add(-time.Hour), keyID}
			case "role":
				query, args = `UPDATE organization_members SET role='viewer' WHERE organization_id=$1`, []any{org}
			case "membership":
				query, args = `DELETE FROM organization_members WHERE organization_id=$1`, []any{org}
			}
			if _, err := f.db.Exec(repository.Q(f.d, query), args...); err != nil {
				t.Fatal(err)
			}
			before := f.countSnapshot()
			minted := f.request("POST", prefix+"/agent-token", `{"lease_id":"`+response.Lease.ID+`","duration_minutes":1}`, "", f.token)
			assertCoreResponse(t, "/projects/{id}/agent-token", "POST", minted, 403)
			if after := f.countSnapshot(); !reflect.DeepEqual(before, after) {
				t.Fatal("current parent/member denial changed persisted authority")
			}
		})
	}
}

func TestPlatformGrantContractsAndExactLineage(t *testing.T) {
	f := newPlatformGrantFixture(t)
	key, _ := f.key([]string{"read", "write"}, nil, nil)
	sibling, _ := f.key([]string{"read", "write"}, nil, nil)
	prefix := "/api/v1/projects/" + f.project.String()
	created := f.request("POST", prefix+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, "")
	assertCoreResponse(t, "/projects/{id}/leases", "POST", created, 201)
	var response struct {
		Lease struct{ ID string } `json:"lease"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	assertCoreResponse(t, "/projects/{id}/leases", "GET", f.request("GET", prefix+"/leases", "", key, ""), 200)
	minted := f.request("POST", prefix+"/agent-token", `{"lease_id":"`+response.Lease.ID+`","duration_minutes":1}`, key, "")
	assertCoreResponse(t, "/projects/{id}/agent-token", "POST", minted, 201)
	var token struct{ Token string }
	if err := json.Unmarshal(minted.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	claims, err := f.jwt.ValidateToken(token.Token)
	if err != nil {
		t.Fatal(err)
	}
	revokeBody := `{"jti":"` + claims.ID + `"}`
	assertCoreResponse(t, "/projects/{id}/agent-token/revoke", "POST", f.request("POST", prefix+"/agent-token/revoke", revokeBody, sibling, ""), 404)
	assertCoreResponse(t, "/projects/{id}/leases/{leaseId}", "DELETE", f.request("DELETE", prefix+"/leases/"+response.Lease.ID, "", sibling, ""), 404)
	assertCoreResponse(t, "/projects/{id}/agent-token/revoke", "POST", f.request("POST", prefix+"/agent-token/revoke", revokeBody, "", f.token), 204)
	assertCoreResponse(t, "/projects/{id}/leases/{leaseId}", "DELETE", f.request("DELETE", prefix+"/leases/"+response.Lease.ID, "", "", f.token), 204)
	assertCoreResponse(t, "/projects/{id}/leases", "GET", f.request("GET", prefix+"/leases", "", key, ""), 200)
}
