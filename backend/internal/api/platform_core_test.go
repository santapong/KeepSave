package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auditview"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/diagnostics"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/policy"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/vault"
	"net/http/httptest"
	"strings"
	"testing"
)

type coreFixture struct {
	*platformFixture
	v      *vault.Service
	tokens map[uuid.UUID]string
	org    *service.OrganizationService
}

func newCoreFixture(t *testing.T) *coreFixture {
	f := newPlatformFixture(t)
	if f.d.DBType() != repository.DBTypePostgres {
		t.Skip("PostgreSQL core journal contract")
	}
	ctx := context.Background()
	cs, _ := crypto.NewService(make([]byte, 32))
	ar := repository.NewAuditRepository(f.db, f.d)
	v := vault.New(f.db, cs, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d}}).Authorize(ctx, p, a, r)
	}, ar)
	for _, item := range []struct{ owner, project uuid.UUID }{{f.owner, f.project}, {f.other, f.foreign}} {
		if err := v.Enroll(ctx, policy.Principal{Kind: policy.Human, SubjectID: item.owner, ActorID: item.owner}, item.project); err != nil {
			t.Fatal(err)
		}
	}
	jwt := auth.NewJWTService("disposable-core-session-key-more-than-32-bytes")
	sessions := service.NewSessionService(f.db, f.d, jwt, ar)
	jwt.EnableHumanSessions(sessions)
	v = vault.New(f.db, cs, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, ar)
	ur := repository.NewUserRepository(f.db, f.d)
	pr := repository.NewProjectRepository(f.db, f.d)
	er := repository.NewEnvironmentRepository(f.db, f.d)
	sr := repository.NewSecretRepository(f.db, f.d)
	ps := service.NewProjectService(pr, er, ar, cs)
	ps.EnableVault(v)
	ps.EnableSessions(sessions)
	ss := service.NewSecretService(sr, pr, er, ar, cs)
	ss.EnableVault(v)
	apiKeys := service.NewAPIKeyService(f.keys, pr, ar)
	apiKeys.EnableSessions(sessions)
	promos := service.NewPromotionService(repository.NewPromotionRepository(f.db, f.d), sr, pr, er, ar, cs)
	promos.EnableVault(v)
	rotations := service.NewKeyRotationService(pr, sr, er, ar, cs)
	rotations.EnableVault(v)
	envs := service.NewEnvFileService(sr, pr, er, ar, cs)
	envs.EnableVault(v)
	templates := service.NewTemplateService(repository.NewTemplateRepository(f.db, f.d), sr, pr, er, ar, cs)
	templates.EnableVault(v)
	templates.EnableSessions(sessions)
	org := service.NewOrganizationService(repository.NewOrganizationRepository(f.db, f.d), ar)
	org.EnableSessions(sessions)
	versions := NewVersionHandler(repository.NewSecretVersionRepository(f.db, f.d), sr, pr, cs)
	versions.EnableVault(v)
	login := service.NewAuthService(ur, repository.NewAuthAttemptsRepository(f.db, f.d), ar, jwt)
	login.EnableSessions(sessions)
	r := NewRouter(Dependencies{CoreRelease: true, DisableLocalMCP: true, CORSOrigins: "http://localhost", PromotionsEnabled: true, JWTService: jwt, APIKeyRepo: f.keys, ProjectRepo: pr, AuthHandler: NewAuthHandler(login), SessionHandler: NewSessionHandler(sessions), ProjectHandler: NewProjectHandler(ps), SecretHandler: NewSecretHandler(ss), APIKeyHandler: NewAPIKeyHandler(apiKeys), PromotionHandler: NewPromotionHandler(promos), KeyRotationHandler: NewKeyRotationHandler(rotations), EnvFileHandler: NewEnvFileHandler(envs), TemplateHandler: NewTemplateHandler(templates), VersionHandler: versions, OrgHandler: NewOrganizationHandler(org), RecoveryHandler: NewRecoveryHandler(v), TeamVaultHandler: NewTeamVaultHandler(v, auditview.New(f.db, func(ctx context.Context, tx *sql.Tx, p policy.Principal, a policy.Action, r policy.Resource) (policy.Decision, error) {
		return (policy.Evaluator{Store: repository.AuthorityStore{DB: tx, Dialect: f.d, RequireHumanSession: true}}).Authorize(ctx, p, a, r)
	}, ar), &diagnostics.Service{DB: f.db, Config: diagnostics.Config{PostgreSQL: true}}, true), OperatorAdminChecker: repository.NewPlatformAdminRepository(f.db, f.d, ar), DB: f.db})
	c := &coreFixture{f, v, map[uuid.UUID]string{}, org}
	f.r = r
	f.jwt = jwt
	for _, userID := range []uuid.UUID{f.owner, f.other} {
		u, err := ur.GetByID(userID)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := f.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		token, err := sessions.IssueTx(ctx, tx, u, "fixture", "", "")
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
		c.tokens[userID] = token
	}
	return c
}
func (f *coreFixture) request(method, path, body string, user uuid.UUID, key string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != uuid.Nil {
		req.Header.Set("Authorization", "Bearer "+f.tokens[user])
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	return w
}
func coreDecode[T any](t *testing.T, w *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d (response omitted to protect values)", w.Code, status)
	}
	var value T
	if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func (f *coreFixture) put(environment, key, value string) models.Secret {
	f.t.Helper()
	body, _ := json.Marshal(map[string]string{"environment": environment, "key": key, "value": value})
	result := coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](f.t, f.request("POST", "/api/v1/projects/"+f.project.String()+"/secrets", string(body), f.owner, ""), 201)
	return result.Secret
}
func TestPlatformCoreRouterVaultAndSessionJourney(t *testing.T) {
	f := newCoreFixture(t)
	base := "/api/v1/projects/" + f.project.String()
	s := f.put("alpha", "VISIBLE", "${PRIVATE}")
	f.put("alpha", "PRIVATE", "synthetic-not-a-real-secret")
	alpha := "alpha"
	key, _ := f.key([]string{"read:VISIBLE"}, &alpha, nil)
	if w := f.request("GET", base+"/secrets?environment=alpha&resolve=true", "", uuid.Nil, key); w.Code != 403 {
		t.Fatal("scoped dependency expansion was not denied", w.Code)
	}
	batch := coreDecode[struct {
		Secrets []models.Secret `json:"secrets"`
		Missing []string        `json:"missing_keys"`
	}](t, f.request("POST", base+"/secrets/batch", `{"environment":"alpha","keys":["VISIBLE","PRIVATE","ABSENT"]}`, uuid.Nil, key), 200)
	if len(batch.Secrets) != 1 || batch.Secrets[0].Key != "VISIBLE" || len(batch.Missing) != 2 {
		t.Fatal("batch scope/missing contract")
	}
	if w := f.request("POST", base+"/secrets/batch", `{"environment":"alpha","keys":[]}`, f.owner, ""); w.Code != 400 {
		t.Fatal("empty batch accepted")
	}
	secretPath := base + "/secrets/" + s.ID.String()
	changed := coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, f.request("PUT", secretPath, `{"value":"new-synthetic","expected_revision":1}`, f.owner, ""), 200).Secret
	if changed.Revision != 2 {
		t.Fatal("missing revision")
	}
	history := coreDecode[[]vault.Revision](t, f.request("GET", secretPath+"/versions", "", f.owner, ""), 200)
	if len(history) != 2 || strings.Contains(f.request("GET", secretPath+"/versions", "", f.owner, "").Body.String(), "new-synthetic") {
		t.Fatal("history is not metadata-only")
	}
	restored := coreDecode[vault.Record](t, f.request("POST", secretPath+"/versions/1/restore", `{"expected_current_revision":2}`, f.owner, ""), 200)
	if restored.Revision != 3 {
		t.Fatal("restore did not append")
	}
	if w := f.request("POST", secretPath+"/versions/1/restore", `{"expected_current_revision":2}`, f.owner, ""); w.Code != 409 {
		t.Fatal("stale restore accepted", w.Code)
	}
	if w := f.request("GET", secretPath, "", f.other, ""); w.Code != 403 {
		t.Fatal("foreign secret read", w.Code)
	}
	// Required audit failure rolls back both current value and its revision.
	if _, err := f.db.Exec(`ALTER TABLE audit_log ADD CONSTRAINT fail_core_update CHECK(action<>'secret.updated') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if w := f.request("PUT", secretPath, `{"value":"must-not-commit","expected_revision":3}`, f.owner, ""); w.Code != 500 {
		t.Fatal("audit failure was not fatal", w.Code)
	}
	var revision int64
	if err := f.db.QueryRow(`SELECT revision FROM vault_entries WHERE secret_id=$1`, s.ID).Scan(&revision); err != nil || revision != 3 {
		t.Fatal("partial journal commit", err)
	}
	if _, err := f.db.Exec(`ALTER TABLE audit_log DROP CONSTRAINT fail_core_update`); err != nil {
		t.Fatal(err)
	}
	// Tombstoning retains history and denies every dependent key.
	if w := f.request("DELETE", base, "", f.owner, ""); w.Code != 204 {
		t.Fatal("project archive", w.Code)
	}
	if w := f.request("DELETE", base, "", f.owner, ""); w.Code != 204 {
		t.Fatal("archive retry", w.Code)
	}
	if w := f.request("POST", base+"/secrets/batch", `{"environment":"alpha","keys":["VISIBLE"]}`, uuid.Nil, key); w.Code != 401 {
		t.Fatal("archived key retained access", w.Code)
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM vault_revisions WHERE secret_id=$1`, s.ID).Scan(&count); err != nil || count != 4 {
		t.Fatal("archive discarded history", err)
	}
	if w := f.request("POST", "/api/v1/auth/logout", "", f.owner, ""); w.Code != 204 {
		t.Fatal("logout", w.Code)
	}
	if w := f.request("GET", "/api/v1/projects", "", f.owner, ""); w.Code != 401 {
		t.Fatal("revoked session continued", w.Code)
	}
}
func TestPlatformCorePromotionRotationRollbackAndApproval(t *testing.T) {
	f := newCoreFixture(t)
	base := "/api/v1/projects/" + f.project.String()
	f.put("alpha", "PROMOTED", "source-synthetic")
	old := f.put("uat", "PROMOTED", "prior-synthetic")
	result := coreDecode[struct {
		Promotion models.PromotionRequest `json:"promotion"`
	}](t, f.request("POST", base+"/promote", `{"source_environment":"alpha","target_environment":"uat","keys":["PROMOTED"],"override_policy":"overwrite"}`, f.owner, ""), 200)
	if result.Promotion.Status != "completed" {
		t.Fatal("promotion status")
	}
	if w := f.request("POST", base+"/rotate-keys", "", f.owner, ""); w.Code != 200 {
		t.Fatal("rotation", w.Code)
	}
	if _, err := repository.NewPromotionRepository(f.db, f.d).GetByID(result.Promotion.ID); err != nil {
		t.Fatalf("promotion metadata read: %v", err)
	}
	rollback := base + "/promotions/" + result.Promotion.ID.String() + "/rollback"
	if w := f.request("POST", rollback, "", f.owner, ""); w.Code != 200 {
		t.Fatal("rollback after rotation", w.Code)
	}
	restored := coreDecode[struct {
		Secret models.Secret `json:"secret"`
	}](t, f.request("GET", base+"/secrets/"+old.ID.String(), "", f.owner, ""), 200).Secret
	if restored.Value != "prior-synthetic" || restored.Revision != 4 {
		t.Fatal("rollback lost retained snapshot")
	}
	org, err := f.org.Create("Approval fixture", f.owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.org.AssignProjectWithAudit(org.ID, f.owner, f.project, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = f.org.AddMember(org.ID, f.owner, f.other, "promoter", ""); err != nil {
		t.Fatal(err)
	}
	pending := coreDecode[struct {
		Promotion models.PromotionRequest `json:"promotion"`
	}](t, f.request("POST", base+"/promote", `{"source_environment":"uat","target_environment":"prod","keys":["PROMOTED"],"override_policy":"overwrite"}`, f.owner, ""), 202).Promotion
	approve := base + "/promotions/" + pending.ID.String() + "/approve"
	if w := f.request("POST", approve, "", f.owner, ""); w.Code != 403 {
		t.Fatal("self approval", w.Code)
	}
	if w := f.request("PUT", base+"/secrets/"+old.ID.String(), `{"value":"changed-after-request"}`, f.owner, ""); w.Code != 200 {
		t.Fatal("edit", w.Code)
	}
	if w := f.request("POST", approve, "", f.other, ""); w.Code != 409 {
		t.Fatal("changed artifact approval accepted", w.Code)
	}
	pending = coreDecode[struct {
		Promotion models.PromotionRequest `json:"promotion"`
	}](t, f.request("POST", base+"/promote", `{"source_environment":"uat","target_environment":"prod","keys":["PROMOTED"],"override_policy":"overwrite"}`, f.owner, ""), 202).Promotion
	if w := f.request("POST", base+"/promotions/"+pending.ID.String()+"/approve", "", f.other, ""); w.Code != 200 {
		t.Fatal("independent eligible approval", w.Code)
	}
}
