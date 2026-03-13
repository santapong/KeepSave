package main

import (
	"os"

	"github.com/santapong/KeepSave/backend/internal/api"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/logging"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
)

func main() {
	logger := logging.NewLogger(os.Stdout, logging.LevelInfo)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	if err := repository.RunMigrations(db, "migrations"); err != nil {
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

	// Repositories
	userRepo := repository.NewUserRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	envRepo := repository.NewEnvironmentRepository(db)
	secretRepo := repository.NewSecretRepository(db)
	apikeyRepo := repository.NewAPIKeyRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	promotionRepo := repository.NewPromotionRepository(db)
	versionRepo := repository.NewSecretVersionRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	depRepo := repository.NewDependencyRepository(db)

	// Services
	authService := service.NewAuthService(userRepo, jwtService)
	projectService := service.NewProjectService(projectRepo, envRepo, cryptoSvc)
	secretService := service.NewSecretService(secretRepo, projectRepo, envRepo, cryptoSvc)
	apikeyService := service.NewAPIKeyService(apikeyRepo)
	promotionService := service.NewPromotionService(promotionRepo, secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	keyRotationService := service.NewKeyRotationService(projectRepo, secretRepo, envRepo, cryptoSvc)
	webhookService := service.NewWebhookService()
	orgService := service.NewOrganizationService(orgRepo)
	templateService := service.NewTemplateService(templateRepo, secretRepo, projectRepo, envRepo, cryptoSvc)
	envFileService := service.NewEnvFileService(secretRepo, projectRepo, envRepo, cryptoSvc)
	depService := service.NewDependencyService(depRepo, secretRepo, projectRepo, envRepo, cryptoSvc)

	// Handlers
	authHandler := api.NewAuthHandler(authService)
	projectHandler := api.NewProjectHandler(projectService)
	secretHandler := api.NewSecretHandler(secretService)
	apikeyHandler := api.NewAPIKeyHandler(apikeyService)
	promotionHandler := api.NewPromotionHandler(promotionService)
	keyRotationHandler := api.NewKeyRotationHandler(keyRotationService)
	webhookHandler := api.NewWebhookHandler(webhookService)
	versionHandler := api.NewVersionHandler(versionRepo, secretRepo, projectRepo, cryptoSvc)
	healthHandler := api.NewHealthHandler(db)
	orgHandler := api.NewOrganizationHandler(orgService)
	templateHandler := api.NewTemplateHandler(templateService)
	envFileHandler := api.NewEnvFileHandler(envFileService)
	depHandler := api.NewDependencyHandler(depService)

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
		db,
		logger,
	)

	logger.Info("starting server", map[string]interface{}{"port": cfg.Port})
	if err := router.Run(":" + cfg.Port); err != nil {
		logger.Error("failed to start server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}
