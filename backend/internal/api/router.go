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
	corsOrigins string, jwtService *auth.JWTService, apikeyRepo *repository.APIKeyRepository,
	authHandler *AuthHandler, projectHandler *ProjectHandler, secretHandler *SecretHandler,
	apikeyHandler *APIKeyHandler, promotionHandler *PromotionHandler, keyRotationHandler *KeyRotationHandler,
	webhookHandler *WebhookHandler, versionHandler *VersionHandler, healthHandler *HealthHandler,
	orgHandler *OrganizationHandler, templateHandler *TemplateHandler, envFileHandler *EnvFileHandler,
	depHandler *DependencyHandler, metricsHandler *MetricsHandler, enterpriseHandler *EnterpriseHandler,
	agentHandler *AgentHandler, platformHandler *PlatformHandler, openAPIHandler *OpenAPIHandler,
	oauthHandler *OAuthHandler, mcpHubHandler *MCPHubHandler, mcpGatewayHandler *MCPGatewayHandler,
	applicationHandler *ApplicationHandler, intelligenceHandler *IntelligenceHandler,
	appMetrics *metrics.AppMetrics, tracer *tracing.Tracer, db *sql.DB, logger *logging.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(TrustedProxyMiddleware())
	r.Use(SecurityHeadersMiddleware())
	r.Use(RequestSizeLimitMiddleware(1 << 20))
	if logger != nil { r.Use(logging.GinMiddleware(logger)) }
	r.Use(CORSMiddleware(corsOrigins))
	r.Use(RateLimitMiddleware(NewRateLimiter(100, time.Second, 200)))
	if appMetrics != nil { r.Use(metrics.GinMiddleware(appMetrics)) }
	if tracer != nil { r.Use(tracing.GinMiddleware(tracer)) }

	r.GET("/healthz", healthHandler.Liveness)
	r.GET("/readyz", healthHandler.Readiness)
	r.GET("/metrics", metricsHandler.Metrics)
	r.GET("/api/docs", openAPIHandler.Spec)
	r.GET("/.well-known/openid-configuration", oauthHandler.OpenIDConfiguration)

	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{ auth.POST("/register", authHandler.Register); auth.POST("/login", authHandler.Login) }

		users := v1.Group("/users"); users.Use(JWTAuthMiddleware(jwtService))
		{ users.GET("/lookup", authHandler.LookupUser) }

		proj := v1.Group("/projects"); proj.Use(JWTAuthMiddleware(jwtService))
		{ proj.POST("", projectHandler.Create); proj.GET("", projectHandler.List); proj.GET("/:id", projectHandler.Get); proj.PUT("/:id", projectHandler.Update); proj.DELETE("/:id", projectHandler.Delete) }

		sec := v1.Group("/projects/:id/secrets"); sec.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{ sec.POST("", secretHandler.Create); sec.GET("", secretHandler.List); sec.GET("/:secretId", secretHandler.Get); sec.PUT("/:secretId", secretHandler.Update); sec.DELETE("/:secretId", secretHandler.Delete); sec.GET("/:secretId/versions", versionHandler.ListVersions); sec.GET("/:secretId/versions/:version", versionHandler.GetVersion) }

		pm := v1.Group("/projects/:id"); pm.Use(JWTAuthMiddleware(jwtService))
		{
			pm.POST("/promote", promotionHandler.Promote); pm.POST("/promote/diff", promotionHandler.Diff)
			pm.GET("/promotions", promotionHandler.ListPromotions); pm.GET("/promotions/:promotionId", promotionHandler.GetPromotion)
			pm.POST("/promotions/:promotionId/approve", promotionHandler.ApprovePromotion); pm.POST("/promotions/:promotionId/reject", promotionHandler.RejectPromotion)
			pm.POST("/promotions/:promotionId/rollback", promotionHandler.Rollback); pm.GET("/audit-log", promotionHandler.AuditLog)
			pm.POST("/rotate-keys", keyRotationHandler.RotateProjectKey); pm.GET("/verify-encryption", keyRotationHandler.VerifyEncryption)
			pm.POST("/webhooks", webhookHandler.Register); pm.GET("/webhooks", webhookHandler.List); pm.DELETE("/webhooks", webhookHandler.Remove)
			pm.GET("/env-export", envFileHandler.Export); pm.POST("/env-import", envFileHandler.Import)
			pm.POST("/dependencies/analyze", depHandler.Analyze); pm.GET("/dependencies/graph", depHandler.Graph)
			pm.POST("/backups", enterpriseHandler.CreateBackup); pm.GET("/backups", enterpriseHandler.ListBackups)
			pm.GET("/policy", enterpriseHandler.GetSecretPolicy); pm.PUT("/policy", enterpriseHandler.SetSecretPolicy)
			pm.GET("/agent-activity", agentHandler.GetRecentActivity); pm.GET("/agent-heatmap", agentHandler.GetAccessHeatmap)
			pm.GET("/access-policies", platformHandler.ListAccessPolicies); pm.POST("/access-policies", platformHandler.CreateAccessPolicy); pm.DELETE("/access-policies/:policyId", platformHandler.DeleteAccessPolicy)

			// Phase 15: AI Intelligence - per-project
			pm.POST("/drift", intelligenceHandler.DetectDrift); pm.GET("/drift", intelligenceHandler.ListDriftChecks)
			pm.POST("/drift/schedules", intelligenceHandler.CreateDriftSchedule); pm.GET("/drift/schedules", intelligenceHandler.ListDriftSchedules)
			pm.PUT("/drift/schedules/:scheduleId", intelligenceHandler.UpdateDriftSchedule); pm.DELETE("/drift/schedules/:scheduleId", intelligenceHandler.DeleteDriftSchedule)
			pm.POST("/anomalies/scan", intelligenceHandler.RunAnomalyDetection)
			pm.GET("/analytics/trends", intelligenceHandler.GetUsageTrends); pm.GET("/analytics/forecast", intelligenceHandler.GetUsageForecast)
			pm.GET("/analytics/export", intelligenceHandler.ExportAnalyticsCSV)
			pm.POST("/recommendations/generate", intelligenceHandler.GenerateRecommendations); pm.GET("/recommendations", intelligenceHandler.ListRecommendations); pm.DELETE("/recommendations/:recId", intelligenceHandler.DismissRecommendation)
		}

		ls := v1.Group("/projects/:id/leases"); ls.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{ ls.POST("", agentHandler.CreateLease); ls.GET("", agentHandler.ListLeases); ls.DELETE("/:leaseId", agentHandler.RevokeLease) }

		rk := v1.Group("/rotate-keys"); rk.Use(JWTAuthMiddleware(jwtService))
		{ rk.POST("", keyRotationHandler.RotateAllKeys) }

		ak := v1.Group("/api-keys"); ak.Use(JWTAuthMiddleware(jwtService))
		{ ak.POST("", apikeyHandler.Create); ak.GET("", apikeyHandler.List); ak.DELETE("/:id", apikeyHandler.Delete) }

		wh := v1.Group("/webhook-deliveries"); wh.Use(JWTAuthMiddleware(jwtService))
		{ wh.GET("", webhookHandler.Deliveries) }

		og := v1.Group("/organizations"); og.Use(JWTAuthMiddleware(jwtService))
		{
			og.POST("", orgHandler.Create); og.GET("", orgHandler.List); og.GET("/:orgId", orgHandler.Get); og.PUT("/:orgId", orgHandler.Update); og.DELETE("/:orgId", orgHandler.Delete)
			og.POST("/:orgId/members", orgHandler.AddMember); og.GET("/:orgId/members", orgHandler.ListMembers); og.PUT("/:orgId/members/:userId", orgHandler.UpdateMemberRole); og.DELETE("/:orgId/members/:userId", orgHandler.RemoveMember)
			og.POST("/:orgId/projects", orgHandler.AssignProject); og.GET("/:orgId/projects", orgHandler.ListProjects)
			og.POST("/:orgId/sso", enterpriseHandler.ConfigureSSO); og.GET("/:orgId/sso", enterpriseHandler.ListSSOConfigs); og.DELETE("/:orgId/sso/:provider", enterpriseHandler.DeleteSSOConfig)
			og.POST("/:orgId/compliance", enterpriseHandler.GenerateComplianceReport); og.GET("/:orgId/compliance", enterpriseHandler.ListComplianceReports)
			// Phase 15: Quota management per org
			og.GET("/:orgId/quota", intelligenceHandler.GetQuota); og.PUT("/:orgId/quota", intelligenceHandler.SetQuota)
		}

		tpl := v1.Group("/templates"); tpl.Use(JWTAuthMiddleware(jwtService))
		{ tpl.POST("", templateHandler.Create); tpl.GET("", templateHandler.List); tpl.GET("/builtin", templateHandler.ListBuiltin); tpl.GET("/:templateId", templateHandler.Get); tpl.PUT("/:templateId", templateHandler.Update); tpl.DELETE("/:templateId", templateHandler.Delete); tpl.POST("/:templateId/apply", templateHandler.Apply) }

		adm := v1.Group("/admin"); adm.Use(JWTAuthMiddleware(jwtService))
		{ adm.GET("/dashboard", metricsHandler.AdminDashboard); adm.GET("/traces", metricsHandler.Traces) }

		ag := v1.Group("/agent"); ag.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{ ag.GET("/activity", agentHandler.GetActivitySummary) }

		pl := v1.Group("/platform"); pl.Use(JWTAuthMiddleware(jwtService))
		{ pl.GET("/events", platformHandler.ListEvents); pl.POST("/events/replay", platformHandler.ReplayEvents); pl.GET("/plugins", platformHandler.ListPlugins); pl.POST("/plugins", platformHandler.RegisterPlugin); pl.PUT("/plugins/:pluginId", platformHandler.TogglePlugin) }

		// Phase 15: AI Intelligence - global
		ai := v1.Group("/ai"); ai.Use(JWTAuthMiddleware(jwtService))
		{
			ai.GET("/providers", intelligenceHandler.ListProviders)
			ai.POST("/query", intelligenceHandler.NLPQuery)
			ai.POST("/converse", intelligenceHandler.NLPConverse)
			ai.GET("/anomalies", intelligenceHandler.ListAnomalies)
			ai.PUT("/anomalies/:anomalyId/acknowledge", intelligenceHandler.AcknowledgeAnomaly)
			ai.PUT("/anomalies/:anomalyId/resolve", intelligenceHandler.ResolveAnomaly)
			ai.POST("/rules", intelligenceHandler.CreateAlertRule); ai.GET("/rules", intelligenceHandler.ListAlertRules)
			ai.PUT("/rules/:ruleId", intelligenceHandler.UpdateAlertRule); ai.DELETE("/rules/:ruleId", intelligenceHandler.DeleteAlertRule)
			ai.POST("/drift/run-scheduled", intelligenceHandler.RunScheduledDriftChecks)
		}

		oauthPub := v1.Group("/oauth")
		{ oauthPub.POST("/token", oauthHandler.Token); oauthPub.GET("/userinfo", oauthHandler.UserInfo); oauthPub.POST("/revoke", oauthHandler.Revoke); oauthPub.GET("/.well-known/jwks.json", oauthHandler.JWKS) }

		oauthAuth := v1.Group("/oauth"); oauthAuth.Use(JWTAuthMiddleware(jwtService))
		{ oauthAuth.GET("/authorize", oauthHandler.Authorize); oauthAuth.POST("/clients", oauthHandler.RegisterClient); oauthAuth.GET("/clients", oauthHandler.ListClients); oauthAuth.DELETE("/clients/:clientId", oauthHandler.DeleteClient) }

		mcpPub := v1.Group("/mcp")
		{ mcpPub.GET("/servers/public", mcpHubHandler.ListPublicServers) }

		mcp := v1.Group("/mcp"); mcp.Use(JWTAuthMiddleware(jwtService))
		{
			mcp.POST("/servers", mcpHubHandler.RegisterServer); mcp.GET("/servers", mcpHubHandler.ListMyServers); mcp.GET("/servers/:serverId", mcpHubHandler.GetServer); mcp.PUT("/servers/:serverId", mcpHubHandler.UpdateServer); mcp.DELETE("/servers/:serverId", mcpHubHandler.DeleteServer); mcp.POST("/servers/:serverId/rebuild", mcpHubHandler.RebuildServer)
			mcp.POST("/installations", mcpHubHandler.InstallServer); mcp.GET("/installations", mcpHubHandler.ListInstallations); mcp.PUT("/installations/:installId", mcpHubHandler.UpdateInstallation); mcp.DELETE("/installations/:installId", mcpHubHandler.UninstallServer)
			mcp.POST("/gateway", mcpGatewayHandler.HandleToolCall); mcp.GET("/gateway/tools", mcpGatewayHandler.ListTools); mcp.GET("/gateway/stats", mcpHubHandler.GetGatewayStats); mcp.GET("/config", mcpGatewayHandler.MCPConfig)
		}

		app := v1.Group("/applications"); app.Use(APIKeyAuthMiddleware(jwtService, apikeyRepo))
		{ app.POST("", applicationHandler.Create); app.GET("", applicationHandler.List); app.GET("/:appId", applicationHandler.Get); app.PUT("/:appId", applicationHandler.Update); app.DELETE("/:appId", applicationHandler.Delete); app.POST("/:appId/favorite", applicationHandler.ToggleFavorite) }
	}

	return r
}
