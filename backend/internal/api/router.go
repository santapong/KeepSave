package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/logging"
	"github.com/santapong/KeepSave/backend/internal/metrics"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/tracing"
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
	orgHandler *OrganizationHandler,
	templateHandler *TemplateHandler,
	envFileHandler *EnvFileHandler,
	depHandler *DependencyHandler,
	metricsHandler *MetricsHandler,
	enterpriseHandler *EnterpriseHandler,
	agentHandler *AgentHandler,
	platformHandler *PlatformHandler,
	openAPIHandler *OpenAPIHandler,
	oauthHandler *OAuthHandler,
	mcpHubHandler *MCPHubHandler,
	mcpGatewayHandler *MCPGatewayHandler,
	applicationHandler *ApplicationHandler,
	intelligenceHandler *IntelligenceHandler,
	appMetrics *metrics.AppMetrics,
	tracer *tracing.Tracer,
	db *sql.DB,
	logger *logging.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TrustedProxyMiddleware())
	r.Use(SecurityHeadersMiddleware())
	r.Use(RequestSizeLimitMiddleware(1 << 20))
	if logger != nil {
		r.Use(logging.GinMiddleware(logger))
	}
	r.Use(CORSMiddleware(corsOrigins))
	limiter := NewRateLimiter(100, time.Second, 200)
	r.Use(RateLimitMiddleware(limiter))
	if appMetrics != nil {
		r.Use(metrics.GinMiddleware(appMetrics))
	}
	if tracer != nil {
		r.Use(tracing.GinMiddleware(tracer))
	}

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)
	r.GET("/metrics", metricsHandler.Metrics)
	r.GET("/api/docs", openAPIHandler.Spec)
	r.GET("/.well-known/openid-configuration", oauthHandler.OpenIDConfiguration)

	v1 := r.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		usersGroup := v1.Group("/users")
		usersGroup.Use(JWTAuthMiddleware(jwtService))
		{
			usersGroup.GET("/lookup", authHandler.LookupUser)
		}

		projectGroup := v1.Group("/projects")
		projectGroup.Use(JWTAuthMiddleware(jwtService))
		{
			projectGroup.POST("", projectHandler.Create)
			projectGroup.GET("", projectHandler.List)
			projectGroup.GET("/:id", projectHandler.Get)
			projectGroup.PUT("/:id", projectHandler.Update)
			projectGroup.DELETE("/:id", projectHandler.Delete)
		}

		secretGroup := v1.Group("/projects/:id/secrets")
		secretGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			secretGroup.POST("", secretHandler.Create)
			secretGroup.GET("", secretHandler.List)
			secretGroup.GET("/:secretId", secretHandler.Get)
			secretGroup.PUT("/:secretId", secretHandler.Update)
			secretGroup.DELETE("/:secretId", secretHandler.Delete)
			secretGroup.GET("/:secretId/versions", versionHandler.ListVersions)
			secretGroup.GET("/:secretId/versions/:version", versionHandler.GetVersion)
		}

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
			promoteGroup.POST("/rotate-keys", keyRotationHandler.RotateProjectKey)
			promoteGroup.GET("/verify-encryption", keyRotationHandler.VerifyEncryption)
			promoteGroup.POST("/webhooks", webhookHandler.Register)
			promoteGroup.GET("/webhooks", webhookHandler.List)
			promoteGroup.DELETE("/webhooks", webhookHandler.Remove)
			promoteGroup.GET("/env-export", envFileHandler.Export)
			promoteGroup.POST("/env-import", envFileHandler.Import)
			promoteGroup.POST("/dependencies/analyze", depHandler.Analyze)
			promoteGroup.GET("/dependencies/graph", depHandler.Graph)
			promoteGroup.POST("/backups", enterpriseHandler.CreateBackup)
			promoteGroup.GET("/backups", enterpriseHandler.ListBackups)
			promoteGroup.GET("/policy", enterpriseHandler.GetSecretPolicy)
			promoteGroup.PUT("/policy", enterpriseHandler.SetSecretPolicy)
			promoteGroup.GET("/agent-activity", agentHandler.GetRecentActivity)
			promoteGroup.GET("/agent-heatmap", agentHandler.GetAccessHeatmap)
			promoteGroup.GET("/access-policies", platformHandler.ListAccessPolicies)
			promoteGroup.POST("/access-policies", platformHandler.CreateAccessPolicy)
			promoteGroup.DELETE("/access-policies/:policyId", platformHandler.DeleteAccessPolicy)

			// Phase 15: AI Intelligence - per-project endpoints
			promoteGroup.POST("/drift", intelligenceHandler.DetectDrift)
			promoteGroup.GET("/drift", intelligenceHandler.ListDriftChecks)
			promoteGroup.POST("/anomalies/scan", intelligenceHandler.RunAnomalyDetection)
			promoteGroup.GET("/analytics/trends", intelligenceHandler.GetUsageTrends)
			promoteGroup.GET("/analytics/forecast", intelligenceHandler.GetUsageForecast)
			promoteGroup.POST("/recommendations/generate", intelligenceHandler.GenerateRecommendations)
			promoteGroup.GET("/recommendations", intelligenceHandler.ListRecommendations)
			promoteGroup.DELETE("/recommendations/:recId", intelligenceHandler.DismissRecommendation)
		}

		leaseGroup := v1.Group("/projects/:id/leases")
		leaseGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			leaseGroup.POST("", agentHandler.CreateLease)
			leaseGroup.GET("", agentHandler.ListLeases)
			leaseGroup.DELETE("/:leaseId", agentHandler.RevokeLease)
		}

		rotateGroup := v1.Group("/rotate-keys")
		rotateGroup.Use(JWTAuthMiddleware(jwtService))
		{
			rotateGroup.POST("", keyRotationHandler.RotateAllKeys)
		}

		apikeyGroup := v1.Group("/api-keys")
		apikeyGroup.Use(JWTAuthMiddleware(jwtService))
		{
			apikeyGroup.POST("", apikeyHandler.Create)
			apikeyGroup.GET("", apikeyHandler.List)
			apikeyGroup.DELETE("/:id", apikeyHandler.Delete)
		}

		webhookGroup := v1.Group("/webhook-deliveries")
		webhookGroup.Use(JWTAuthMiddleware(jwtService))
		{
			webhookGroup.GET("", webhookHandler.Deliveries)
		}

		orgGroup := v1.Group("/organizations")
		orgGroup.Use(JWTAuthMiddleware(jwtService))
		{
			orgGroup.POST("", orgHandler.Create)
			orgGroup.GET("", orgHandler.List)
			orgGroup.GET("/:orgId", orgHandler.Get)
			orgGroup.PUT("/:orgId", orgHandler.Update)
			orgGroup.DELETE("/:orgId", orgHandler.Delete)
			orgGroup.POST("/:orgId/members", orgHandler.AddMember)
			orgGroup.GET("/:orgId/members", orgHandler.ListMembers)
			orgGroup.PUT("/:orgId/members/:userId", orgHandler.UpdateMemberRole)
			orgGroup.DELETE("/:orgId/members/:userId", orgHandler.RemoveMember)
			orgGroup.POST("/:orgId/projects", orgHandler.AssignProject)
			orgGroup.GET("/:orgId/projects", orgHandler.ListProjects)
			orgGroup.POST("/:orgId/sso", enterpriseHandler.ConfigureSSO)
			orgGroup.GET("/:orgId/sso", enterpriseHandler.ListSSOConfigs)
			orgGroup.DELETE("/:orgId/sso/:provider", enterpriseHandler.DeleteSSOConfig)
			orgGroup.POST("/:orgId/compliance", enterpriseHandler.GenerateComplianceReport)
			orgGroup.GET("/:orgId/compliance", enterpriseHandler.ListComplianceReports)
		}

		templateGroup := v1.Group("/templates")
		templateGroup.Use(JWTAuthMiddleware(jwtService))
		{
			templateGroup.POST("", templateHandler.Create)
			templateGroup.GET("", templateHandler.List)
			templateGroup.GET("/builtin", templateHandler.ListBuiltin)
			templateGroup.GET("/:templateId", templateHandler.Get)
			templateGroup.PUT("/:templateId", templateHandler.Update)
			templateGroup.DELETE("/:templateId", templateHandler.Delete)
			templateGroup.POST("/:templateId/apply", templateHandler.Apply)
		}

		adminGroup := v1.Group("/admin")
		adminGroup.Use(JWTAuthMiddleware(jwtService))
		{
			adminGroup.GET("/dashboard", metricsHandler.AdminDashboard)
			adminGroup.GET("/traces", metricsHandler.Traces)
		}

		agentGroup := v1.Group("/agent")
		agentGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			agentGroup.GET("/activity", agentHandler.GetActivitySummary)
		}

		platformGroup := v1.Group("/platform")
		platformGroup.Use(JWTAuthMiddleware(jwtService))
		{
			platformGroup.GET("/events", platformHandler.ListEvents)
			platformGroup.POST("/events/replay", platformHandler.ReplayEvents)
			platformGroup.GET("/plugins", platformHandler.ListPlugins)
			platformGroup.POST("/plugins", platformHandler.RegisterPlugin)
			platformGroup.PUT("/plugins/:pluginId", platformHandler.TogglePlugin)
		}

		// Phase 15: AI Intelligence - global endpoints
		aiGroup := v1.Group("/ai")
		aiGroup.Use(JWTAuthMiddleware(jwtService))
		{
			aiGroup.GET("/providers", intelligenceHandler.ListProviders)
			aiGroup.POST("/query", intelligenceHandler.NLPQuery)
			aiGroup.GET("/anomalies", intelligenceHandler.ListAnomalies)
			aiGroup.PUT("/anomalies/:anomalyId/acknowledge", intelligenceHandler.AcknowledgeAnomaly)
			aiGroup.PUT("/anomalies/:anomalyId/resolve", intelligenceHandler.ResolveAnomaly)
		}

		oauthPublicGroup := v1.Group("/oauth")
		{
			oauthPublicGroup.POST("/token", oauthHandler.Token)
			oauthPublicGroup.GET("/userinfo", oauthHandler.UserInfo)
			oauthPublicGroup.POST("/revoke", oauthHandler.Revoke)
			oauthPublicGroup.GET("/.well-known/jwks.json", oauthHandler.JWKS)
		}

		oauthAuthGroup := v1.Group("/oauth")
		oauthAuthGroup.Use(JWTAuthMiddleware(jwtService))
		{
			oauthAuthGroup.GET("/authorize", oauthHandler.Authorize)
			oauthAuthGroup.POST("/clients", oauthHandler.RegisterClient)
			oauthAuthGroup.GET("/clients", oauthHandler.ListClients)
			oauthAuthGroup.DELETE("/clients/:clientId", oauthHandler.DeleteClient)
		}

		mcpPublicGroup := v1.Group("/mcp")
		{
			mcpPublicGroup.GET("/servers/public", mcpHubHandler.ListPublicServers)
		}

		mcpAuthGroup := v1.Group("/mcp")
		mcpAuthGroup.Use(JWTAuthMiddleware(jwtService))
		{
			mcpAuthGroup.POST("/servers", mcpHubHandler.RegisterServer)
			mcpAuthGroup.GET("/servers", mcpHubHandler.ListMyServers)
			mcpAuthGroup.GET("/servers/:serverId", mcpHubHandler.GetServer)
			mcpAuthGroup.PUT("/servers/:serverId", mcpHubHandler.UpdateServer)
			mcpAuthGroup.DELETE("/servers/:serverId", mcpHubHandler.DeleteServer)
			mcpAuthGroup.POST("/servers/:serverId/rebuild", mcpHubHandler.RebuildServer)
			mcpAuthGroup.POST("/installations", mcpHubHandler.InstallServer)
			mcpAuthGroup.GET("/installations", mcpHubHandler.ListInstallations)
			mcpAuthGroup.PUT("/installations/:installId", mcpHubHandler.UpdateInstallation)
			mcpAuthGroup.DELETE("/installations/:installId", mcpHubHandler.UninstallServer)
			mcpAuthGroup.POST("/gateway", mcpGatewayHandler.HandleToolCall)
			mcpAuthGroup.GET("/gateway/tools", mcpGatewayHandler.ListTools)
			mcpAuthGroup.GET("/gateway/stats", mcpHubHandler.GetGatewayStats)
			mcpAuthGroup.GET("/config", mcpGatewayHandler.MCPConfig)
		}

		appGroup := v1.Group("/applications")
		appGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			appGroup.POST("", applicationHandler.Create)
			appGroup.GET("", applicationHandler.List)
			appGroup.GET("/:appId", applicationHandler.Get)
			appGroup.PUT("/:appId", applicationHandler.Update)
			appGroup.DELETE("/:appId", applicationHandler.Delete)
			appGroup.POST("/:appId/favorite", applicationHandler.ToggleFavorite)
		}
	}

	return r
}
