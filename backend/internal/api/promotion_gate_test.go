package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newPromotionGateRouter mirrors the production wiring in router.go: the
// gate is applied per-route to promote and approve only, while reject,
// rollback, diff, and list share the same group but stay ungated. Stub
// handlers return 200 so any non-200 outcome traces back to the gate.
func newPromotionGateRouter(enabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	gate := PromotionGateMiddleware(enabled)
	pm := r.Group("/projects/:id")
	{
		pm.POST("/promote", gate, ok)
		pm.POST("/promote/diff", ok)
		pm.GET("/promotions", ok)
		pm.POST("/promotions/:promotionId/approve", gate, ok)
		pm.POST("/promotions/:promotionId/reject", ok)
		pm.POST("/promotions/:promotionId/rollback", ok)
	}
	return r
}

func TestPromotionGate(t *testing.T) {
	cases := []struct {
		name       string
		enabled    bool
		method     string
		path       string
		wantStatus int
	}{
		{"disabled blocks promote", false, http.MethodPost, "/projects/p1/promote", http.StatusServiceUnavailable},
		{"disabled blocks approve", false, http.MethodPost, "/projects/p1/promotions/pr1/approve", http.StatusServiceUnavailable},
		{"disabled keeps reject live", false, http.MethodPost, "/projects/p1/promotions/pr1/reject", http.StatusOK},
		{"disabled keeps rollback live", false, http.MethodPost, "/projects/p1/promotions/pr1/rollback", http.StatusOK},
		{"disabled keeps diff live", false, http.MethodPost, "/projects/p1/promote/diff", http.StatusOK},
		{"disabled keeps list live", false, http.MethodGet, "/projects/p1/promotions", http.StatusOK},
		{"enabled passes promote", true, http.MethodPost, "/projects/p1/promote", http.StatusOK},
		{"enabled passes approve", true, http.MethodPost, "/projects/p1/promotions/pr1/approve", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newPromotionGateRouter(tc.enabled)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("%s %s = %d, want %d", tc.method, tc.path, w.Code, tc.wantStatus)
			}
		})
	}
}

// TestPromotionGate_ResponseBody pins the client contract: the 503 body
// must carry the SERVICE_UNAVAILABLE symbol and the operator-facing
// message that the frontend banners render verbatim.
func TestPromotionGate_ResponseBody(t *testing.T) {
	r := newPromotionGateRouter(false)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/projects/p1/promote", nil)
	r.ServeHTTP(w, req)

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	if resp.Error.Code != http.StatusServiceUnavailable {
		t.Errorf("error.code = %d, want %d", resp.Error.Code, http.StatusServiceUnavailable)
	}
	if resp.Error.ErrorCode != "SERVICE_UNAVAILABLE" {
		t.Errorf("error.error_code = %q, want SERVICE_UNAVAILABLE", resp.Error.ErrorCode)
	}
	if want := "promotions are temporarily disabled by the operator"; resp.Error.Message != want {
		t.Errorf("error.message = %q, want %q", resp.Error.Message, want)
	}
}
