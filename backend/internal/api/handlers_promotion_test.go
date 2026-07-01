package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// promotionTestEnv stands up an in-memory sqlite DB with a real
// PromotionService + ProjectRepository so the /projects/:id promotion routes
// run through the production middleware chain (stubAuth -> RequireProjectAccess
// -> handler). secretRepo/envRepo/cryptoSvc are nil: the paths under test
// (reads and the cross-project deny) never reach them.
type promotionTestEnv struct {
	db          *sql.DB
	projectRepo *repository.ProjectRepository
	router      *gin.Engine
}

func newPromotionTestEnv(t *testing.T) *promotionTestEnv {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
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
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,
		`CREATE TABLE organization_members (
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL
		)`,
		`CREATE TABLE promotion_requests (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			source_environment TEXT NOT NULL,
			target_environment TEXT NOT NULL,
			status TEXT NOT NULL,
			requested_by TEXT NOT NULL,
			approved_by TEXT,
			keys_filter TEXT NOT NULL DEFAULT '[]',
			override_policy TEXT NOT NULL DEFAULT 'skip',
			notes TEXT,
			created_at TIMESTAMP,
			completed_at TIMESTAMP
		)`,
		`CREATE TABLE audit_log (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			project_id TEXT,
			action TEXT NOT NULL,
			environment TEXT,
			details TEXT,
			ip_address TEXT,
			created_at TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	projectRepo := repository.NewProjectRepository(db, dialect)
	promotionRepo := repository.NewPromotionRepository(db, dialect)
	auditRepo := repository.NewAuditRepository(db, dialect)
	svc := service.NewPromotionService(promotionRepo, nil, projectRepo, nil, auditRepo, nil)
	h := NewPromotionHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/projects/:id")
	g.Use(stubAuth(), RequireProjectAccess(projectRepo))
	{
		g.GET("/promotions/:promotionId", h.GetPromotion)
		g.POST("/promotions/:promotionId/approve", h.ApprovePromotion)
		g.POST("/promotions/:promotionId/reject", h.RejectPromotion)
		g.POST("/promotions/:promotionId/rollback", h.Rollback)
		g.GET("/audit-log", h.AuditLog)
	}
	return &promotionTestEnv{db: db, projectRepo: projectRepo, router: r}
}

func (e *promotionTestEnv) seedProject(t *testing.T, owner uuid.UUID) uuid.UUID {
	t.Helper()
	pid := uuid.New()
	if _, err := e.db.Exec(
		`INSERT INTO projects (id, name, description, owner_id) VALUES (?, ?, '', ?)`,
		pid.String(), "p-"+pid.String()[:8], owner.String(),
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return pid
}

func (e *promotionTestEnv) seedPromotion(t *testing.T, projectID, requestedBy uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.db.Exec(
		`INSERT INTO promotion_requests
		 (id, project_id, source_environment, target_environment, status, requested_by, keys_filter, override_policy, notes, created_at)
		 VALUES (?, ?, 'alpha', 'uat', 'pending', ?, '[]', 'skip', '', CURRENT_TIMESTAMP)`,
		id.String(), projectID.String(), requestedBy.String(),
	); err != nil {
		t.Fatalf("seed promotion: %v", err)
	}
	return id
}

func (e *promotionTestEnv) do(t *testing.T, method, path string, user uuid.UUID) (int, []byte) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	req.Header.Set("X-Test-User-ID", user.String())
	e.router.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// TestPromotion_RejectsCrossProjectActions verifies P-09: a caller who can
// access project P must not be able to act on a promotion belonging to
// project Q via /projects/P/promotions/<Qprom>. Every verb must 404
// (anti-enumeration), while the legitimate same-project read still succeeds.
func TestPromotion_RejectsCrossProjectActions(t *testing.T) {
	env := newPromotionTestEnv(t)
	owner := uuid.New() // owns BOTH projects, so RequireProjectAccess passes
	projectP := env.seedProject(t, owner)
	projectQ := env.seedProject(t, owner)
	promQ := env.seedPromotion(t, projectQ, owner)

	cross := []struct {
		name   string
		method string
		suffix string
	}{
		{"get", "GET", ""},
		{"approve", "POST", "/approve"},
		{"reject", "POST", "/reject"},
		{"rollback", "POST", "/rollback"},
	}
	for _, tc := range cross {
		t.Run(tc.name, func(t *testing.T) {
			path := "/projects/" + projectP.String() + "/promotions/" + promQ.String() + tc.suffix
			if code, body := env.do(t, tc.method, path, owner); code != http.StatusNotFound {
				t.Errorf("cross-project %s = %d, want 404 (body=%s)", tc.name, code, body)
			}
		})
	}

	// Same-project read reaches the handler and returns the promotion: proves
	// the gate is project-scoping, not a blanket deny.
	if code, body := env.do(t, "GET", "/projects/"+projectQ.String()+"/promotions/"+promQ.String(), owner); code != http.StatusOK {
		t.Errorf("same-project get = %d, want 200 (body=%s)", code, body)
	}
}

// TestPromotion_AuditLogCapsLimit verifies AUTH-10: the audit-log limit query
// param is capped (500) regardless of how large a value the caller requests.
func TestPromotion_AuditLogCapsLimit(t *testing.T) {
	env := newPromotionTestEnv(t)
	owner := uuid.New()
	project := env.seedProject(t, owner)
	for i := 0; i < 600; i++ {
		if _, err := env.db.Exec(
			`INSERT INTO audit_log (id, user_id, project_id, action, environment, details, ip_address, created_at)
			 VALUES (?, ?, ?, 'secret.created', 'alpha', '{}', '127.0.0.1', CURRENT_TIMESTAMP)`,
			uuid.New().String(), owner.String(), project.String(),
		); err != nil {
			t.Fatalf("seed audit row %d: %v", i, err)
		}
	}

	count := func(query string) int {
		code, body := env.do(t, "GET", "/projects/"+project.String()+"/audit-log"+query, owner)
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", code, body)
		}
		var resp struct {
			AuditLog []json.RawMessage `json:"audit_log"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return len(resp.AuditLog)
	}

	if n := count("?limit=100000"); n != 500 {
		t.Errorf("limit=100000 returned %d rows, want cap 500", n)
	}
	if n := count("?limit=10"); n != 10 {
		t.Errorf("limit=10 returned %d rows, want 10", n)
	}
}

// TestNegAuth_SelfApproval_Returns403 covers NEGATIVE_AUTH_PLAN A10 (the
// four-eyes invariant, ADR-0003): the user who REQUESTED a promotion cannot
// approve it themselves. The service returns ErrSelfApproval; the handler must
// surface that as 403 FORBIDDEN (an authorization failure), not 400.
func TestNegAuth_SelfApproval_Returns403(t *testing.T) {
	env := newPromotionTestEnv(t)
	owner := uuid.New()
	project := env.seedProject(t, owner)
	// The promotion is requested BY the owner; owner then tries to approve it.
	prom := env.seedPromotion(t, project, owner)

	code, body := env.do(t, "POST",
		"/projects/"+project.String()+"/promotions/"+prom.String()+"/approve", owner)
	if code != http.StatusForbidden {
		t.Errorf("self-approval status = %d, want 403 (body=%s)", code, body)
	}
}
