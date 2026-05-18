package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestCORSMiddleware_AllowListAndGlobs covers the three accepted formats:
// wildcard (dev only), exact list, and glob pattern for Vercel preview
// subdomains. Per audit F-H3.
func TestCORSMiddleware_AllowListAndGlobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		cors        string
		reqOrigin   string
		wantHeader  string // expected ACAO; "" means not present
		wantPreflight int
	}{
		{
			name:          "exact match",
			cors:          "https://app.example.com",
			reqOrigin:     "https://app.example.com",
			wantHeader:    "https://app.example.com",
			wantPreflight: http.StatusNoContent,
		},
		{
			name:          "exact list miss",
			cors:          "https://app.example.com,https://x.io",
			reqOrigin:     "https://evil.example.com",
			wantHeader:    "",
			wantPreflight: http.StatusNoContent,
		},
		{
			name:          "vercel preview glob",
			cors:          "https://keepsave-*-team.vercel.app",
			reqOrigin:     "https://keepsave-uat-abc123-team.vercel.app",
			wantHeader:    "https://keepsave-uat-abc123-team.vercel.app",
			wantPreflight: http.StatusNoContent,
		},
		{
			name:          "vercel glob no match",
			cors:          "https://keepsave-*-team.vercel.app",
			reqOrigin:     "https://other.vercel.app",
			wantHeader:    "",
			wantPreflight: http.StatusNoContent,
		},
		{
			name:          "wildcard dev mode mirrors origin",
			cors:          "*",
			reqOrigin:     "https://anything.local",
			wantHeader:    "https://anything.local",
			wantPreflight: http.StatusNoContent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(CORSMiddleware(tc.cors))
			r.GET("/x", func(c *gin.Context) { c.JSON(200, gin.H{}) })

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("OPTIONS", "/x", nil)
			req.Header.Set("Origin", tc.reqOrigin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			r.ServeHTTP(w, req)

			if w.Code != tc.wantPreflight {
				t.Errorf("preflight status = %d, want %d", w.Code, tc.wantPreflight)
			}
			got := w.Header().Get("Access-Control-Allow-Origin")
			if got != tc.wantHeader {
				t.Errorf("ACAO = %q, want %q", got, tc.wantHeader)
			}
		})
	}
}

// TestCORSMiddleware_NoCredentialsHeader documents that we never set
// Access-Control-Allow-Credentials (KeepSave is bearer-token only).
func TestCORSMiddleware_NoCredentialsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware("https://app.example.com"))
	r.GET("/x", func(c *gin.Context) { c.JSON(200, gin.H{}) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("ACAC must not be set, got %q", got)
	}
}
