// Tests for the ADR-0006 embed-config endpoint.
//
// Covers the audit-driven requirements:
//   - allow-listed project returns its config (200)
//   - missing project AND embed_policy_enabled=false BOTH return 404
//     with the SAME shape (enumeration mitigation)
//   - rate-limit middleware returns 429 once the per-IP budget is exhausted
//   - wildcard sentinel ["*"] on the authenticated update endpoint returns 422
//
// The tests use a SQLite in-memory database with a minimal `projects` schema —
// the goal is to exercise the handler + service + repository wiring end-to-end
// without spinning up a full server.
package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

func newEmbedTestDB(t *testing.T) (*sql.DB, repository.Dialect) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Minimal projects schema. Mirrors migration 007 for SQLite.
	if _, err := db.Exec(`CREATE TABLE projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		owner_id TEXT NOT NULL,
		encrypted_dek BLOB NOT NULL,
		dek_nonce BLOB NOT NULL,
		allowed_origins TEXT NOT NULL DEFAULT '[]',
		embed_policy_enabled INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create projects table: %v", err)
	}
	return db, repository.NewDialect(repository.DBTypeSQLite)
}

func insertTestProject(t *testing.T, db *sql.DB, id uuid.UUID, allowedOrigins string, embedEnabled bool) {
	t.Helper()
	embedFlag := 0
	if embedEnabled {
		embedFlag = 1
	}
	_, err := db.Exec(
		`INSERT INTO projects (id, name, owner_id, encrypted_dek, dek_nonce, allowed_origins, embed_policy_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), "test-project", uuid.New().String(), []byte("dek"), []byte("nonce"), allowedOrigins, embedFlag,
	)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
}

func newEmbedTestHandler(t *testing.T, db *sql.DB, dialect repository.Dialect) *EmbedHandler {
	t.Helper()
	projectRepo := repository.NewProjectRepository(db, dialect)
	// envRepo and cryptoSvc are only used by Create/Delete paths; the
	// embed-config endpoint never touches them, so nil is acceptable here.
	svc := service.NewProjectService(projectRepo, nil, nil)
	return NewEmbedHandler(svc)
}

// embedTestRouter mirrors the production wiring for the embed-config endpoint
// (route + per-IP rate limiter). burstAndRate controls the limiter; tests use
// a small budget for the 429 case.
func embedTestRouter(h *EmbedHandler, rate int, interval time.Duration, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	limiter := NewRateLimiter(rate, interval, burst)
	r.GET("/api/v1/embed-config/:project_id",
		RateLimitMiddleware(limiter),
		h.GetEmbedConfig,
	)
	return r
}

