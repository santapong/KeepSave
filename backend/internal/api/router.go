package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/logging"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

func SetupRouter(
	corsOrigins string,
	jwtService *auth.JWTService,
	apikeyRepo *repository.APIKeyRepository,
	authHandler *AuthHandler,
	projectHandler *ProjectHandler,
	secretHandler *SecretHandler,
	apikeyHandler *APIKeyHandler,
	promotionHandler *PromotionHandler,
	keyRotationHandler *KeyRotationHandler,
	webhookHandler *WebhookHandler,
	versionHandler *VersionHandler,
	healthHandler *HealthHandler,
	db *sql.DB,
	logger *logging.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Structured JSON logging middleware
	if logger != nil {
		r.Use(logging.GinMiddleware(logger))
	}

	r.Use(CORSMiddleware(corsOrigins))

	// Rate limiting: 100 requests per second, burst of 200
	limiter := NewRateLimiter(100, time.Second, 200)
	r.Use(RateLimitMiddleware(limiter))

	// Health check endpoints (no auth required)
	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)

	v1 := r.Group("/api/v1")
	{
		// Auth routes (no auth required)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		// Project routes (JWT required)
		projectGroup := v1.Group("/projects")
		projectGroup.Use(JWTAuthMiddleware(jwtService))
		{
			projectGroup.POST("", projectHandler.Create)
			projectGroup.GET("", projectHandler.List)
			projectGroup.GET("/:id", projectHandler.Get)
			projectGroup.PUT("/:id", projectHandler.Update)
			projectGroup.DELETE("/:id", projectHandler.Delete)
		}

		// Secret routes (JWT or API key)
		secretGroup := v1.Group("/projects/:id/secrets")
		secretGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			secretGroup.POST("", secretHandler.Create)
			secretGroup.GET("", secretHandler.List)
			secretGroup.GET("/:secretId", secretHandler.Get)
			secretGroup.PUT("/:secretId", secretHandler.Update)
			secretGroup.DELETE("/:secretId", secretHandler.Delete)

			// Secret version history
			secretGroup.GET("/:secretId/versions", versionHandler.ListVersions)
			secretGroup.GET("/:secretId/versions/:version", versionHandler.GetVersion)
		}

		// Promotion routes (JWT required)
		promoteGroup := v1.Group("/projects/:id")
		promoteGroup.Use(JWTAuthMiddleware(jwtService))
		{
			promoteGroup.POST("/promote", promotionHandler.Promote)
			promoteGroup.POST("/promote/diff", promotionHandler.Diff)
			promoteGroup.GET("/promotions", promotionHandler.ListPromotions)
			promoteGroup.GET("/promotions/:promotionId", promotionHandler.GetPromotion)
			promoteGroup.POST("/promotions/:promotionId/approve", promotionHandler.ApprovePromotion)
			promoteGroup.POST("/promotions/:promotionId/reject", promotionHandler.RejectPromotion)
			promoteGroup.POST("/promotions/:promotionId/rollback", promotionHandler.Rollback)
			promoteGroup.GET("/audit-log", promotionHandler.AuditLog)

			// Key rotation
			promoteGroup.POST("/rotate-keys", keyRotationHandler.RotateProjectKey)
			promoteGroup.GET("/verify-encryption", keyRotationHandler.VerifyEncryption)

			// Webhooks
			promoteGroup.POST("/webhooks", webhookHandler.Register)
			promoteGroup.GET("/webhooks", webhookHandler.List)
			promoteGroup.DELETE("/webhooks", webhookHandler.Remove)
		}

		// Key rotation for all projects (JWT required)
		rotateGroup := v1.Group("/rotate-keys")
		rotateGroup.Use(JWTAuthMiddleware(jwtService))
		{
			rotateGroup.POST("", keyRotationHandler.RotateAllKeys)
		}

		// API key routes (JWT required)
		apikeyGroup := v1.Group("/api-keys")
		apikeyGroup.Use(JWTAuthMiddleware(jwtService))
		{
			apikeyGroup.POST("", apikeyHandler.Create)
			apikeyGroup.GET("", apikeyHandler.List)
			apikeyGroup.DELETE("/:id", apikeyHandler.Delete)
		}

		// Webhook deliveries (JWT required)
		webhookGroup := v1.Group("/webhook-deliveries")
		webhookGroup.Use(JWTAuthMiddleware(jwtService))
		{
			webhookGroup.GET("", webhookHandler.Deliveries)
		}
	}

	return r
}
