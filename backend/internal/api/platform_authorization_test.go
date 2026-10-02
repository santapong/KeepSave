package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/migrations"
)

type platformFixture struct {
	t                *testing.T
	db               *sql.DB
	d                repository.Dialect
	r                *gin.Engine
	owner, other     uuid.UUID
	project, foreign uuid.UUID
	jwt              *auth.JWTService
	keys             *repository.APIKeyRepository
	mcp              *repository.MCPRepository
}

func newPlatformFixture(t *testing.T) *platformFixture {
	t.Helper()
	dsn := "sqlite://" + filepath.Join(t.TempDir(), "platform.db")
	if os.Getenv("KEEPSAVE_PLATFORM_POSTGRES_TEST") == "1" {
		// Deliberately fixed disposable target; never accept a production DSN.
		dsn = "postgres://keepsave_platform_test:local-test-only@keepsave-platform-postgres-test:5432/keepsave_platform_test?sslmode=disable"
		admin, _, err := repository.NewDB(dsn)
		if err != nil {
			t.Fatal(err)
		}
		schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	t.Cleanup(func() { _ = db.Close() })
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	ur := repository.NewUserRepository(db, d)
	u, err := ur.Create(uuid.NewString()+"@example.invalid", "!")
	if err != nil {
		t.Fatal(err)
	}
	v, err := ur.Create(uuid.NewString()+"@example.invalid", "!")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	pr := repository.NewProjectRepository(db, d)
	er := repository.NewEnvironmentRepository(db, d)
	sr := repository.NewSecretRepository(db, d)
	ar := repository.NewAuditRepository(db, d)
	ps := service.NewProjectService(pr, er, ar, cs)
	ss := service.NewSecretService(sr, pr, er, ar, cs)
	p, err := ps.Create("Authorized fixture", "", u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	q, err := ps.Create("Foreign fixture", "", v.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ss.Create(p.ID, "alpha", "DB_URL", uuid.NewString(), u.ID, ""); err != nil {
		t.Fatal(err)
	}
	j := auth.NewJWTService("disposable-platform-test-signing-key-32-bytes")
	kr := repository.NewAPIKeyRepository(db, d)
	mr := repository.NewMCPRepository(db, d)
	ls := service.NewLeaseService(db, d, ar)
	deny := repository.NewTokenDenylistRepository(db, d)
	j.EnableDenylist(deny)
	a := NewAgentHandler(ls, service.NewAgentAnalyticsService(db, d), service.NewAgentTokenService(j, ls, deny, ar))
	h := NewMCPHubHandler(service.NewMCPService(mr, sr, pr, er, ar), nil)
	r := SetupRouter("http://localhost", true, nil, nil, j, kr, pr,
		&AuthHandler{}, nil, NewSecretHandler(ss), nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, a, nil, nil, nil, h, nil, nil, nil, nil, nil, nil, nil, db, nil)
	return &platformFixture{t, db, d, r, u.ID, v.ID, p.ID, q.ID, j, kr, mr}
}

func (f *platformFixture) key(scopes []string, env *string, expiry *time.Time) (string, uuid.UUID) {
	f.t.Helper()
	raw, hash, err := auth.GenerateAPIKey()
	if err != nil {
		f.t.Fatal(err)
	}
	k, err := f.keys.Create("fixture", hash, f.owner, f.project, scopes, env, expiry)
	if err != nil {
		f.t.Fatal(err)
	}
	return raw, k.ID
}

func (f *platformFixture) call(method, path, body, key string, user uuid.UUID) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if user != uuid.Nil {
		token, err := f.jwt.GenerateToken(user, "fixture@example.invalid")
		if err != nil {
			f.t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	return w
}

func TestPlatformLeaseBoundary(t *testing.T) {
	f := newPlatformFixture(t)
	alpha := "alpha"
	expiry := time.Now().Add(90 * time.Minute)
	key, id := f.key([]string{"read:DB_*", "write"}, &alpha, &expiry)
	path := "/api/v1/projects/" + f.project.String() + "/leases"
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"permitted", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, 201},
		{"wider key", `{"environment":"alpha","secret_keys":["PRIVATE_TOKEN"],"duration_minutes":10}`, 403},
		{"wildcard", `{"environment":"alpha","secret_keys":["*"],"duration_minutes":10}`, 403},
		{"empty selection", `{"environment":"alpha","secret_keys":[],"duration_minutes":10}`, 403},
		{"wider expiry", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":120}`, 403},
		{"wrong environment", `{"environment":"prod","secret_keys":["DB_URL"],"duration_minutes":10}`, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call("POST", path, tc.body, key, uuid.Nil)
			if w.Code != tc.want {
				t.Fatalf("status %d, expected %d", w.Code, tc.want)
			}
		})
	}
	var leaseID, storedKey, userID uuid.UUID
	if err := f.db.QueryRow("SELECT id,api_key_id FROM secret_leases").Scan(&leaseID, &storedKey); err != nil {
		t.Fatal(err)
	}
	if storedKey != id {
		t.Fatal("lease lost API key identity")
	}
	if err := f.db.QueryRow("SELECT user_id FROM audit_log WHERE action='lease.created'").Scan(&userID); err != nil || userID != f.owner {
		t.Fatal("lease audit actor is not human owner", err)
	}
	body := `{"lease_id":"` + leaseID.String() + `","duration_minutes":5}`
	sibling, _ := f.key([]string{"read", "write"}, &alpha, &expiry)
	w := f.call("POST", "/api/v1/projects/"+f.project.String()+"/agent-token", body, sibling, uuid.Nil)
	if w.Code != 403 {
		t.Fatalf("sibling key minted token: %d", w.Code)
	}
	w = f.call("POST", "/api/v1/projects/"+f.project.String()+"/agent-token", body, key, uuid.Nil)
	if w.Code != 201 {
		t.Fatalf("own lease mint: %d", w.Code)
	}
	var minted struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &minted); err != nil {
		t.Fatal(err)
	}
	claims, err := f.jwt.ValidateToken(minted.Token)
	if err != nil {
		t.Fatal(err)
	}
	revokeBody := `{"jti":"` + claims.ID + `"}`
	if wrong := f.call("POST", "/api/v1/projects/"+f.project.String()+"/agent-token/revoke", revokeBody, sibling, uuid.Nil); wrong.Code != 404 {
		t.Fatalf("sibling revoked token: %d", wrong.Code)
	}
	if wrong := f.call("POST", "/api/v1/projects/"+f.foreign.String()+"/agent-token/revoke", revokeBody, "", f.other); wrong.Code != 404 {
		t.Fatalf("foreign project revoked token: %d", wrong.Code)
	}
	req := httptest.NewRequest("GET", "/api/v1/projects/"+f.project.String()+"/secrets?environment=alpha", nil)
	req.Header.Set("Authorization", "Bearer "+minted.Token)
	w = httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("lease-bound read: %d", w.Code)
	}
	w = f.call("GET", path, "", key, uuid.Nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), leaseID.String()) {
		t.Fatal("lease list does not use API key identity")
	}
}

