package api

import (
	"log"

	"github.com/gin-gonic/gin"
)

// PromotionGateMiddleware is the promotion kill switch (FOLLOWUPS #0e,
// ADR-0003 rollback plan step 1: "disable PROD promotion endpoint via
// feature flag or 503 short-circuit in middleware"). It gates only
// Promote and ApprovePromotion: Reject and Rollback stay live because
// they ARE the incident response (ADR-0003 step 2 depends on rollback
// remaining available mid-incident), and the read endpoints
// (diff/list/get/audit-log) mutate nothing.
//
// The flag is read once at boot (KEEPSAVE_PROMOTIONS_ENABLED via
// config.Load) — flipping it is an env change + restart, no redeploy.
// Blocked attempts are logged but NOT written to audit_log:
// a blocked call mutates no state, and "promotion.blocked" is not in
// the canonical taxonomy (docs/AUDIT_LOG_COVERAGE.md); extending the
// taxonomy is ADR-0014's job. No Retry-After header is sent — incident
// duration is unknowable and a static value would mislead clients.
func PromotionGateMiddleware(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enabled {
			c.Next()
			return
		}
		userID, _ := c.Get("user_id")
		log.Printf("promotion blocked by kill switch path=%s user_id=%v ip=%s", c.FullPath(), userID, c.ClientIP())
		WrapError(c, WrapMessage(ErrServiceUnavailable, "promotions are temporarily disabled by the operator", nil))
		c.Abort()
	}
}
