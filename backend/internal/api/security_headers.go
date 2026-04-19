package api

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware sets security-related HTTP headers on all responses.
// The CSP is intentionally strict: no 'unsafe-inline' anywhere. The dashboard
// uses Tailwind-compiled stylesheets and Shadcn components that emit classes,
// not inline <style> tags. The embed widget uses Shadow DOM which sets styles
// via a <link> element shipped with the widget bundle.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		c.Next()
	}
}

// RequestSizeLimitMiddleware limits the request body size.
func RequestSizeLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = newLimitedReader(c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

type limitedReader struct {
	r         interface{ Read([]byte) (int, error) }
	remaining int64
}

func newLimitedReader(r interface{ Read([]byte) (int, error) }, limit int64) *limitedReader {
	return &limitedReader{r: r, remaining: limit}
}

func (lr *limitedReader) Read(p []byte) (int, error) {
	if lr.remaining <= 0 {
		return 0, &requestTooLargeError{}
	}
	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}
	n, err := lr.r.Read(p)
	lr.remaining -= int64(n)
	return n, err
}

func (lr *limitedReader) Close() error {
	if closer, ok := lr.r.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type requestTooLargeError struct{}

func (e *requestTooLargeError) Error() string {
	return "request body too large"
}
