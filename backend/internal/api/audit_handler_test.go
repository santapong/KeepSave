package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// auditHandlerEnv drives the REAL state-mutating HTTP handlers
// (secret/project/api-key) through the production handler→service→repo stack
// over in-memory SQLite, so we can assert that each mutation writes its
// canonical audit_log row (CLAUDE.md audit-log invariant / docs/AUDIT_LOG_COVERAGE.md).
//
// Auth is provided by stubAuth (defined in negative_auth_test.go) which sets
// user_id; RequireProjectAccess then runs for real against the seeded project.
// Audit rows are asserted by counting audit_log rows for the expected action.
type auditHandlerEnv struct {
	db      *sql.DB
	router  *gin.Engine
	crypto  *crypto.Service
	projSvc *service.ProjectService
	owner   uuid.UUID
}

func newAuditHandlerEnv(t *testing.T) *auditHandlerEnv {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	for _, ddl := range []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			owner_id TEXT NOT NULL,
			organization_id TEXT,
			encrypted_dek BLOB,
			dek_nonce BLOB,
			allowed_origins TEXT NOT NULL DEFAULT '[]',
			embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE environments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE secrets (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			environment_id TEXT NOT NULL,
			key TEXT NOT NULL,
			encrypted_value BLOB,
			value_nonce BLOB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			hashed_key TEXT NOT NULL,
			user_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			scopes TEXT NOT NULL DEFAULT '[]',
			environment TEXT,
			expires_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE organization_members (
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL
		)`,
		// a02-shape audit_log: DB-default id + created_at so the unchained
		// AuditRepository.Create insert lands cleanly on SQLite.
		`CREATE TABLE audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT,
			project_id TEXT,
			action TEXT,
			environment TEXT,
			details TEXT,
			ip_address TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	dialect := repository.NewDialect(repository.DBTypeSQLite)
	cryptoSvc, err := crypto.NewService(bytes.Repeat([]byte{0x2a}, 32))
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}

	projectRepo := repository.NewProjectRepository(db, dialect)
	envRepo := repository.NewEnvironmentRepository(db, dialect)
	secretRepo := repository.NewSecretRepository(db, dialect)
	apikeyRepo := repository.NewAPIKeyRepository(db, dialect)
	auditRepo := repository.NewAuditRepository(db, dialect)

	projSvc := service.NewProjectService(projectRepo, envRepo, auditRepo, cryptoSvc)
	secretSvc := service.NewSecretService(secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	apikeySvc := service.NewAPIKeyService(apikeyRepo, projectRepo, auditRepo)

	projectHandler := NewProjectHandler(projSvc)
	secretHandler := NewSecretHandler(secretSvc)
	apikeyHandler := NewAPIKeyHandler(apikeySvc)

	owner := uuid.New()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The owner is injected by stubAuth via X-Test-User-ID.
	r.POST("/projects", stubAuth(), projectHandler.Create)
	r.DELETE("/projects/:id", stubAuth(), RequireProjectAccess(projectRepo), projectHandler.Delete)

	secrets := r.Group("/projects/:id", stubAuth(), RequireProjectAccess(projectRepo))
	{
		secrets.POST("/secrets", secretHandler.Create)
		secrets.PUT("/secrets/:secretId", secretHandler.Update)
		secrets.DELETE("/secrets/:secretId", secretHandler.Delete)
	}

	r.POST("/api-keys", stubAuth(), apikeyHandler.Create)
	r.DELETE("/api-keys/:id", stubAuth(), apikeyHandler.Delete)

	return &auditHandlerEnv{db: db, router: r, crypto: cryptoSvc, projSvc: projSvc, owner: owner}
}

// do issues a request as the env owner and returns status + body.
func (e *auditHandlerEnv) do(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Test-User-ID", e.owner.String())
	e.router.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// auditCount returns the number of audit_log rows for the given action.
func (e *auditHandlerEnv) auditCount(t *testing.T, action string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, action).Scan(&n); err != nil {
		t.Fatalf("count audit action=%s: %v", action, err)
	}
	return n
}

// requireAuditRow asserts exactly `want` audit rows exist for the action.
func (e *auditHandlerEnv) requireAuditRow(t *testing.T, action string, want int) {
	t.Helper()
	if got := e.auditCount(t, action); got != want {
		t.Fatalf("audit action=%s rows=%d, want %d", action, got, want)
	}
}

// seedProject creates a project (with a valid DEK + default environments) via
// the real ProjectService so downstream secret encryption works. It bypasses
// the HTTP layer so project.created is not counted against secret/apikey tests.
func (e *auditHandlerEnv) seedProject(t *testing.T) uuid.UUID {
	t.Helper()
	p, err := e.projSvc.Create("proj", "", e.owner, "127.0.0.1")
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	// Clear the project.created row this seed emitted so per-test asserts are clean.
	if _, err := e.db.Exec(`DELETE FROM audit_log`); err != nil {
		t.Fatalf("clear seed audit: %v", err)
	}
	return p.ID
}

// TestHandlerAudit_SecretCreateUpdateDelete drives the secret CRUD handlers and
// asserts each mutation wrote its canonical audit row. The assertion is a direct
// COUNT on audit_log for the action string the service emits via emitAudit →
// AuditRepository.Create, proving the handler→service→audit path is wired.
func TestHandlerAudit_SecretCreateUpdateDelete(t *testing.T) {
	env := newAuditHandlerEnv(t)
	pid := env.seedProject(t)
	base := "/projects/" + pid.String() + "/secrets"

	// Create
	code, body := env.do(t, http.MethodPost, base, `{"key":"API_TOKEN","value":"s3cr3t","environment":"alpha"}`)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "secret.created", 1)

	// Extract the created secret id.
	var created struct {
		Secret struct {
			ID string `json:"id"`
		} `json:"secret"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if created.Secret.ID == "" {
		t.Fatalf("no secret id in create body: %s", body)
	}

	// Update
	code, body = env.do(t, http.MethodPut, base+"/"+created.Secret.ID, `{"value":"rotated"}`)
	if code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "secret.updated", 1)

	// Delete
	code, body = env.do(t, http.MethodDelete, base+"/"+created.Secret.ID, "")
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "secret.deleted", 1)
}

// TestHandlerAudit_ProjectCreateDelete drives the project create/delete handlers
// and asserts the project.created / project.deleted audit rows.
func TestHandlerAudit_ProjectCreateDelete(t *testing.T) {
	env := newAuditHandlerEnv(t)

	code, body := env.do(t, http.MethodPost, "/projects", `{"name":"my-project","description":"d"}`)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "project.created", 1)

	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if created.Project.ID == "" {
		t.Fatalf("no project id in body: %s", body)
	}

	code, body = env.do(t, http.MethodDelete, "/projects/"+created.Project.ID, "")
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "project.deleted", 1)
}

// TestHandlerAudit_APIKeyCreateDelete drives the api-key create/delete handlers
// and asserts the apikey.created / apikey.deleted audit rows.
func TestHandlerAudit_APIKeyCreateDelete(t *testing.T) {
	env := newAuditHandlerEnv(t)
	pid := env.seedProject(t)

	code, body := env.do(t, http.MethodPost, "/api-keys",
		`{"name":"ci-key","project_id":"`+pid.String()+`","scopes":["read"]}`)
	if code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "apikey.created", 1)

	var created struct {
		APIKey struct {
			ID string `json:"id"`
		} `json:"api_key"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if created.APIKey.ID == "" {
		t.Fatalf("no api key id in body: %s", body)
	}

	code, body = env.do(t, http.MethodDelete, "/api-keys/"+created.APIKey.ID, "")
	if code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body=%s)", code, body)
	}
	env.requireAuditRow(t, "apikey.deleted", 1)
}
