package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/santapong/KeepSave/backend/internal/api"
	"github.com/santapong/KeepSave/backend/internal/auth"
	"github.com/santapong/KeepSave/backend/internal/config"
	"github.com/santapong/KeepSave/backend/internal/crypto"
	"github.com/santapong/KeepSave/backend/internal/crypto/keyprovider"
	"github.com/santapong/KeepSave/backend/internal/events"
	"github.com/santapong/KeepSave/backend/internal/logging"
	"github.com/santapong/KeepSave/backend/internal/metrics"
	"github.com/santapong/KeepSave/backend/internal/plugins"
	"github.com/santapong/KeepSave/backend/internal/repository"
	"github.com/santapong/KeepSave/backend/internal/service"
	"github.com/santapong/KeepSave/backend/internal/tracing"
)

// version is the semantic-version string surfaced in logs and health checks.
const version = "1.1.0"

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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	masterKey, err := resolveMasterKey(ctx, cfg)
	cancel()
	if err != nil {
		logger.Error("failed to resolve master key", map[string]interface{}{"provider": cfg.KeyProvider, "error": err.Error()})
		os.Exit(1)
	}
	cryptoSvc, err := crypto.NewService(masterKey)
	if err != nil {
		logger.Error("failed to create crypto service", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	logger.Info("master key resolved", map[string]interface{}{"provider": cfg.KeyProvider})

	jwtService := auth.NewJWTService(cfg.JWTSecret)

	appMetrics := metrics.NewAppMetrics()
	tracer := tracing.NewTracer("keepsave-api")

	eventBus := events.NewBus(db, dialect)
	pluginRegistry := plugins.NewRegistry(db, dialect)

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

	ssoRepo := repository.NewSSORepository(db, dialect)
	complianceRepo := repository.NewComplianceRepository(db, dialect)
	backupRepo := repository.NewBackupRepository(db, dialect)
	accessPolicyRepo := repository.NewAccessPolicyRepository(db, dialect)
	oauthRepo := repository.NewOAuthRepository(db, dialect)
	mcpRepo := repository.NewMCPRepository(db, dialect)
	appRepo := repository.NewApplicationRepository(db, dialect)

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

	ssoService := service.NewSSOService(ssoRepo, cryptoSvc)
	complianceService := service.NewComplianceService(complianceRepo, auditRepo, orgRepo)
	backupService := service.NewBackupService(backupRepo, secretRepo, cryptoSvc)
	policyService := service.NewSecretPolicyService(db, dialect)

	leaseService := service.NewLeaseService(db, dialect)
	agentAnalyticsSvc := service.NewAgentAnalyticsService(db, dialect)

	oauthService := service.NewOAuthService(oauthRepo, userRepo)
	mcpService := service.NewMCPService(mcpRepo, secretRepo, projectRepo, envRepo)
	mcpBuilderService := service.NewMCPBuilderService(mcpRepo)

	appService := service.NewApplicationService(appRepo)

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

	intelligenceHandler := api.NewIntelligenceHandler(driftService, anomalyService, usageAnalyticsSvc, recommService, nlpService, aiMgr)

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

	tlsEnabled := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	logger.Info("starting server", map[string]interface{}{
		"version":  version,
		"port":     cfg.Port,
		"env":      cfg.Env,
		"tls":      tlsEnabled,
		"provider": cfg.KeyProvider,
	})

	if tlsEnabled {
		if cfg.TLSRedirect {
			go startHTTPRedirect(logger)
		}
		srv := &http.Server{
			Addr:              ":" + cfg.Port,
			Handler:           router,
			TLSConfig:         buildTLSConfig(cfg.TLSCipherSuites),
			ReadHeaderTimeout: 10 * time.Second,
		}
		if err := srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile); err != nil {
			logger.Error("failed to start TLS server", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
		return
	}

	if err := router.Run(":" + cfg.Port); err != nil {
		logger.Error("failed to start server", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
}

// resolveMasterKey sources the 32-byte master key from the configured
// provider. Env and Vault are wired inline because they require no extra
// Go-module dependencies; AWS/GCP KMS return a clear error until a
// follow-up commit wires the SDK adapters.
func resolveMasterKey(ctx context.Context, cfg *config.Config) ([]byte, error) {
	switch cfg.KeyProvider {
	case "env", "":
		if len(cfg.MasterKey) != 32 {
			return nil, fmt.Errorf("env provider selected but MasterKey is empty (check MASTER_KEY)")
		}
		return cfg.MasterKey, nil
	case "vault":
		p, err := keyprovider.NewVaultProvider(nil, cfg.VaultAddr, cfg.VaultToken, cfg.VaultKeyName, cfg.VaultCiphertext)
		if err != nil {
			return nil, err
		}
		return p.GetMasterKey(ctx)
	case "awskms", "gcpkms":
		return nil, fmt.Errorf("KEEPSAVE_KEY_PROVIDER=%s requires the SDK adapter; see docs/RUNBOOK.md and helm/keepsave/values.yaml", cfg.KeyProvider)
	default:
		return nil, fmt.Errorf("unknown KEEPSAVE_KEY_PROVIDER=%q", cfg.KeyProvider)
	}
}

// buildTLSConfig returns a TLS 1.2+ config. cipherList is a comma-separated
// list of IANA cipher names; when empty the Go default is used.
func buildTLSConfig(cipherList string) *tls.Config {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cipherList == "" {
		return cfg
	}
	names := map[string]uint16{}
	for _, s := range tls.CipherSuites() {
		names[s.Name] = s.ID
	}
	var ids []uint16
	for _, name := range strings.Split(cipherList, ",") {
		if id, ok := names[strings.TrimSpace(name)]; ok {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		cfg.CipherSuites = ids
	}
	return cfg
}

// startHTTPRedirect serves a plain-HTTP listener on :80 that 301-redirects
// every request to https://<host><path>.
func startHTTPRedirect(logger *logging.Logger) {
	srv := &http.Server{
		Addr: ":80",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("http redirect listener exited", map[string]interface{}{"error": err.Error()})
	}
}
