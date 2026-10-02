package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/migrations"
)

func TestIdentityHTTPRejectsForeignSessionsAndClaimedAdminEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, d, err := repository.NewDB("sqlite://" + filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = repository.RunMigrationsFS(db, d, migrations.FS); err != nil {
		t.Fatal(err)
	}
	audit := repository.NewAuditRepository(db, d)
	audit.SetChainKey([]byte("synthetic-identity-http-audit"))
	jwt := auth.NewJWTService("synthetic-identity-http-signing")
	sessions := service.NewSessionService(db, d, jwt, audit)
	jwt.EnableHumanSessions(sessions)
	svc := service.NewAuthService(repository.NewUserRepository(db, d), nil, audit, jwt)
	svc.EnableSessions(sessions)
	owner, err := svc.Register("claimed-operator@example.invalid", "Synthetic-Only1!")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := svc.Register("other@example.invalid", "Synthetic-Only1!")
	if err != nil {
		t.Fatal(err)
	}
	fc, err := jwt.ValidateToken(foreign.Token)
	if err != nil {
		t.Fatal(err)
	}
	checker := repository.NewPlatformAdminRepository(db, d, audit)
	t.Setenv("KEEPSAVE_PLATFORM_ADMIN_EMAILS", owner.User.Email)
	router := gin.New()
	handler := NewSessionHandler(sessions)
	router.GET("/account/sessions", JWTAuthMiddleware(jwt), handler.List)
	router.DELETE("/account/sessions/:sessionId", JWTAuthMiddleware(jwt), handler.Revoke)
	router.POST("/auth/logout", JWTAuthMiddleware(jwt), handler.Logout)
	router.GET("/operator", JWTAuthMiddleware(jwt), RequireOperatorAdmin(checker), func(c *gin.Context) { c.Status(200) })
	call := func(method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if rec := call("GET", "/operator", owner.Token); rec.Code != 403 {
		t.Fatal("self-claimed email obtained operator authority", rec.Code)
	}
	if err = checker.SetGrant(context.Background(), owner.User.ID, true, "synthetic-operator", "test grant"); err != nil {
		t.Fatal(err)
	}
	if rec := call("GET", "/operator", owner.Token); rec.Code != 200 {
		t.Fatal("stored grant denied", rec.Code)
	}
	if rec := call("GET", "/account/sessions", owner.Token); rec.Code != 200 || strings.Contains(rec.Body.String(), "token_hash") || strings.Contains(rec.Body.String(), owner.Token) || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe session listing", rec.Code)
	}
	if rec := call("DELETE", "/account/sessions/"+fc.SessionID, owner.Token); rec.Code != 404 {
		t.Fatal("foreign revoke was allowed", rec.Code)
	}
	if rec := call("POST", "/auth/logout", owner.Token); rec.Code != 204 {
		t.Fatal("logout", rec.Code)
	}
	if rec := call("GET", "/account/sessions", owner.Token); rec.Code != 401 {
		t.Fatal("revoked session reused", rec.Code)
	}
	if rec := call("GET", "/account/sessions", foreign.Token); rec.Code != 200 {
		t.Fatal("revoked foreign owner", rec.Code)
	}
	db.Close()
	if rec := call("GET", "/account/sessions", foreign.Token); rec.Code != http.StatusServiceUnavailable {
		t.Fatal("missing authoritative database did not fail closed", rec.Code)
	}
}
