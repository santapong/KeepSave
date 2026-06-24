package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequirePlatformAdmin covers DB-06: only allowlisted emails reach /admin,
// the match is case-insensitive, a missing email claim is 401, and an empty
// allowlist denies everyone (fail-closed).
func TestRequirePlatformAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(allow []string, email string) int {
		r := gin.New()
		r.GET("/admin/x",
			func(c *gin.Context) {
				if email != "" {
					c.Set("email", email)
				}
				c.Next()
			},
			RequirePlatformAdmin(allow),
			func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
		)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin/x", nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	cases := []struct {
		name  string
		allow []string
		email string
		want  int
	}{
		{"allowed", []string{"admin@x.com"}, "admin@x.com", http.StatusOK},
		{"case-insensitive", []string{"admin@x.com"}, "Admin@X.com", http.StatusOK},
		{"not allowed", []string{"admin@x.com"}, "intruder@x.com", http.StatusForbidden},
		{"no email claim", []string{"admin@x.com"}, "", http.StatusUnauthorized},
		{"empty allowlist denies all", nil, "anyone@x.com", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := run(tc.allow, tc.email); code != tc.want {
				t.Errorf("status = %d, want %d", code, tc.want)
			}
		})
	}
}
