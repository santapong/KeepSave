package logging

import (
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware that logs requests as structured JSON.
func GinMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		fields := map[string]interface{}{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
			"latency_ms": latency.Milliseconds(),
			"bytes_out":  c.Writer.Size(),
		}

		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}

		if len(c.Errors) > 0 {
			fields["errors"] = c.Errors.String()
			logger.Error("request completed with errors", fields)
		} else if c.Writer.Status() >= 500 {
			logger.Error("server error", fields)
		} else if c.Writer.Status() >= 400 {
			logger.Warn("client error", fields)
		} else {
			logger.Info("request completed", fields)
		}
	}
}
