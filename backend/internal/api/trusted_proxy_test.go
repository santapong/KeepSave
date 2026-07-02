package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestTrustedProxy_ForgedXFFIgnoredWhenNoTrustedProxies proves the CWE-348 fix:
// with no trusted proxies configured (the default), a forged X-Forwarded-For /
// X-Real-IP header does NOT change the recorded client_ip — it stays the direct
// peer. This is the value both the rate limiter and the audit log consume.
func TestTrustedProxy_ForgedXFFIgnoredWhenNoTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil): %v", err)
	}
	r.Use(TrustedProxyMiddleware())
	r.GET("/ip", func(c *gin.Context) {
		v, _ := c.Get("client_ip")
		s, _ := v.(string)
		c.String(http.StatusOK, s)
	})

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.7:5555" // the real, direct peer
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Body.String(); got != "203.0.113.7" {
		t.Errorf("client_ip = %q, want 203.0.113.7 (forged XFF/X-Real-IP must be ignored)", got)
	}
}