func TestPlatformMCPObjectBoundary(t *testing.T) {
	f := newPlatformFixture(t)
	s := &models.MCPServer{Name: "private", OwnerID: f.owner, GitHubBranch: "main", EntryCommand: "node server.js", Transport: "stdio", Version: "1", Status: "ready", EnvMappings: models.JSONMap{}, ToolDefinitions: models.JSONMap{}}
	if err := f.mcp.CreateServer(s); err != nil {
		t.Fatal(err)
	}
	w := f.call("GET", "/api/v1/mcp/servers/"+s.ID.String(), "", "", f.other)
	if w.Code != 404 {
		t.Fatalf("private server visible: %d", w.Code)
	}
	body := `{"mcp_server_id":"` + s.ID.String() + `"}`
	w = f.call("POST", "/api/v1/mcp/installations", body, "", f.other)
	if w.Code != 404 {
		t.Fatalf("private server installed: %d", w.Code)
	}
	body = `{"mcp_server_id":"` + s.ID.String() + `","project_id":"` + f.foreign.String() + `"}`
	w = f.call("POST", "/api/v1/mcp/installations", body, "", f.owner)
	if w.Code != 404 {
		t.Fatalf("foreign project bound: %d", w.Code)
	}
	inst := &models.MCPInstallation{UserID: f.owner, MCPServerID: s.ID, Enabled: true, Config: models.JSONMap{}}
	if err := f.mcp.CreateInstallation(inst); err != nil {
		t.Fatal(err)
	}
	w = f.call("PUT", "/api/v1/mcp/installations/"+inst.ID.String(), `{"enabled":false,"config":{}}`, "", f.other)
	if w.Code != 404 {
		t.Fatalf("foreign installation changed: %d", w.Code)
	}
	stored, err := f.mcp.GetInstallation(f.owner, s.ID)
	if err != nil || !stored.Enabled {
		t.Fatal("denied update changed installation")
	}
	w = f.call("PUT", "/api/v1/mcp/installations/"+inst.ID.String(), `{"enabled":false,"config":{}}`, "", f.owner)
	if w.Code != 200 {
		t.Fatalf("owner update: %d", w.Code)
	}
	var count int
	if err = f.db.QueryRow("SELECT COUNT(*) FROM audit_log WHERE action='mcp.installation_updated'").Scan(&count); err != nil || count != 1 {
		t.Fatal("installation audit missing or includes denied mutation", err)
	}
}

