package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/migrations"
)

// TestPlatformMySQLLegacy exercises shipped MySQL migrations and preserved
// user-facing routes. The new journal/recovery guarantees are not enabled.
func TestPlatformMySQLLegacy(t *testing.T) {
	if os.Getenv("KEEPSAVE_MYSQL_TEST") != "1" {
		t.Skip("fixed disposable MySQL compatibility target")
	}
	const baseDSN = "mysql://root:local-test-only@keepsave-backend-mysql-test-20261002:3306/"
	admin, _, err := repository.NewDB(baseDSN + "mysql")
	if err != nil {
		t.Fatal(err)
	}
	database := "keepsave_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec("CREATE DATABASE " + database + " CHARACTER SET utf8mb4"); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + database); _ = admin.Close() })
	db, d, err := repository.NewDB(baseDSN + database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	cs, err := crypto.NewService(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ur := repository.NewUserRepository(db, d)
	pr := repository.NewProjectRepository(db, d)
	er := repository.NewEnvironmentRepository(db, d)
	sr := repository.NewSecretRepository(db, d)
	ar := repository.NewAuditRepository(db, d)
	ar.SetChainKey(cs.DeriveAuditChainKey())
	kr := repository.NewAPIKeyRepository(db, d)
	jwt := auth.NewJWTService("synthetic-mysql-session-signing-key-at-least-32-bytes")
	sessions := service.NewSessionService(db, d, jwt, ar)
	jwt.EnableHumanSessions(sessions)
	login := service.NewAuthService(ur, repository.NewAuthAttemptsRepository(db, d), ar, jwt)
	login.EnableSessions(sessions)
	apiKeys := service.NewAPIKeyService(kr, pr, ar)
	apiKeys.EnableSessions(sessions)
	r := NewRouter(Dependencies{CoreRelease: true, DisableLocalMCP: true, PromotionsEnabled: true, CORSOrigins: "http://localhost", DB: db, JWTService: jwt, APIKeyRepo: kr, ProjectRepo: pr,
		AuthHandler: NewAuthHandler(login), SessionHandler: NewSessionHandler(sessions), ProjectHandler: NewProjectHandler(service.NewProjectService(pr, er, ar, cs)), SecretHandler: NewSecretHandler(service.NewSecretService(sr, pr, er, ar, cs)),
		APIKeyHandler: NewAPIKeyHandler(apiKeys), OrgHandler: NewOrganizationHandler(service.NewOrganizationService(repository.NewOrganizationRepository(db, d), ar)), VersionHandler: NewVersionHandler(repository.NewSecretVersionRepository(db, d), sr, pr, cs), RecoveryHandler: NewRecoveryHandler(nil)})
	call := func(method, path, body, token, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	const password = "Synthetic-Test-Only-2026!"
	email := uuid.NewString() + "@example.invalid"
	signupBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	registered := coreDecode[service.AuthResponse](t, call("POST", "/api/v1/auth/register", string(signupBody), "", ""), 201)
	if registered.User == nil || registered.Token == "" {
		t.Fatal("signup did not create account/session")
	}
	signed := coreDecode[service.AuthResponse](t, call("POST", "/api/v1/auth/login", string(signupBody), "", ""), 200)
	if signed.User.ID != registered.User.ID || signed.Token == registered.Token {
		t.Fatal("login session identity/rotation")
	}
	current := coreDecode[struct {
		Sessions []models.SessionView `json:"sessions"`
	}](t, call("GET", "/api/v1/account/sessions", "", signed.Token, ""), 200)
	if len(current.Sessions) != 2 {
		t.Fatal("stored login sessions", len(current.Sessions))
	}
	project := coreDecode[struct {
		Project models.Project `json:"project"`
	}](t, call("POST", "/api/v1/projects", `{"name":"MySQL compatibility","description":"synthetic"}`, signed.Token, ""), 201).Project
	base := "/api/v1/projects/" + project.ID.String()
	visible := coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, call("POST", base+"/secrets", `{"environment":"alpha","key":"PUBLIC","value":"synthetic-public"}`, signed.Token, ""), 201).Secret
	coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, call("POST", base+"/secrets", `{"environment":"alpha","key":"PRIVATE","value":"synthetic-private"}`, signed.Token, ""), 201)
	keyBody, _ := json.Marshal(map[string]any{"name": "scoped", "project_id": project.ID.String(), "environment": "alpha", "scopes": []string{"read:PUBLIC", "write:PUBLIC"}})
	key := coreDecode[service.CreateAPIKeyResponse](t, call("POST", "/api/v1/api-keys", string(keyBody), signed.Token, ""), 201)
	if key.RawKey == "" {
		t.Fatal("API key missing")
	}
	listed := coreDecode[struct {
		Secrets []models.Secret `json:"secrets"`
	}](t, call("GET", base+"/secrets?environment=alpha", "", "", key.RawKey), 200)
	if len(listed.Secrets) != 1 || listed.Secrets[0].Key != "PUBLIC" {
		t.Fatal("scoped list widened")
	}
	batch := coreDecode[struct {
		Secrets []models.Secret `json:"secrets"`
		Missing []string        `json:"missing_keys"`
	}](t, call("POST", base+"/secrets/batch", `{"environment":"alpha","keys":["PUBLIC","PRIVATE","MISSING"]}`, "", key.RawKey), 200)
	if len(batch.Secrets) != 1 || len(batch.Missing) != 2 {
		t.Fatal("scoped batch/missing parity")
	}
	secretPath := base + "/secrets/" + visible.ID.String()
	changed := coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, call("PUT", secretPath, `{"value":"synthetic-updated"}`, "", key.RawKey), 200).Secret
	if changed.Value != "synthetic-updated" {
		t.Fatal("compatibility update not persisted")
	}
	org := coreDecode[struct {
		Organization models.Organization `json:"organization"`
	}](t, call("POST", "/api/v1/organizations", `{"name":"MySQL team"}`, signed.Token, ""), 201).Organization
	coreDecode[struct {
		Organization models.Organization `json:"organization"`
	}](t, call("GET", "/api/v1/organizations/"+org.ID.String(), "", signed.Token, ""), 200)
	foreignBody, _ := json.Marshal(map[string]string{"email": uuid.NewString() + "@example.invalid", "password": password})
	foreign := coreDecode[service.AuthResponse](t, call("POST", "/api/v1/auth/register", string(foreignBody), "", ""), 201)
	if w := call("GET", secretPath, "", foreign.Token, ""); w.Code != 403 {
		t.Fatal("cross-tenant secret access", w.Code)
	}
	orgBase := "/api/v1/organizations/" + org.ID.String()
	memberBody, _ := json.Marshal(map[string]string{"user_id": foreign.User.ID.String(), "role": "viewer"})
	coreDecode[struct {
		Member models.OrgMember `json:"member"`
	}](t, call("POST", orgBase+"/members", string(memberBody), signed.Token, ""), 201)
	assignmentBody, _ := json.Marshal(map[string]string{"project_id": project.ID.String()})
	if w := call("POST", orgBase+"/projects", string(assignmentBody), signed.Token, ""); w.Code != 200 {
		t.Fatal("organization project assignment", w.Code)
	}
	coreDecode[struct {
		Project models.Project `json:"project"`
	}](t, call("GET", base, "", foreign.Token, ""), 200)
	if w := call("GET", secretPath, "", foreign.Token, ""); w.Code != 403 {
		t.Fatal("organization viewer value access", w.Code)
	}
	if w := call("PUT", secretPath, `{"value":"viewer-write-denied"}`, foreign.Token, ""); w.Code != 403 {
		t.Fatal("organization viewer mutation", w.Code)
	}
	if w := call("GET", secretPath, "", "", key.RawKey); w.Code != 401 {
		t.Fatal("personal API key survived organization assignment", w.Code)
	}
	if w := call("PUT", orgBase+"/members/"+foreign.User.ID.String(), `{"role":"editor"}`, signed.Token, ""); w.Code != 200 {
		t.Fatal("organization role update", w.Code)
	}
	coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, call("GET", secretPath, "", foreign.Token, ""), 200)
	if w := call("DELETE", orgBase+"/members/"+foreign.User.ID.String(), "", signed.Token, ""); w.Code != 204 {
		t.Fatal("organization membership revocation", w.Code)
	}
	if w := call("GET", secretPath, "", foreign.Token, ""); w.Code != 403 {
		t.Fatal("removed organization member retained access", w.Code)
	}
	// The MySQL migration adapter must enforce self-approval at the database
	// boundary, including writes that bypass the application service.
	promotion, err := repository.NewPromotionRepository(db, d).Create(project.ID, "alpha", "prod", signed.User.ID, nil, "skip", "synthetic")
	if err != nil {
		t.Fatal("legacy promotion fixture", err)
	}
	if _, err = db.Exec(`UPDATE promotion_requests SET approved_by=? WHERE id=?`, signed.User.ID, promotion.ID); err == nil {
		t.Fatal("database accepted self-approved update")
	}
	if _, err = db.Exec(`INSERT INTO promotion_requests(id,project_id,source_environment,target_environment,requested_by,approved_by) VALUES(?,?,?,?,?,?)`, uuid.New(), project.ID, "alpha", "prod", signed.User.ID, signed.User.ID); err == nil {
		t.Fatal("database accepted self-approved insert")
	}
	if w := call("POST", base+"/backups", "{}", signed.Token, ""); w.Code != 503 {
		t.Fatal("unsupported durable backup advertised/executed", w.Code)
	}
	teamKey := coreDecode[service.CreateAPIKeyResponse](t, call("POST", "/api/v1/api-keys", string(keyBody), signed.Token, ""), 201)
	if w := call("DELETE", secretPath, "", "", teamKey.RawKey); w.Code != 204 {
		t.Fatal("write implies delete compatibility", w.Code)
	}
	if w := call("POST", "/api/v1/auth/logout", "", signed.Token, ""); w.Code != 204 {
		t.Fatal("logout", w.Code)
	}
	if w := call("GET", "/api/v1/account/sessions", "", signed.Token, ""); w.Code != 401 {
		t.Fatal("revoked session accepted", w.Code)
	}
	if w := call("GET", "/api/v1/account/sessions", "", registered.Token, ""); w.Code != 200 {
		t.Fatal("logout revoked sibling session", w.Code)
	}
}
