package api

import (
	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
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
) *gin.Engine {
	r := gin.Default()

	r.Use(CORSMiddleware(corsOrigins))

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
		}

		// API key routes (JWT required)
		apikeyGroup := v1.Group("/api-keys")
		apikeyGroup.Use(JWTAuthMiddleware(jwtService))
		{
			apikeyGroup.POST("", apikeyHandler.Create)
			apikeyGroup.GET("", apikeyHandler.List)
			apikeyGroup.DELETE("/:id", apikeyHandler.Delete)
		}
	}

	return r
}
