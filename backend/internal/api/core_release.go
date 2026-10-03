package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// coreRoute is mounted as the final handler, after identity and resource gates.
// Preserved legacy source is deliberately unavailable until its own acceptance
// journey proves the guarantees of the core release.
func coreRoute(core bool, capability string, legacy gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if core {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": ErrorDetail{Code: http.StatusServiceUnavailable, ErrorCode: "CAPABILITY_UNAVAILABLE", Message: "capability is unavailable in the core release"}, "capability": capability})
			return
		}
		legacy(c)
	}
}
func coreCapabilities(core bool, durableVault bool, platform ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		available := []string{"password_login", "social_login_when_configured", "human_sessions", "organizations", "projects", "secret_crud", "secret_batch"}
		if durableVault {
			available = append(available, "secret_history", "secret_restore", "encrypted_recovery", "key_rotation_history")
		}
		available = append(available, platform...)
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"profile": "core", "application_origin": "https://app.keepsave.draveniq.dev", "landing_origin": "https://keepsave.draveniq.dev", "restricted_profile": core, "available": available, "postgresql_only": []string{"secret_history", "secret_restore", "key_rotation_history", "encrypted_recovery"}, "unavailable": []string{"mcp_execution", "experimental_intelligence", "dependency_analysis", "webhook_automation", "enterprise_sso", "compliance_assessment", "policy_metadata", "legacy_oauth", "event_replay", "plugin_execution"}})
	}
}
