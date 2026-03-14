package metrics

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns a Gin middleware that records request metrics.
func GinMiddleware(m *AppMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		label := fmt.Sprintf("%s_%s", method, path)

		m.RequestsInFlight.Inc("")
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		m.RequestsInFlight.Dec("")
		m.RequestsTotal.Inc(label)
		m.RequestDuration.ObserveDuration(label, duration)

		if status >= 400 {
			errLabel := fmt.Sprintf("%d_%s", status, path)
			m.ErrorsTotal.Inc(errLabel)
		}

		if status == 429 {
			m.RateLimitHits.Inc(c.ClientIP())
		}
	}
}
