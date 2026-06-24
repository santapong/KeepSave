package api

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// PanicRecoveryMiddleware catches panics inside handler chains and emits
// the canonical INTERNAL error response shape instead of Gin's default
// plain 500. The stack trace is logged server-side (with the request
// path) but never reaches the client. Per audit B-M1.
//
// This middleware is mounted INSTEAD OF gin.Recovery so the response
// body shape matches what httperror.WrapError produces - frontend code
// can rely on `{"error":{"code":500,"message":"internal error",
// "error_code":"INTERNAL"}}` regardless of whether the 500 came from a
// returned error or a panic.
func PanicRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic TYPE, not its value (A-09): a panic value can
				// carry a decrypted secret or other sensitive payload that must
				// not reach logs. The stack trace pinpoints the location.
				log.Printf("panic recovered method=%s path=%s remote=%s panic_type=%T\nstack=%s",
					c.Request.Method, c.FullPath(), c.ClientIP(), r, debug.Stack())
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
						Error: ErrorDetail{
							Code:      http.StatusInternalServerError,
							Message:   ErrInternal.Message,
							ErrorCode: ErrInternal.Symbol,
						},
					})
				} else {
					c.Abort()
				}
			}
		}()
		c.Next()
	}
}
