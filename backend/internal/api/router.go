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
	appMetrics *metrics.AppMetrics,
	tracer *tracing.Tracer,
	db *sql.DB,
	logger *logging.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Phase 10: Security headers
	r.Use(SecurityHeadersMiddleware())

	// Phase 10: Request body size limit (1MB)
	r.Use(RequestSizeLimitMiddleware(1 << 20))

	// Structured JSON logging middleware
	if logger != nil {
		r.Use(logging.GinMiddleware(logger))
	}

	r.Use(CORSMiddleware(corsOrigins))

	// Rate limiting: 100 requests per second, burst of 200
	limiter := NewRateLimiter(100, time.Second, 200)
	r.Use(RateLimitMiddleware(limiter))

	// Phase 7: Metrics middleware
	if appMetrics != nil {
		r.Use(metrics.GinMiddleware(appMetrics))
	}

	// Phase 7: Tracing middleware
	if tracer != nil {
		r.Use(tracing.GinMiddleware(tracer))
	}

	// Health check endpoints (no auth required)
	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)

	// Phase 7: Metrics endpoint (no auth required)
	r.GET("/metrics", metricsHandler.Metrics)

	// Phase 8: OpenAPI spec (no auth required)
	r.GET("/api/docs", openAPIHandler.Spec)

	// OIDC Discovery (no auth required)
	r.GET("/.well-known/openid-configuration", oauthHandler.OpenIDConfiguration)

	v1 := r.Group("/api/v1")
	{
		// Auth routes (no auth required)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		// User lookup (JWT required)
		usersGroup := v1.Group("/users")
		usersGroup.Use(JWTAuthMiddleware(jwtService))
		{
			usersGroup.GET("/lookup", authHandler.LookupUser)
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

			// Import/Export .env files
			promoteGroup.GET("/env-export", envFileHandler.Export)
			promoteGroup.POST("/env-import", envFileHandler.Import)

			// Secret dependency graph
			promoteGroup.POST("/dependencies/analyze", depHandler.Analyze)
			promoteGroup.GET("/dependencies/graph", depHandler.Graph)

			// Phase 9: Backups
			promoteGroup.POST("/backups", enterpriseHandler.CreateBackup)
			promoteGroup.GET("/backups", enterpriseHandler.ListBackups)

			// Phase 9: Secret policies
			promoteGroup.GET("/policy", enterpriseHandler.GetSecretPolicy)
			promoteGroup.PUT("/policy", enterpriseHandler.SetSecretPolicy)

			// Phase 11: Agent activity
			promoteGroup.GET("/agent-activity", agentHandler.GetRecentActivity)
			promoteGroup.GET("/agent-heatmap", agentHandler.GetAccessHeatmap)

			// Phase 12: Access policies
			promoteGroup.GET("/access-policies", platformHandler.ListAccessPolicies)
			promoteGroup.POST("/access-policies", platformHandler.CreateAccessPolicy)
			promoteGroup.DELETE("/access-policies/:policyId", platformHandler.DeleteAccessPolicy)
		}

		// Phase 11: Agent lease routes (JWT or API key)
		leaseGroup := v1.Group("/projects/:id/leases")
		leaseGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			leaseGroup.POST("", agentHandler.CreateLease)
			leaseGroup.GET("", agentHandler.ListLeases)
			leaseGroup.DELETE("/:leaseId", agentHandler.RevokeLease)
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

		// Organization routes (JWT required)
		orgGroup := v1.Group("/organizations")
		orgGroup.Use(JWTAuthMiddleware(jwtService))
		{
			orgGroup.POST("", orgHandler.Create)
			orgGroup.GET("", orgHandler.List)
			orgGroup.GET("/:orgId", orgHandler.Get)
			orgGroup.PUT("/:orgId", orgHandler.Update)
			orgGroup.DELETE("/:orgId", orgHandler.Delete)

			// Members
			orgGroup.POST("/:orgId/members", orgHandler.AddMember)
			orgGroup.GET("/:orgId/members", orgHandler.ListMembers)
			orgGroup.PUT("/:orgId/members/:userId", orgHandler.UpdateMemberRole)
			orgGroup.DELETE("/:orgId/members/:userId", orgHandler.RemoveMember)

			// Project assignment
			orgGroup.POST("/:orgId/projects", orgHandler.AssignProject)
			orgGroup.GET("/:orgId/projects", orgHandler.ListProjects)

			// Phase 9: SSO
			orgGroup.POST("/:orgId/sso", enterpriseHandler.ConfigureSSO)
			orgGroup.GET("/:orgId/sso", enterpriseHandler.ListSSOConfigs)
			orgGroup.DELETE("/:orgId/sso/:provider", enterpriseHandler.DeleteSSOConfig)

			// Phase 9: Compliance
			orgGroup.POST("/:orgId/compliance", enterpriseHandler.GenerateComplianceReport)
			orgGroup.GET("/:orgId/compliance", enterpriseHandler.ListComplianceReports)
		}

		// Template routes (JWT required)
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

		// Phase 7: Admin routes (JWT required)
		adminGroup := v1.Group("/admin")
		adminGroup.Use(JWTAuthMiddleware(jwtService))
		{
			adminGroup.GET("/dashboard", metricsHandler.AdminDashboard)
			adminGroup.GET("/traces", metricsHandler.Traces)
		}

		// Phase 11: Agent analytics routes (JWT or API key)
		agentGroup := v1.Group("/agent")
		agentGroup.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{
			agentGroup.GET("/activity", agentHandler.GetActivitySummary)
		}

		// Phase 12: Platform routes (JWT required)
		platformGroup := v1.Group("/platform")
		platformGroup.Use(JWTAuthMiddleware(jwtService))
		{
			platformGroup.GET("/events", platformHandler.ListEvents)
			platformGroup.POST("/events/replay", platformHandler.ReplayEvents)
			platformGroup.GET("/plugins", platformHandler.ListPlugins)
			platformGroup.POST("/plugins", platformHandler.RegisterPlugin)
			platformGroup.PUT("/plugins/:pluginId", platformHandler.TogglePlugin)
		}

		// Phase 13: OAuth 2.0 Provider
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

		// Phase 13: MCP Server Hub
		mcpPublicGroup := v1.Group("/mcp")
		{
			mcpPublicGroup.GET("/servers/public", mcpHubHandler.ListPublicServers)
		}

		mcpAuthGroup := v1.Group("/mcp")
		mcpAuthGroup.Use(JWTAuthMiddleware(jwtService))
		{
			// Server management
			mcpAuthGroup.POST("/servers", mcpHubHandler.RegisterServer)
			mcpAuthGroup.GET("/servers", mcpHubHandler.ListMyServers)
			mcpAuthGroup.GET("/servers/:serverId", mcpHubHandler.GetServer)
			mcpAuthGroup.PUT("/servers/:serverId", mcpHubHandler.UpdateServer)
			mcpAuthGroup.DELETE("/servers/:serverId", mcpHubHandler.DeleteServer)
			mcpAuthGroup.POST("/servers/:serverId/rebuild", mcpHubHandler.RebuildServer)

			// Installation management
			mcpAuthGroup.POST("/installations", mcpHubHandler.InstallServer)
			mcpAuthGroup.GET("/installations", mcpHubHandler.ListInstallations)
			mcpAuthGroup.PUT("/installations/:installId", mcpHubHandler.UpdateInstallation)
			mcpAuthGroup.DELETE("/installations/:installId", mcpHubHandler.UninstallServer)

			// Gateway
			mcpAuthGroup.POST("/gateway", mcpGatewayHandler.HandleToolCall)
			mcpAuthGroup.GET("/gateway/tools", mcpGatewayHandler.ListTools)
			mcpAuthGroup.GET("/gateway/stats", mcpHubHandler.GetGatewayStats)
			mcpAuthGroup.GET("/config", mcpGatewayHandler.MCPConfig)
		}
	}

	return r
}
