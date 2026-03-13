package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

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
