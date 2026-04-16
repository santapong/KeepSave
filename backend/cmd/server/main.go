package main

import (
	"os"

	"github.com/santapong/KeepSave/backend/internal/api"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/events"
	"github.com/santapong/KeepSave/backend/internal/logging"
	"github.com/santapong/KeepSave/backend/internal/metrics"
	"github.com/santapong/KeepSave/backend/internal/plugins"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/tracing"
)

func main() {
	logger := logging.NewLogger(os.Stdout, logging.LevelInfo)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	db, dialect, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("connected to database", map[string]interface{}{"type": string(dialect.DBType())})

	if err := repository.RunMigrations(db, dialect, "migrations"); err != nil {
		logger.Error("failed to run migrations", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	logger.Info("migrations completed successfully", nil)

	cryptoSvc, err := crypto.NewService(cfg.MasterKey)
	if err != nil {
		logger.Error("failed to create crypto service", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	jwtService := auth.NewJWTService(cfg.JWTSecret)

	// Phase 7: Observability
	appMetrics := metrics.NewAppMetrics()
	tracer := tracing.NewTracer("keepsave-api")

	// Phase 12: Event bus and plugin registry
	eventBus := events.NewBus(db, dialect)
	pluginRegistry := plugins.NewRegistry(db, dialect)

	// Repositories
	userRepo := repository.NewUserRepository(db, dialect)
	projectRepo := repository.NewProjectRepository(db, dialect)
	envRepo := repository.NewEnvironmentRepository(db, dialect)
	secretRepo := repository.NewSecretRepository(db, dialect)
	apikeyRepo := repository.NewAPIKeyRepository(db, dialect)
	auditRepo := repository.NewAuditRepository(db, dialect)
	promotionRepo := repository.NewPromotionRepository(db, dialect)
	_ = repository.NewSecretVersionRepository(db, dialect)
	orgRepo := repository.NewOrganizationRepository(db, dialect)
	templateRepo := repository.NewTemplateRepository(db, dialect)
	depRepo := repository.NewDependencyRepository(db, dialect)

	// Phase 9: Enterprise repositories
	ssoRepo := repository.NewSSORepository(db, dialect)
	complianceRepo := repository.NewComplianceRepository(db, dialect)
	backupRepo := repository.NewBackupRepository(db, dialect)

	// Phase 12: Platform repositories
	accessPolicyRepo := repository.NewAccessPolicyRepository(db, dialect)

	// Phase 13: OAuth & MCP repositories
	oauthRepo := repository.NewOAuthRepository(db, dialect)
	mcpRepo := repository.NewMCPRepository(db, dialect)

	// Phase 14: Application Dashboard repository
	appRepo := repository.NewApplicationRepository(db, dialect)

	// Services
	authService := service.NewAuthService(userRepo, jwtService)
	projectService := service.NewProjectService(projectRepo, envRepo, cryptoSvc)
	secretService := service.NewSecretService(secretRepo, projectRepo, envRepo, cryptoSvc)
	apikeyService := service.NewAPIKeyService(apikeyRepo, projectRepo)
	promotionService := service.NewPromotionService(promotionRepo, secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	keyRotationService := service.NewKeyRotationService(projectRepo, secretRepo, envRepo, cryptoSvc)
	webhookService := service.NewWebhookService()
	orgService := service.NewOrganizationService(orgRepo)
	templateService := service.NewTemplateService(templateRepo, secretRepo, projectRepo, envRepo, cryptoSvc)
	envFileService := service.NewEnvFileService(secretRepo, projectRepo, envRepo, cryptoSvc)
	depService := service.NewDependencyService(depRepo, secretRepo, projectRepo, envRepo, cryptoSvc)

	// Phase 9: Enterprise services
	ssoService := service.NewSSOService(ssoRepo, cryptoSvc)
	complianceService := service.NewComplianceService(complianceRepo, auditRepo, orgRepo)
	backupService := service.NewBackupService(backupRepo, secretRepo, cryptoSvc)
	policyService := service.NewSecretPolicyService(db, dialect)

	// Phase 11: Agent services
	leaseService := service.NewLeaseService(db, dialect)
	agentAnalyticsSvc := service.NewAgentAnalyticsService(db, dialect)

	// Phase 13: OAuth & MCP services
	oauthService := service.NewOAuthService(oauthRepo, userRepo)
	mcpService := service.NewMCPService(mcpRepo, secretRepo, projectRepo, envRepo)
	mcpBuilderService := service.NewMCPBuilderService(mcpRepo)

	// Phase 14: Application Dashboard service
	appService := service.NewApplicationService(appRepo)

	// Phase 15: AI Intelligence services
	aiMgr := service.NewAIProviderManager()
	if aiMgr.HasProvider() {
		logger.Info("AI providers initialized", map[string]interface{}{"count": len(aiMgr.ListProviders())})
	} else {
		logger.Info("no AI providers configured (Phase 15 features will use fallback mode)", nil)
	}
	driftService := service.NewDriftService(db, dialect, secretRepo, projectRepo, envRepo, cryptoSvc, aiMgr)
	anomalyService := service.NewAnomalyService(db, dialect, aiMgr)
	usageAnalyticsSvc := service.NewUsageAnalyticsService(db, dialect)
	recommService := service.NewRecommendationService(db, dialect, secretRepo, projectRepo, envRepo, cryptoSvc, aiMgr)
	nlpService := service.NewNLPQueryService(db, dialect, projectRepo, envRepo, secretRepo, aiMgr)

	// Handlers
	authHandler := api.NewAuthHandler(authService)
	projectHandler := api.NewProjectHandler(projectService)
	secretHandler := api.NewSecretHandler(secretService)
	apikeyHandler := api.NewAPIKeyHandler(apikeyService)
	promotionHandler := api.NewPromotionHandler(promotionService)
	keyRotationHandler := api.NewKeyRotationHandler(keyRotationService)
	webhookHandler := api.NewWebhookHandler(webhookService)
	versionHandler := api.NewVersionHandler(repository.NewSecretVersionRepository(db, dialect), secretRepo, projectRepo, cryptoSvc)
	healthHandler := api.NewHealthHandler(db)
	orgHandler := api.NewOrganizationHandler(orgService)
	templateHandler := api.NewTemplateHandler(templateService)
	envFileHandler := api.NewEnvFileHandler(envFileService)
	depHandler := api.NewDependencyHandler(depService)

	metricsHandler := api.NewMetricsHandler(appMetrics, tracer)
	openAPIHandler := api.NewOpenAPIHandler()
	enterpriseHandler := api.NewEnterpriseHandler(ssoService, complianceService, backupService, policyService)
	agentHandler := api.NewAgentHandler(leaseService, agentAnalyticsSvc)
	platformHandler := api.NewPlatformHandler(eventBus, pluginRegistry, accessPolicyRepo)
	oauthHandler := api.NewOAuthHandler(oauthService)
	mcpHubHandler := api.NewMCPHubHandler(mcpService, mcpBuilderService)
	mcpGatewayHandler := api.NewMCPGatewayHandler(mcpService, mcpBuilderService, mcpRepo, secretRepo, projectRepo, envRepo, cryptoSvc)
	applicationHandler := api.NewApplicationHandler(appService)

	// Phase 15: Intelligence handler
	intelligenceHandler := api.NewIntelligenceHandler(driftService, anomalyService, usageAnalyticsSvc, recommService, nlpService, aiMgr)

	// Router
	router := api.SetupRouter(
		cfg.CORSOrigins,
		jwtService,
		apikeyRepo,
		authHandler,
		projectHandler,
		secretHandler,
		apikeyHandler,
		promotionHandler,
		keyRotationHandler,
		webhookHandler,
		versionHandler,
		healthHandler,
		orgHandler,
		templateHandler,
		envFileHandler,
		depHandler,
		metricsHandler,
		enterpriseHandler,
		agentHandler,
		platformHandler,
		openAPIHandler,
		oauthHandler,
		mcpHubHandler,
		mcpGatewayHandler,
		applicationHandler,
		intelligenceHandler,
		appMetrics,
		tracer,
		db,
		logger,
	)

	logger.Info("starting server", map[string]interface{}{"port": cfg.Port})
	if err := router.Run(":" + cfg.Port); err != nil {
		logger.Error("failed to start server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