func TestPlatformParentRevocation(t *testing.T) {
	for _, change := range []string{"expiry", "scope", "delete", "token"} {
		t.Run(change, func(t *testing.T) {
			f := newPlatformFixture(t)
			key, id := f.key([]string{"read"}, nil, nil)
			w := f.call("POST", "/api/v1/projects/"+f.project.String()+"/leases", `{"environment":"alpha","secret_keys":["DB_URL"],"duration_minutes":10}`, key, uuid.Nil)
			if w.Code != 201 {
				t.Fatalf("create %d", w.Code)
			}
			var result struct {
				Lease struct {
					ID uuid.UUID `json:"id"`
				} `json:"lease"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			w = f.call("POST", "/api/v1/projects/"+f.project.String()+"/agent-token", `{"lease_id":"`+result.Lease.ID.String()+`","duration_minutes":5}`, key, uuid.Nil)
			if w.Code != 201 {
				t.Fatalf("mint %d", w.Code)
			}
			var token struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &token); err != nil {
				t.Fatal(err)
			}
			claims, err := f.jwt.ValidateToken(token.Token)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "expiry":
				_, err = f.db.Exec(repository.Q(f.d, `UPDATE api_keys SET expires_at=$1 WHERE id=$2`), time.Now().Add(-time.Hour), id)
			case "scope":
				scopes, _ := f.d.ArrayParam([]string{"read:OTHER"})
				_, err = f.db.Exec(repository.Q(f.d, `UPDATE api_keys SET scopes=$1 WHERE id=$2`), scopes, id)
			case "delete":
				_, err = f.db.Exec(repository.Q(f.d, `DELETE FROM api_keys WHERE id=$1`), id)
			case "token":
				err = repository.NewTokenDenylistRepository(f.db, f.d).Revoke(claims.ID, nil, time.Now().Add(time.Hour), "another instance")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.jwt.ValidateToken(token.Token); err == nil {
				t.Fatal("revoked parent/token still accepted")
			}
		})
	}
}

func TestPlatformRoleBoundary(t *testing.T) {
	f := newPlatformFixture(t)
	org := uuid.New()
	_, err := f.db.Exec(repository.Q(f.d, `INSERT INTO organizations(id,name,slug,owner_id) VALUES($1,'fixture',$2,$3)`), org, org.String(), f.owner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Exec(repository.Q(f.d, `UPDATE projects SET organization_id=$1 WHERE id=$2`), org, f.project)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Exec(repository.Q(f.d, `INSERT INTO organization_members(id,organization_id,user_id,role) VALUES($1,$2,$3,'viewer')`), uuid.New(), org, f.other)
	if err != nil {
		t.Fatal(err)
	}
	// Template application bypasses the :id router group; its service must
	// enforce the same live role before reading or writing credential data.
	templateSvc := service.NewTemplateService(nil, repository.NewSecretRepository(f.db, f.d), repository.NewProjectRepository(f.db, f.d), repository.NewEnvironmentRepository(f.db, f.d), nil, nil)
	if _, err = templateSvc.ApplyTemplate(uuid.New(), f.project, "alpha", f.other); !errors.Is(err, service.ErrTemplateProjectAccess) {
		t.Fatal("viewer template mutation accepted", err)
	}
	path := "/api/v1/projects/" + f.project.String() + "/secrets"
	for _, method := range []string{"GET", "POST"} {
		w := f.call(method, path+"?environment=alpha", `{"environment":"alpha","key":"DENIED","value":"fixture"}`, "", f.other)
		if w.Code != 403 {
			t.Fatalf("viewer %s status %d", method, w.Code)
		}
	}
	for _, path := range []string{"/env-export", "/verify-encryption", "/dependencies/graph", "/webhooks"} {
		if w := f.call("GET", "/api/v1/projects/"+f.project.String()+path, "", "", f.other); w.Code != 403 {
			t.Fatalf("viewer credential route %s: %d", path, w.Code)
		}
	}
	_, err = f.db.Exec(repository.Q(f.d, `UPDATE organization_members SET role='editor' WHERE user_id=$1`), f.other)
	if err != nil {
		t.Fatal(err)
	}
	if w := f.call("GET", path+"?environment=alpha", "", "", f.other); w.Code != 200 {
		t.Fatalf("editor %d", w.Code)
	}
	for _, path := range []string{"/rotate-keys", "/webhooks", "/backups", "/promotions/" + uuid.NewString() + "/approve"} {
		if w := f.call("POST", "/api/v1/projects/"+f.project.String()+path, `{}`, "", f.other); w.Code != 403 {
			t.Fatalf("editor administrator/promoter route %s: %d", path, w.Code)
		}
	}
	_, err = f.db.Exec(repository.Q(f.d, `DELETE FROM organization_members WHERE user_id=$1`), f.other)
	if err != nil {
		t.Fatal(err)
	}
	if w := f.call("GET", path+"?environment=alpha", "", "", f.other); w.Code != 403 {
		t.Fatalf("removed member %d", w.Code)
	}
}
