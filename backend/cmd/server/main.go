package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	"github.com/santapong/KeepSave/backend/internal/version"
)

// shutdownGracePeriod bounds how long the server waits for in-flight
// requests to complete after receiving SIGTERM/SIGINT. Kubernetes default
// terminationGracePeriodSeconds is 30s; we match it.
const shutdownGracePeriod = 30 * time.Second

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
	// RS256/JWKS (ADR-0008): load or self-heal the signing key and switch token
	// signing to RS256. JWT_ALG_VERIFY controls accepted verification algs during
	// the HS256->RS256 cutover (default "RS256,HS256"; set "RS256" once HS256
	// tokens have aged out).
	jwtKeystore, err := auth.LoadOrInit(cryptoSvc, repository.NewJWTKeyRepository(db, dialect))
	if err != nil {
		logger.Error("failed to load JWT keystore", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}
	jwtService.EnableRS256(jwtKeystore, parseAlgVerify(os.Getenv("JWT_ALG_VERIFY")))
	logger.Info("JWT signing initialized", map[string]interface{}{"alg": "RS256", "jwks": true})

	appMetrics := metrics.NewAppMetrics()
	// Surface audit-emission outcomes to metrics (A-06) without coupling the
	// service layer to the metrics package.
	service.SetAuditObserver(func(action string, ok bool) {
		appMetrics.AuditEventsTotal.Inc(action)
		if !ok {
			appMetrics.AuditEmitFailures.Inc(action)
		}
	})
	tracer := tracing.NewTracer("keepsave-api")

	eventBus := events.NewBus(db, dialect)
	pluginRegistry := plugins.NewRegistry(db, dialect)

	userRepo := repository.NewUserRepository(db, dialect)
	projectRepo := repository.NewProjectRepository(db, dialect)
	envRepo := repository.NewEnvironmentRepository(db, dialect)
	secretRepo := repository.NewSecretRepository(db, dialect)
	apikeyRepo := repository.NewAPIKeyRepository(db, dialect)
	auditRepo := repository.NewAuditRepository(db, dialect)
	// Enable the tamper-evident audit hash chain (ADR-0019). The key is derived
	// from the master key and never leaves internal/crypto.
	auditRepo.SetChainKey(cryptoSvc.DeriveAuditChainKey())
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

	attemptsRepo := repository.NewAuthAttemptsRepository(db, dialect)
	authService := service.NewAuthService(userRepo, attemptsRepo, auditRepo, jwtService)
	projectService := service.NewProjectService(projectRepo, envRepo, auditRepo, cryptoSvc)
	secretService := service.NewSecretService(secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	apikeyService := service.NewAPIKeyService(apikeyRepo, projectRepo, auditRepo)
	promotionService := service.NewPromotionService(promotionRepo, secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	keyRotationService := service.NewKeyRotationService(projectRepo, secretRepo, envRepo, auditRepo, cryptoSvc)
	webhookService := service.NewWebhookService(auditRepo)
	orgService := service.NewOrganizationService(orgRepo, auditRepo)
	templateService := service.NewTemplateService(templateRepo, secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	envFileService := service.NewEnvFileService(secretRepo, projectRepo, envRepo, auditRepo, cryptoSvc)
	depService := service.NewDependencyService(depRepo, secretRepo, projectRepo, envRepo, cryptoSvc)

	ssoService := service.NewSSOService(ssoRepo, orgRepo, auditRepo, cryptoSvc)
	complianceService := service.NewComplianceService(complianceRepo, auditRepo, orgRepo)
	backupService := service.NewBackupService(backupRepo, secretRepo, auditRepo, cryptoSvc)
	policyService := service.NewSecretPolicyService(db, dialect, auditRepo)

	leaseService := service.NewLeaseService(db, dialect, auditRepo)
	agentAnalyticsSvc := service.NewAgentAnalyticsService(db, dialect)

	oauthService := service.NewOAuthService(oauthRepo, userRepo, orgRepo, auditRepo)
	mcpService := service.NewMCPService(mcpRepo, secretRepo, projectRepo, envRepo, auditRepo)
	mcpBuilderService := service.NewMCPBuilderService(mcpRepo)

	appService := service.NewApplicationService(appRepo, auditRepo)

	aiMgr := service.NewAIProviderManager()
	if aiMgr.HasProvider() {
		logger.Info("AI providers initialized", map[string]interface{}{"count": len(aiMgr.ListProviders())})
	} else {
		logger.Info("no AI providers configured (Phase 15 features will use fallback mode)", nil)
	}
	driftService := service.NewDriftService(db, dialect, secretRepo, projectRepo, envRepo, cryptoSvc, aiMgr)
	anomalyService := service.NewAnomalyService(db, dialect, aiMgr, auditRepo)
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
	oauthHandler := api.NewOAuthHandler(oauthService, jwtKeystore)
	mcpHubHandler := api.NewMCPHubHandler(mcpService, mcpBuilderService)
	mcpGatewayHandler := api.NewMCPGatewayHandler(mcpService, mcpBuilderService, mcpRepo, secretRepo, projectRepo, envRepo, cryptoSvc)
	applicationHandler := api.NewApplicationHandler(appService)

	intelligenceHandler := api.NewIntelligenceHandler(driftService, anomalyService, usageAnalyticsSvc, recommService, nlpService, aiMgr)
	// ADR-0006: embed widget origin allow-list.
	embedHandler := api.NewEmbedHandler(projectService)

	if !cfg.PromotionsEnabled {
		logger.Info("promotions disabled by kill switch (KEEPSAVE_PROMOTIONS_ENABLED=false); /promote and /approve will return 503", nil)
	}
	if len(cfg.PlatformAdminEmails) == 0 {
		logger.Warn("KEEPSAVE_PLATFORM_ADMIN_EMAILS is empty; /admin endpoints will reject all callers (fail-closed)", nil)
	}

	router := api.SetupRouter(
		cfg.CORSOrigins,
		cfg.PromotionsEnabled,
		cfg.PlatformAdminEmails,
		jwtService,
		apikeyRepo,
		projectRepo,
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
		embedHandler,
		appMetrics,
		tracer,
		db,
		logger,
	)

	// Background workers (pruner, etc.) share a context that the signal
	// handler cancels at shutdown so they exit cleanly with the HTTP server.
	bgCtx, cancelBackground := context.WithCancel(context.Background())
	defer cancelBackground()

	// Audit-log retention: delete rows older than AUDIT_LOG_RETENTION_DAYS
	// once on startup and every 24h thereafter. The goroutine exits when
	// the context is cancelled or when retention is disabled (days <= 0).
	go startAuditLogPruner(bgCtx, logger, auditRepo, cfg.AuditLogRetentionDays)

	// DB pool gauges (audit B-L1). Polled every 15s; exits on shutdown.
	go metrics.StartDBPoolUpdater(bgCtx, db, appMetrics)

	tlsEnabled := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	logger.Info("starting server", map[string]interface{}{
		"version":  version.Version,
		"port":     cfg.Port,
		"env":      cfg.Env,
		"tls":      tlsEnabled,
		"provider": cfg.KeyProvider,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// Slowloris / slow-client protection (INF-3). The API has no streaming
		// (SSE) endpoints, so a bounded WriteTimeout is safe.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if tlsEnabled {
		srv.TLSConfig = buildTLSConfig(cfg.TLSCipherSuites)
		if cfg.TLSRedirect {
			go startHTTPRedirect(bgCtx, logger)
		}
	}

	// Run the listener in a goroutine so the main goroutine can wait on a
	// shutdown signal. http.ErrServerClosed is the expected return after
	// srv.Shutdown; anything else is a startup failure.
	serverErr := make(chan error, 1)
	go func() {
		var err error
		if tlsEnabled {
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("server exited with error", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
	case sig := <-quit:
		logger.Info("shutdown signal received", map[string]interface{}{"signal": sig.String()})
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", map[string]interface{}{"error": err.Error()})
			os.Exit(1)
		}
		cancelBackground()
		logger.Info("shutdown complete", nil)
	}
}

// parseAlgVerify parses the JWT_ALG_VERIFY allowlist (comma-separated). Empty
// defaults to the HS256->RS256 cutover set {RS256,HS256} (ADR-0008).
func parseAlgVerify(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil // EnableRS256 defaults to {"RS256","HS256"}
	}
	var algs []string
	for _, a := range strings.Split(raw, ",") {
		if a = strings.TrimSpace(a); a != "" {
			algs = append(algs, a)
		}
	}
	return algs
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
		// Deferred per ADR-0016 / DEPLOYMENT_PLAN.md: this round wires
		// Vault only. AWS/GCP adapters require pulling in their SDKs and
		// are tracked as FOLLOWUPS #1. Use KEEPSAVE_KEY_PROVIDER=vault for
		// UAT and early PROD; revisit when the production cloud is fixed.
		return nil, fmt.Errorf("KEEPSAVE_KEY_PROVIDER=%s is not wired in this build (deferred per ADR-0016, tracked as FOLLOWUPS #1); use KEEPSAVE_KEY_PROVIDER=vault or env", cfg.KeyProvider)
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
// every request to https://<host><path>. The listener honors ctx so a
// SIGTERM tears it down together with the main server - per audit Phase 2
// recheck NEW-2 (consistency with the pruner-goroutine pattern).
func startHTTPRedirect(ctx context.Context, logger *logging.Logger) {
	srv := &http.Server{
		Addr: ":80",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http redirect listener exited", map[string]interface{}{"error": err.Error()})
		}
		close(done)
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	<-done
}

// startAuditLogPruner deletes audit_log rows older than retentionDays on
// startup and once every 24 hours thereafter. It exits when ctx is cancelled
// (SIGTERM/SIGINT shutdown) or when retention is disabled (days <= 0).
func startAuditLogPruner(ctx context.Context, logger *logging.Logger, repo *repository.AuditRepository, retentionDays int) {
	if retentionDays <= 0 {
		logger.Info("audit-log pruner disabled", map[string]interface{}{"retention_days": retentionDays})
		return
	}
	logger.Info("audit-log pruner started", map[string]interface{}{
		"retention_days": retentionDays,
		"interval":       "24h",
	})

	prune := func() {
		deleted, err := repo.DeleteOlderThan(retentionDays)
		if err != nil {
			logger.Error("audit-log prune failed", map[string]interface{}{"error": err.Error()})
			return
		}
		if deleted > 0 {
			logger.Info("audit-log pruner deleted rows", map[string]interface{}{
				"deleted":        deleted,
				"retention_days": retentionDays,
			})
		}
	}

	// Run once immediately so a freshly-started server applies retention
	// without waiting 24 hours for the first tick.
	prune()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("audit-log pruner stopping", nil)
			return
		case <-ticker.C:
			prune()
		}
	}
}
