package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// RunnerRouter is served only on the dedicated mutual-TLS listener. It exposes
// no browser, vault, database or arbitrary proxy operation.
func RunnerRouter(h *ToolPlatformHandler) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
		c.Header("Cache-Control", "no-store")
		c.Next()
	})
	h.RegisterRunnerRoutes(r.Group("/api/v1"))
	return r
}
