package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Production never executes user-selected commands in the vault API process.
// Catalog and revocation/management reads remain available for migration.
func localMCPExecutionGate(disabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.FullPath()
		if disabled && c.Request.Method == http.MethodPost &&
			(p == "/api/v1/mcp/servers" || p == "/api/v1/mcp/gateway" || strings.HasSuffix(p, "/rebuild")) {
			RespondError(c, http.StatusServiceUnavailable, "local connector execution is disabled; use an approved isolated runner")
			c.Abort()
			return
		}
		c.Next()
	}
}
