package logging

import (
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryParams are redacted from the request log so an attacker
// who reads the log cannot recover credentials that should never have
// been on the wire as querystring. Per audit S-M2. The list matches
// OWASP's "do not log" guidance plus KeepSave-specific tokens.
var sensitiveQueryParams = map[string]struct{}{
	"password":      {},
	"token":         {},
	"access_token":  {},
	"refresh_token": {},
	"id_token":      {},
	"api_key":       {},
	"apikey":        {},
	"code":          {},
	"code_verifier": {},
	"client_secret": {},
	"secret":        {},
	// email is enumeration-sensitive on routes like /users/lookup. Redact
	// in logs; the application code can still read the query directly.
	"email": {},
}

// redactQuery returns raw with any sensitive parameter value replaced by
// "REDACTED" while keeping the param name visible for debugging. Anything
// that fails to parse is returned unchanged - we'd rather log a
// malformed query than swallow it.
func redactQuery(raw string) string {
	if raw == "" {
		return raw
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	dirty := false
	for k := range values {
		if _, sensitive := sensitiveQueryParams[k]; sensitive {
			values.Set(k, "REDACTED")
			dirty = true
		}
	}
	if !dirty {
		return raw
	}
	return values.Encode()
}

// GinMiddleware returns a Gin middleware that logs requests as structured JSON.
func GinMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactQuery(c.Request.URL.RawQuery)

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
