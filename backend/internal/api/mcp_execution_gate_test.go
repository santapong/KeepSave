package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalMCPExecutionGate(t *testing.T) {
	r := gin.New()
	r.Use(localMCPExecutionGate(true))
	for _, route := range []string{"/api/v1/mcp/servers", "/api/v1/mcp/servers/:id/rebuild", "/api/v1/mcp/gateway"} {
		r.POST(route, func(c *gin.Context) { t.Error("disabled route dispatched"); c.Status(200) })
	}
	r.GET("/api/v1/mcp/servers", func(c *gin.Context) { c.Status(200) })
	for _, path := range []string{"/api/v1/mcp/servers", "/api/v1/mcp/servers/test/rebuild", "/api/v1/mcp/gateway"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != 503 {
			t.Fatalf("%s %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil))
	if w.Code != 200 {
		t.Fatal("catalog unavailable")
	}
}
