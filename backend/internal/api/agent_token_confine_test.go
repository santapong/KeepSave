package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/models"
)

// TestAgentTokenRouteAllowed locks in the allowlist: only GET on the two
// secret-read routes is reachable by an agent token (ADR-0021). Everything
// else — every write method, and every other route — is denied.
func TestAgentTokenRouteAllowed(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/api/v1/projects/:id/secrets", true},
		{"GET", "/api/v1/projects/:id/secrets/:secretId", true},
		{"POST", "/api/v1/projects/:id/secrets", false},
		{"PUT", "/api/v1/projects/:id/secrets/:secretId", false},
		{"DELETE", "/api/v1/projects/:id/secrets/:secretId", false},
		{"GET", "/api/v1/projects/:id/secrets/:secretId/versions", false},
		{"GET", "/api/v1/projects/:id/promotions", false},
		{"POST", "/api/v1/projects/:id/promote", false},
		{"POST", "/api/v1/projects/:id/leases", false},
		{"POST", "/api/v1/projects/:id/agent-token", false},
		{"GET", "/api/v1/organizations/:orgId/quota", false},
		{"GET", "/api/v1/applications/:appId", false},
	}
	for _, tc := range cases {
		if got := agentTokenRouteAllowed(tc.method, tc.path); got != tc.want {
			t.Errorf("agentTokenRouteAllowed(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

// TestAgentReadScopes maps lease keys to ADR-0022 read scopes (read:<key>) and
// nothing else — so the coarse action gate forbids writes and the per-key gate
// forbids non-leased keys.
func TestAgentReadScopes(t *testing.T) {
	scopes := agentReadScopes([]string{"DB_URL", "API_KEY"})
	if len(scopes) != 2 || scopes[0] != "read:DB_URL" || scopes[1] != "read:API_KEY" {
		t.Fatalf("agentReadScopes = %v, want [read:DB_URL read:API_KEY]", scopes)
	}
	// No write/delete on any key.
	if apiKeyHasScope(scopes, "write") || apiKeyHasScope(scopes, "delete") {
		t.Error("agent read scopes must not grant write/delete")
	}
	// Read allowed only on the leased keys.
	if !apiKeyScopeAllowsKey(scopes, "read", "DB_URL") {
		t.Error("DB_URL read should be allowed")
	}
	if apiKeyScopeAllowsKey(scopes, "read", "OTHER_KEY") {
		t.Error("non-leased key read must be denied")
	}
}

// TestAgentToken_EndToEndConfinement mints a real agent token and drives it
// through the exact middleware chain the secret routes use
// (JWTAuthMiddleware → RequireProjectAccess → EnforceAPIKeyScope). It proves the
// privilege-escalation is closed: the token reads only its leased project +
// environment, and is rejected everywhere else — instead of inheriting the
// owning user's full access.
func TestAgentToken_EndToEndConfinement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newNegativeAuthEnv(t)
	svc := auth.NewJWTService("test-secret")

	owner := uuid.New()
	projectP := env.seedProject(t, owner) // the lease's project, owned by `owner`
	projectQ := env.seedProject(t, owner) // another project the SAME user owns

	// Mint an agent token scoped to project P / env alpha / key DB_URL only.
	agentTok, _, _, err := svc.GenerateAgentToken(owner, "", uuid.New(), projectP, "alpha", []string{"DB_URL"}, 5*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}
	// A plain user token for the same owner — must remain unconfined.
	userTok, err := svc.GenerateToken(owner, "owner@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	r := gin.New()
	v1 := r.Group("/api/v1")
	sec := v1.Group("/projects/:id/secrets")
	sec.Use(JWTAuthMiddleware(svc), RequireProjectAccess(env.projectRepo), EnforceAPIKeyScope())
	{
		sec.GET("", ok)
		sec.GET("/:secretId", ok)
		sec.POST("", ok)
	}
	pm := v1.Group("/projects/:id")
	pm.Use(JWTAuthMiddleware(svc), RequireProjectAccess(env.projectRepo))
	{
		pm.POST("/promote", ok)
	}
	ls := v1.Group("/projects/:id/leases")
	ls.Use(JWTAuthMiddleware(svc), RequireProjectAccess(env.projectRepo), EnforceAPIKeyScope())
	{
		ls.POST("", ok)
	}

	call := func(method, path, bearer string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		r.ServeHTTP(w, req)
		return w.Code
	}

	pP := projectP.String()
	pQ := projectQ.String()

	cases := []struct {
		name, method, path, tok string
		want                    int
	}{
		{"agent reads leased project+env", "GET", "/api/v1/projects/" + pP + "/secrets?environment=alpha", agentTok, http.StatusOK},
		{"agent blocked cross-environment", "GET", "/api/v1/projects/" + pP + "/secrets?environment=prod", agentTok, http.StatusForbidden},
		{"agent blocked cross-project (even same owner)", "GET", "/api/v1/projects/" + pQ + "/secrets?environment=alpha", agentTok, http.StatusForbidden},
		{"agent blocked from writing secrets", "POST", "/api/v1/projects/" + pP + "/secrets", agentTok, http.StatusForbidden},
		{"agent blocked from promotion", "POST", "/api/v1/projects/" + pP + "/promote", agentTok, http.StatusForbidden},
		{"agent blocked from minting leases", "POST", "/api/v1/projects/" + pP + "/leases", agentTok, http.StatusForbidden},
		// The owning USER token is unaffected by agent confinement.
		{"user token reads any env", "GET", "/api/v1/projects/" + pP + "/secrets?environment=prod", userTok, http.StatusOK},
		{"user token reaches own other project", "GET", "/api/v1/projects/" + pQ + "/secrets?environment=alpha", userTok, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := call(tc.method, tc.path, tc.tok); code != tc.want {
				t.Errorf("status = %d, want %d", code, tc.want)
			}
		})
	}
}

// TestAgentToken_SetsScopedContext verifies the agent-token branch of
// JWTAuthMiddleware installs the lease scope as API-key-equivalent context keys
// (project, read scopes, environment) and the is_agent_token marker — the basis
// for reusing the existing per-key enforcement.
func TestAgentToken_SetsScopedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := auth.NewJWTService("test-secret")
	projectP := uuid.New()
	tok, _, _, err := svc.GenerateAgentToken(uuid.New(), "", uuid.New(), projectP, "alpha", []string{"DB_URL"}, 5*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("GenerateAgentToken: %v", err)
	}

	r := gin.New()
	v1 := r.Group("/api/v1")
	sec := v1.Group("/projects/:id/secrets")
	sec.Use(JWTAuthMiddleware(svc))
	sec.GET("", func(c *gin.Context) {
		if !c.GetBool("is_agent_token") {
			t.Error("is_agent_token not set")
		}
		if pid, _ := c.Get("api_key_project_id"); pid != projectP {
			t.Errorf("api_key_project_id = %v, want %v", pid, projectP)
		}
		if env, _ := c.Get("api_key_environment"); env != "alpha" {
			t.Errorf("api_key_environment = %v, want alpha", env)
		}
		scopesV, _ := c.Get("api_key_scopes")
		scopes, _ := scopesV.(models.StringList)
		if len(scopes) != 1 || scopes[0] != "read:DB_URL" {
			t.Errorf("api_key_scopes = %v, want [read:DB_URL]", scopes)
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/projects/"+projectP.String()+"/secrets", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
