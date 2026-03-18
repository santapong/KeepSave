package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

// TrustedProxyMiddleware extracts the real client IP from reverse proxy headers
// (X-Forwarded-For, X-Real-IP) and sets X-Forwarded-Proto awareness.
// This allows KeepSave to work correctly behind nginx, Traefik, Kong, etc.
func TrustedProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prefer X-Real-IP (set by nginx), then X-Forwarded-For (first IP in chain)
		if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
			c.Set("client_ip", realIP)
		} else if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
			// X-Forwarded-For may contain comma-separated list; first is the client
			parts := strings.SplitN(forwarded, ",", 2)
			c.Set("client_ip", strings.TrimSpace(parts[0]))
		} else {
			c.Set("client_ip", c.ClientIP())
		}

		// Set scheme awareness for redirect URLs (OAuth flows, etc.)
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			c.Set("scheme", proto)
		} else {
			if c.Request.TLS != nil {
				c.Set("scheme", "https")
			} else {
				c.Set("scheme", "http")
			}
		}

		c.Next()
	}
}

func CORSMiddleware(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigins)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func JWTAuthMiddleware(jwtService *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			RespondError(c, http.StatusUnauthorized, "authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			RespondError(c, http.StatusUnauthorized, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateToken(parts[1])
		if err != nil {
			RespondError(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

func APIKeyAuthMiddleware(jwtService *auth.JWTService, apikeyRepo *repository.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try API key first
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			hashedKey := auth.HashAPIKey(apiKey)
			key, err := apikeyRepo.GetByHashedKey(hashedKey)
			if err != nil {
				RespondError(c, http.StatusUnauthorized, "invalid api key")
				c.Abort()
				return
			}

			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				RespondError(c, http.StatusUnauthorized, "api key expired")
				c.Abort()
				return
			}

			c.Set("user_id", key.UserID)
			c.Set("api_key_project_id", key.ProjectID)
			c.Set("api_key_scopes", key.Scopes)
			if key.Environment != nil {
				c.Set("api_key_environment", *key.Environment)
			}
			c.Next()
			return
		}

		// Fall back to JWT
		JWTAuthMiddleware(jwtService)(c)
	}
}
