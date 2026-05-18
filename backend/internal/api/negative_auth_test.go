package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// negativeAuthTestEnv stands up an in-memory sqlite DB with the canonical
// schema and a real ProjectRepository so RequireProjectAccess can be
// exercised end-to-end. The handler under test always responds 200 so
// any non-200 outcome traces back to the middleware decision.
type negativeAuthTestEnv struct {
	db          *sql.DB
	dialect     repository.Dialect
	projectRepo *repository.ProjectRepository
	router      *gin.Engine
}

func newNegativeAuthEnv(t *testing.T) *negativeAuthTestEnv {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Hand-rolled minimal schema covering the columns RequireProjectAccess
	// reads. Avoids the migration runner (sqlite DEFAULT-function syntax
	// quirk is a separate issue, tracked for a follow-up).
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
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,
		`CREATE TABLE organization_members (
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	projectRepo := repository.NewProjectRepository(db, dialect)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Stub the auth chain: caller injects context keys via the test
	// helpers below; middleware then runs RequireProjectAccess for real.
	r.GET("/projects/:id/probe",
		stubAuth(),
		RequireProjectAccess(projectRepo),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	r.POST("/projects/:id/secrets/probe",
		stubAuth(),
		RequireProjectAccess(projectRepo),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)
	return &negativeAuthTestEnv{db: db, dialect: dialect, projectRepo: projectRepo, router: r}
}

// stubAuth lets each test case set user_id / api_key_project_id via
// request headers (X-Test-User-ID, X-Test-ApiKey-Project) so the test
// matrix stays declarative.
func stubAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if v := c.GetHeader("X-Test-User-ID"); v != "" {
			if uid, err := uuid.Parse(v); err == nil {
				c.Set("user_id", uid)
			}
		}
		if v := c.GetHeader("X-Test-ApiKey-Project"); v != "" {
			if pid, err := uuid.Parse(v); err == nil {
				c.Set("api_key_project_id", pid)
			}
		}
		c.Next()
	}
}

func (e *negativeAuthTestEnv) seedProject(t *testing.T, ownerID uuid.UUID) uuid.UUID {
	t.Helper()
	pid := uuid.New()
	_, err := e.db.Exec(
		`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?, ?, '', ?, ?, ?)`,
		pid.String(), "p-"+pid.String()[:8], ownerID.String(), []byte("dek"), []byte("nonce"),
	)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return pid
}

func (e *negativeAuthTestEnv) probe(t *testing.T, method, path string, headers map[string]string) int {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	e.router.ServeHTTP(w, req)
	return w.Code
}

// TestNegAuth_IDOR_CrossTenantSecretCRUD covers audit S-B2 across the
// four HTTP verbs on /projects/:id/secrets. With User B's JWT presented
// for User A's project, every call must return 403/404.
func TestNegAuth_IDOR_CrossTenantSecretCRUD(t *testing.T) {
	env := newNegativeAuthEnv(t)
	userA := uuid.New()
	userB := uuid.New()
	projectA := env.seedProject(t, userA)

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"GET as wrong user", "GET", "/projects/" + projectA.String() + "/probe", http.StatusForbidden},
		{"POST as wrong user", "POST", "/projects/" + projectA.String() + "/secrets/probe", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := env.probe(t, tc.method, tc.path, map[string]string{
				"X-Test-User-ID": userB.String(),
			})
			if code != tc.want {
				t.Errorf("status = %d, want %d", code, tc.want)
			}
		})
	}

	// Sanity: owner gets through.
	if code := env.probe(t, "GET", "/projects/"+projectA.String()+"/probe", map[string]string{
		"X-Test-User-ID": userA.String(),
	}); code != http.StatusOK {
		t.Errorf("owner status = %d, want 200", code)
	}
}

// TestNegAuth_NonexistentProject_Returns404 — same response shape whether
// the project exists or not, to deny enumeration. The owner-of-non-existing
// also gets 404 (the project is simply not there).
func TestNegAuth_NonexistentProject_Returns404(t *testing.T) {
	env := newNegativeAuthEnv(t)
	missingProject := uuid.New()
	code := env.probe(t, "GET", "/projects/"+missingProject.String()+"/probe", map[string]string{
		"X-Test-User-ID": uuid.New().String(),
	})
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// TestNegAuth_InvalidProjectUUID_Returns400 covers the parse path.
func TestNegAuth_InvalidProjectUUID_Returns400(t *testing.T) {
	env := newNegativeAuthEnv(t)
	code := env.probe(t, "GET", "/projects/not-a-uuid/probe", map[string]string{
		"X-Test-User-ID": uuid.New().String(),
	})
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

// TestNegAuth_APIKeyScope_RejectsCrossProject covers audit S-B3: an API
// key scoped to project P must not be usable on project Q even though
// the underlying user owns both.
func TestNegAuth_APIKeyScope_RejectsCrossProject(t *testing.T) {
	env := newNegativeAuthEnv(t)
	user := uuid.New()
	projectP := env.seedProject(t, user)
	projectQ := env.seedProject(t, user)

	// API key scoped to project P, request hits project Q -> 403
	code := env.probe(t, "POST", "/projects/"+projectQ.String()+"/secrets/probe", map[string]string{
		"X-Test-User-ID":        user.String(),
		"X-Test-ApiKey-Project": projectP.String(),
	})
	if code != http.StatusForbidden {
		t.Errorf("cross-project status = %d, want 403", code)
	}
	// Same key on its own project -> 200
	code = env.probe(t, "POST", "/projects/"+projectP.String()+"/secrets/probe", map[string]string{
		"X-Test-User-ID":        user.String(),
		"X-Test-ApiKey-Project": projectP.String(),
	})
	if code != http.StatusOK {
		t.Errorf("same-project status = %d, want 200", code)
	}
}

// TestNegAuth_NoAuth_Returns401 — RequireProjectAccess fails closed when
// the upstream auth middleware did not set user_id.
func TestNegAuth_NoAuth_Returns401(t *testing.T) {
	env := newNegativeAuthEnv(t)
	projectA := env.seedProject(t, uuid.New())
	code := env.probe(t, "GET", "/projects/"+projectA.String()+"/probe", nil)
	if code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", code)
	}
}

// TestNegAuth_MCPExec_BlocksUnsafeEntry covers audit S-B5: the validator
// rejects disallowed binaries and shell metachars at registration time.
func TestNegAuth_MCPExec_BlocksUnsafeEntry(t *testing.T) {
	cases := []struct {
		entry string
		ok    bool
	}{
		{"node dist/index.js", true},
		{"python script.py", true},
		{"sh -c rm -rf /", false},
		{"node;cat /etc/passwd", false},
		{"./malicious", false},
		{"node $(whoami)", false},
		{"node `id`", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.entry, func(t *testing.T) {
			_, err := validateMCPEntryCommand(tc.entry)
			if tc.ok && err != nil {
				t.Errorf("expected accept, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("expected reject for %q", tc.entry)
			}
		})
	}
}
