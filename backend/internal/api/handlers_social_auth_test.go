package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSocialRoutesFailClosedWithoutProviderConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jwtService := auth.NewJWTService("test-secret")
	h := NewSocialAuthHandler(service.NewSocialAuthService(config.SocialAuth{}, nil, nil, jwtService))
	router := gin.New()
	router.GET("/api/v1/auth/providers", h.Providers)
	router.POST("/api/v1/auth/social/:provider/start", h.Start(false))
	router.POST("/api/v1/account/connections/:provider/start", JWTAuthMiddleware(jwtService), h.Start(true))
	user := uuid.New()
	human, _ := jwtService.GenerateToken(user, "test@example.com")
	agent, _, _, err := jwtService.GenerateAgentToken(user, "test@example.com", uuid.New(), uuid.New(), "alpha", []string{"TEST"}, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"metadata", "GET", "/api/v1/auth/providers", "", "", 200},
		{"unconfigured", "POST", "/api/v1/auth/social/github/start", "", `{"code_challenge":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, 503},
		{"unknown provider", "POST", "/api/v1/auth/social/unknown/start", "", `{"code_challenge":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, 503},
		{"malformed", "POST", "/api/v1/auth/social/google/start", "", `{}`, 400},
		{"link anonymous", "POST", "/api/v1/account/connections/google/start", "", `{}`, 401},
		{"link agent denied", "POST", "/api/v1/account/connections/google/start", agent, `{}`, 403},
		{"link human reaches config gate", "POST", "/api/v1/account/connections/google/start", human, `{"code_challenge":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if rec.Code == http.StatusOK && rec.Body.String() != `{"github":false,"google":false}` {
				t.Fatal("metadata exposes more than availability")
			}
			if strings.Contains(rec.Body.String(), "test-secret") {
				t.Fatal("credential leaked")
			}
		})
	}
}
