package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

// feedbackTestEnv wires the real JWTAuthMiddleware + FeedbackHandler over an
// in-memory SQLite audit repo, so tests exercise the production auth chain.
type feedbackTestEnv struct {
	router *gin.Engine
	jwt    *auth.JWTService
	db     *sql.DB
}

func newFeedbackTestEnv(t *testing.T, token, githubURL string) *feedbackTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		project_id TEXT,
		action TEXT,
		environment TEXT,
		details TEXT,
		ip_address TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	dialect := repository.NewDialect(repository.DBTypeSQLite)
	auditRepo := repository.NewAuditRepository(db, dialect)

	svc := service.NewFeedbackService(token, "santapong/KeepSave", auditRepo)
	if githubURL != "" {
		svc.SetAPIBase(githubURL)
	}
	h := NewFeedbackHandler(svc)

	jwtSvc := auth.NewJWTService("test-secret")
	r := gin.New()
	fb := r.Group("/api/v1/feedback")
	fb.Use(JWTAuthMiddleware(jwtSvc))
	fb.POST("", h.Submit)

	return &feedbackTestEnv{router: r, jwt: jwtSvc, db: db}
}

func (e *feedbackTestEnv) do(t *testing.T, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	e.router.ServeHTTP(w, req)
	return w
}

func (e *feedbackTestEnv) token(t *testing.T) string {
	t.Helper()
	tok, err := e.jwt.GenerateToken(uuid.New(), "user@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return tok
}

func TestFeedbackHandler_Unauthenticated(t *testing.T) {
	env := newFeedbackTestEnv(t, "tok", "")
	w := env.do(t, "", `{"category":"bug","message":"hi"}`)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestFeedbackHandler_Unconfigured(t *testing.T) {
	env := newFeedbackTestEnv(t, "", "") // empty token => disabled
	w := env.do(t, env.token(t), `{"category":"bug","message":"hi"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (body=%s)", w.Code, w.Body.String())
	}
}

func TestFeedbackHandler_ValidationErrors(t *testing.T) {
	env := newFeedbackTestEnv(t, "tok", "")
	tok := env.token(t)
	cases := []struct {
		name string
		body string
	}{
		{"bad category", `{"category":"spam","message":"hi"}`},
		{"empty message", `{"category":"bug","message":""}`},
		{"whitespace message", `{"category":"bug","message":"   "}`},
		{"oversize message", `{"category":"bug","message":"` + strings.Repeat("x", 4001) + `"}`},
		{"missing category", `{"message":"hi"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := env.do(t, tok, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (body=%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestFeedbackHandler_HappyPath(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"html_url":"https://github.com/santapong/KeepSave/issues/7","number":7}`)
	}))
	t.Cleanup(gh.Close)

	env := newFeedbackTestEnv(t, "tok", gh.URL)
	w := env.do(t, env.token(t), `{"category":"idea","message":"add dark mode","page_url":"https://app/x"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", w.Code, w.Body.String())
	}
	var resp struct {
		IssueURL    string `json:"issue_url"`
		IssueNumber int    `json:"issue_number"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.IssueURL != "https://github.com/santapong/KeepSave/issues/7" || resp.IssueNumber != 7 {
		t.Errorf("resp = %+v", resp)
	}

	// Audit row written for the state-mutating handler.
	var count int
	if err := env.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = ?`, "feedback.submitted").Scan(&count); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if count != 1 {
		t.Errorf("audit rows = %d, want 1", count)
	}
}

// TestFeedbackHandler_UpstreamErrorNotLeaked verifies a GitHub-side failure
// yields a 502 with a generic message and never echoes the upstream body.
func TestFeedbackHandler_UpstreamErrorNotLeaked(t *testing.T) {
	const secretUpstream = "TOP_SECRET_GITHUB_INTERNAL_DETAIL"
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"message":"`+secretUpstream+`"}`)
	}))
	t.Cleanup(gh.Close)

	env := newFeedbackTestEnv(t, "tok", gh.URL)
	w := env.do(t, env.token(t), `{"category":"bug","message":"broken"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (body=%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), secretUpstream) {
		t.Errorf("response leaked upstream GitHub detail: %s", w.Body.String())
	}
}
