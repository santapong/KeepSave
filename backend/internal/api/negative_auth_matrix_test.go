package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// negAuthMatrixEnv exercises the REAL auth middleware chain
// (APIKeyAuthMiddleware → EnforceAPIKeyScope → RequireProjectAccess) with
// real JWTs and real API keys — NOT stubAuth. Any non-2xx traces to a genuine
// middleware decision, so the matrix asserts 401/403 for each attacker case
// from tests/NEGATIVE_AUTH_PLAN.md.
type negAuthMatrixEnv struct {
	db          *sql.DB
	jwt         *auth.JWTService
	apikeyRepo  *repository.APIKeyRepository
	projectRepo *repository.ProjectRepository
	router      *gin.Engine
	secret      string
}

func newNegAuthMatrixEnv(t *testing.T) *negAuthMatrixEnv {
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
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,
		`CREATE TABLE organization_members (
			organization_id TEXT NOT NULL,
			user_id TEXT NOT NULL
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
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	dialect := repository.NewDialect(repository.DBTypeSQLite)
	const secret = "test-hs256-secret-at-least-32-bytes-long!!"
	jwtSvc := auth.NewJWTService(secret) // HS256-only (no EnableRS256).
	apikeyRepo := repository.NewAPIKeyRepository(db, dialect)
	projectRepo := repository.NewProjectRepository(db, dialect)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// The full production chain. GET is a read (scope "read"), POST is a write
	// (scope "write"); RequireProjectAccess enforces project scoping last.
	g := r.Group("/api/v1/projects/:id",
		APIKeyAuthMiddleware(jwtSvc, apikeyRepo),
		EnforceAPIKeyScope(),
		RequireProjectAccess(projectRepo),
	)
	g.GET("/secrets", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	g.POST("/secrets", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	return &negAuthMatrixEnv{
		db: db, jwt: jwtSvc, apikeyRepo: apikeyRepo, projectRepo: projectRepo,
		router: r, secret: secret,
	}
}

func (e *negAuthMatrixEnv) seedProject(t *testing.T, owner uuid.UUID) uuid.UUID {
	t.Helper()
	pid := uuid.New()
	if _, err := e.db.Exec(
		`INSERT INTO projects (id, name, description, owner_id, encrypted_dek, dek_nonce) VALUES (?, ?, '', ?, ?, ?)`,
		pid.String(), "p-"+pid.String()[:8], owner.String(), []byte("dek"), []byte("nonce"),
	); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return pid
}

// validJWT mints a normal HS256 user token via the real service.
func (e *negAuthMatrixEnv) validJWT(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	tok, err := e.jwt.GenerateToken(userID, "user@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

// expiredJWT hand-builds an HS256 token with the correct secret but exp in the
// past, so it fails validation on expiry (not signature).
func (e *negAuthMatrixEnv) expiredJWT(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	claims := &auth.Claims{
		UserID: userID,
		Email:  "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(e.secret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	return signed
}

// wrongSecretJWT signs a well-formed, unexpired token with a DIFFERENT secret,
// so it fails signature verification.
func (e *negAuthMatrixEnv) wrongSecretJWT(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	claims := &auth.Claims{
		UserID: userID,
		Email:  "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("a-totally-different-secret-not-the-real-one!"))
	if err != nil {
		t.Fatalf("sign wrong-secret token: %v", err)
	}
	return signed
}

// seedAPIKey creates a real API key (returns the raw key for the X-API-Key
// header). env may be "" for an unbound key.
func (e *negAuthMatrixEnv) seedAPIKey(t *testing.T, userID, projectID uuid.UUID, env string, scopes []string) (rawKey string, keyID uuid.UUID) {
	t.Helper()
	raw, hashed, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate api key: %v", err)
	}
	var envPtr *string
	if env != "" {
		envPtr = &env
	}
	expires := time.Now().Add(24 * time.Hour)
	k, err := e.apikeyRepo.Create("test-key", hashed, userID, projectID, scopes, envPtr, &expires)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return raw, k.ID
}

func (e *negAuthMatrixEnv) req(method, path string, headers map[string]string) int {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest(method, path, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	e.router.ServeHTTP(w, r)
	return w.Code
}

// TestNegAuthMatrix_JWTCases covers the JWT attacker cases (A1–A5) against the
// real middleware chain.
func TestNegAuthMatrix_JWTCases(t *testing.T) {
	env := newNegAuthMatrixEnv(t)
	owner := uuid.New()
	stranger := uuid.New()
	pid := env.seedProject(t, owner)
	get := "/api/v1/projects/" + pid.String() + "/secrets"

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"A1 missing token", nil, http.StatusUnauthorized},
		{"A2 malformed bearer", map[string]string{"Authorization": "Bearer not.a.valid.jwt"}, http.StatusUnauthorized},
		{"A2 non-bearer scheme", map[string]string{"Authorization": "Basic Zm9vOmJhcg=="}, http.StatusUnauthorized},
		{"A3 expired jwt", map[string]string{"Authorization": "Bearer " + env.expiredJWT(t, owner)}, http.StatusUnauthorized},
		{"A4 wrong-secret jwt", map[string]string{"Authorization": "Bearer " + env.wrongSecretJWT(t, owner)}, http.StatusUnauthorized},
		{"A5 valid jwt, non-member", map[string]string{"Authorization": "Bearer " + env.validJWT(t, stranger)}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := env.req(http.MethodGet, get, tc.headers); code != tc.want {
				t.Errorf("status = %d, want %d", code, tc.want)
			}
		})
	}

	// Positive control: the owner's valid JWT reaches the handler.
	if code := env.req(http.MethodGet, get, map[string]string{
		"Authorization": "Bearer " + env.validJWT(t, owner),
	}); code != http.StatusOK {
		t.Errorf("owner valid jwt = %d, want 200", code)
	}
}

// TestNegAuthMatrix_APIKeyCases covers the API-key attacker cases (A6–A9)
// against the real middleware chain.
func TestNegAuthMatrix_APIKeyCases(t *testing.T) {
	env := newNegAuthMatrixEnv(t)
	owner := uuid.New()
	projP := env.seedProject(t, owner)
	projQ := env.seedProject(t, owner) // owner owns both, so only scope/env/revoke gate

	getP := "/api/v1/projects/" + projP.String() + "/secrets"
	getQ := "/api/v1/projects/" + projQ.String() + "/secrets"
	postP := getP

	// A6: key scoped to P used against Q → 403 (cross-project).
	rawP, _ := env.seedAPIKey(t, owner, projP, "", []string{"read", "write"})
	if code := env.req(http.MethodGet, getQ, map[string]string{"X-API-Key": rawP}); code != http.StatusForbidden {
		t.Errorf("A6 cross-project key = %d, want 403", code)
	}
	// Same key on its own project → 200 (positive control).
	if code := env.req(http.MethodGet, getP, map[string]string{"X-API-Key": rawP}); code != http.StatusOK {
		t.Errorf("A6 same-project key = %d, want 200", code)
	}

	// A7: env-locked (alpha) key targeting prod via ?environment=prod → 403.
	rawAlpha, _ := env.seedAPIKey(t, owner, projP, "alpha", []string{"read", "write"})
	if code := env.req(http.MethodGet, getP+"?environment=prod", map[string]string{"X-API-Key": rawAlpha}); code != http.StatusForbidden {
		t.Errorf("A7 env mismatch = %d, want 403", code)
	}
	// Same env-locked key targeting its own env → 200 (positive control).
	if code := env.req(http.MethodGet, getP+"?environment=alpha", map[string]string{"X-API-Key": rawAlpha}); code != http.StatusOK {
		t.Errorf("A7 env match = %d, want 200", code)
	}

	// A8: read-only key used on a write (POST) → 403 (scope insufficient).
	rawRead, _ := env.seedAPIKey(t, owner, projP, "", []string{"read"})
	if code := env.req(http.MethodPost, postP, map[string]string{"X-API-Key": rawRead}); code != http.StatusForbidden {
		t.Errorf("A8 scope mismatch = %d, want 403", code)
	}
	// Read on GET with the same key → 200 (positive control).
	if code := env.req(http.MethodGet, getP, map[string]string{"X-API-Key": rawRead}); code != http.StatusOK {
		t.Errorf("A8 read GET = %d, want 200", code)
	}

	// A9: revoked/deleted key → 401. Create, then delete the row, then use it.
	rawRevoked, revokedID := env.seedAPIKey(t, owner, projP, "", []string{"read"})
	if _, err := env.db.Exec(`DELETE FROM api_keys WHERE id = ?`, revokedID.String()); err != nil {
		t.Fatalf("revoke key: %v", err)
	}
	if code := env.req(http.MethodGet, getP, map[string]string{"X-API-Key": rawRevoked}); code != http.StatusUnauthorized {
		t.Errorf("A9 revoked key = %d, want 401", code)
	}

	// A9b: outright invalid key string → 401.
	if code := env.req(http.MethodGet, getP, map[string]string{"X-API-Key": "ks_deadbeef"}); code != http.StatusUnauthorized {
		t.Errorf("A9b invalid key = %d, want 401", code)
	}
}