func TestEmbedConfig_AllowListedProject_Returns200(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	id := uuid.New()
	insertTestProject(t, db, id, `["https://example.com","https://app.example.com"]`, true)

	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 100, time.Minute, 100)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/embed-config/"+id.String(), nil)
	req.RemoteAddr = "1.2.3.4:1000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		ProjectID          string   `json:"project_id"`
		AllowedOrigins     []string `json:"allowed_origins"`
		EmbedPolicyEnabled bool     `json:"embed_policy_enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, w.Body.String())
	}
	if body.ProjectID != id.String() {
		t.Errorf("project_id = %q, want %q", body.ProjectID, id.String())
	}
	if !body.EmbedPolicyEnabled {
		t.Errorf("embed_policy_enabled = false, want true")
	}
	if len(body.AllowedOrigins) != 2 || body.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("allowed_origins = %v, want [https://example.com, https://app.example.com]", body.AllowedOrigins)
	}
}

func TestEmbedConfig_MissingProject_Returns404(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 100, time.Minute, 100)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/embed-config/"+uuid.New().String(), nil)
	req.RemoteAddr = "1.2.3.4:1000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestEmbedConfig_PolicyDisabled_Returns404_SameShape is the enumeration
// mitigation: an attacker who can probe project IDs must not be able to
// distinguish "project exists but widget disabled" from "no such project".
func TestEmbedConfig_PolicyDisabled_Returns404_SameShape(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	id := uuid.New()
	insertTestProject(t, db, id, `[]`, false) // exists but policy disabled
	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 100, time.Minute, 100)

	// Existing-but-disabled
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/v1/embed-config/"+id.String(), nil)
	req1.RemoteAddr = "1.2.3.4:1000"
	r.ServeHTTP(w1, req1)

	// Non-existent
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/embed-config/"+uuid.New().String(), nil)
	req2.RemoteAddr = "1.2.3.4:1000"
	r.ServeHTTP(w2, req2)

	if w1.Code != http.StatusNotFound || w2.Code != http.StatusNotFound {
		t.Fatalf("both should be 404; disabled=%d missing=%d", w1.Code, w2.Code)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Errorf("response shapes differ — enumeration risk!\n  disabled: %s\n  missing:  %s",
			w1.Body.String(), w2.Body.String())
	}
}

func TestEmbedConfig_RateLimit_ReturnsTooManyRequests(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	id := uuid.New()
	insertTestProject(t, db, id, `["https://example.com"]`, true)

	// Burst of 2 to keep the test fast. The production setting is 10/min
	// per ADR-0006; the rate-limit logic is shared, so a smaller budget
	// exercises the same code path.
	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 10, time.Minute, 2)

	hit := func() int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/embed-config/"+id.String(), nil)
		req.RemoteAddr = "5.6.7.8:1000"
		r.ServeHTTP(w, req)
		return w.Code
	}

	if got := hit(); got != http.StatusOK {
		t.Fatalf("req 1: got %d, want 200", got)
	}
	if got := hit(); got != http.StatusOK {
		t.Fatalf("req 2: got %d, want 200", got)
	}
	if got := hit(); got != http.StatusTooManyRequests {
		t.Fatalf("req 3 (over budget): got %d, want 429", got)
	}
}

func TestEmbedConfig_InvalidUUID_Returns400(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 100, time.Minute, 100)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/embed-config/not-a-uuid", nil)
	req.RemoteAddr = "1.2.3.4:1000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestEmbedConfig_WildcardSentinel_Rejected verifies the authenticated update
// endpoint refuses ["*"] with 422 per ADR-0006 §Open-Q #2 (footgun protection).
// Operators must use embed_policy_enabled=false to disable the widget.
func TestEmbedConfig_WildcardSentinel_Rejected(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	id := uuid.New()
	ownerID := uuid.New()
	// Re-insert under a known owner.
	_, err := db.Exec(
		`INSERT INTO projects (id, name, owner_id, encrypted_dek, dek_nonce, allowed_origins, embed_policy_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id.String(), "p", ownerID.String(), []byte("dek"), []byte("nonce"), "[]", 0,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	h := newEmbedTestHandler(t, db, dialect)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Stub the auth middleware: set user_id directly.
	r.PUT("/api/v1/projects/:id/embed-config",
		func(c *gin.Context) { c.Set("user_id", ownerID); c.Next() },
		h.UpdateEmbedConfig,
	)

	body, _ := json.Marshal(UpdateEmbedConfigRequest{
		AllowedOrigins:     []string{"*"},
		EmbedPolicyEnabled: true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/projects/"+id.String()+"/embed-config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "wildcard_origin_not_permitted" {
		t.Errorf("error code = %q, want wildcard_origin_not_permitted", resp["error"])
	}
}

// TestEmbedConfig_WildcardSentinel_StrippedOnRead is defence-in-depth: if a
// historical row already contains "*" (e.g. seeded before the validator
// existed), the service-layer scrub must drop it before returning.
func TestEmbedConfig_WildcardSentinel_StrippedOnRead(t *testing.T) {
	db, dialect := newEmbedTestDB(t)
	id := uuid.New()
	insertTestProject(t, db, id, `["*","https://example.com"]`, true)
	r := embedTestRouter(newEmbedTestHandler(t, db, dialect), 100, time.Minute, 100)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/embed-config/"+id.String(), nil)
	req.RemoteAddr = "9.9.9.9:1000"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, o := range body.AllowedOrigins {
		if o == "*" {
			t.Fatalf("wildcard leaked into response: %v", body.AllowedOrigins)
		}
	}
	if len(body.AllowedOrigins) != 1 || body.AllowedOrigins[0] != "https://example.com" {
		t.Errorf("allowed_origins = %v, want [https://example.com]", body.AllowedOrigins)
	}
}

