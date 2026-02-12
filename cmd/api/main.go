// Package main is the entry point for the API server.
// It initializes all dependencies and starts the HTTP server with graceful shutdown.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/knid/kargonet/internal/api"
	"github.com/knid/kargonet/internal/api/handler"
	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/config"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	redisrepo "github.com/knid/kargonet/internal/repository/redis"
	"github.com/knid/kargonet/internal/payment/paytr"
	"github.com/knid/kargonet/internal/repository/postgres"
	"github.com/knid/kargonet/internal/service/auth"
	"github.com/knid/kargonet/internal/service/billing"
	"github.com/knid/kargonet/internal/service/organization"
	providerservice "github.com/knid/kargonet/internal/service/provider"
	"github.com/knid/kargonet/internal/service/quote"
	"github.com/knid/kargonet/internal/service/shipment"
	"github.com/knid/kargonet/internal/service/tracking"
	"github.com/knid/kargonet/internal/service/webhook"

	// Import all provider adapters to register them with the global factory.
	// This ensures all implemented carrier adapters are available for use.
	_ "github.com/knid/kargonet/internal/provider/all"
)

// Build information (set via ldflags).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Run the application
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize logger
	logger := observability.NewLogger(observability.LoggerConfig{
		Level:       observability.LogLevel(cfg.App.LogLevel),
		Format:      observability.LogFormat(cfg.App.LogFormat),
		Output:      os.Stdout,
		ServiceName: cfg.App.Name,
		Version:     Version,
		Environment: string(cfg.App.Env),
		AddSource:   cfg.App.Debug,
	})
	observability.SetGlobalLogger(logger)

	logger.Info("starting API server",
		"version", Version,
		"commit", Commit,
		"build_time", BuildTime,
		"environment", cfg.App.Env,
	)

	// Set build info for health handler
	handler.Version = Version
	handler.Commit = Commit
	handler.BuildTime = BuildTime

	// Initialize metrics
	var metrics *observability.Metrics
	if cfg.Observability.MetricsEnabled {
		metrics = observability.NewMetrics(observability.MetricsConfig{
			Enabled:     true,
			Namespace:   "shipping",
			Subsystem:   "aggregator",
			ServiceName: cfg.App.Name,
			Environment: string(cfg.App.Env),
		})
		logger.Info("metrics enabled", "port", cfg.Observability.MetricsPort)
	}

	// Initialize tracer
	var tracer *observability.Tracer
	if cfg.Observability.OTELEnabled {
		var err error
		tracer, err = observability.NewTracer(context.Background(), observability.TracerConfig{
			Enabled:     true,
			ServiceName: cfg.Observability.OTELServiceName,
			Version:     Version,
			Environment: string(cfg.App.Env),
			Endpoint:    cfg.Observability.OTELEndpoint,
			Insecure:    cfg.App.Env != config.EnvProduction,
		})
		if err != nil {
			logger.Warn("failed to initialize tracer, continuing without tracing", "error", err)
		} else {
			defer func() {
				if err := tracer.Shutdown(context.Background()); err != nil {
					logger.Error("failed to shutdown tracer", "error", err)
				}
			}()
			logger.Info("tracing enabled", "endpoint", cfg.Observability.OTELEndpoint)
		}
	}

	// Initialize database connection pool
	dbPool, err := initDatabase(context.Background(), cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer dbPool.Close()
	logger.Info("database connection established")

	// Initialize Redis client and cache
	redisClient, redisCache, rateLimiter, err := initRedis(context.Background(), cfg, logger)
	if err != nil {
		// Redis is optional for basic operation
		logger.Warn("failed to initialize Redis, continuing without caching", "error", err)
	} else {
		defer redisClient.Close()
		logger.Info("redis connection established")
	}

	// Initialize database queries
	queries := db.New(dbPool)
	logger.Info("database queries initialized")

	// Initialize session service for authentication
	sessionService := auth.NewSessionService(queries)
	logger.Info("session service initialized")

	// Initialize session validator adapter for middleware
	sessionValidator := auth.NewSessionValidatorAdapter(queries)
	logger.Info("session validator initialized")

	// Initialize admin repositories (US10: Admin Tools) — early init for provider API logging
	auditLogRepo := postgres.NewAuditLogRepository(dbPool, queries)
	providerAPILogRepo := postgres.NewProviderAPILogRepository(dbPool, queries)
	logger.Info("admin repositories initialized")

	// Initialize provider adapter pool using the global factory
	adapterFactory := provider.GlobalFactory()

	// Wire provider API request logging: every SOAP call made by adapters
	// is persisted to the provider_api_logs table for full observability.
	adapterFactory.SetRequestLogger(func(ctx context.Context, providerCode, operation, endpoint string, reqBody, respBody []byte, status int, latencyMs int64, success bool, callErr error) {
		// Extract organization ID from request context before goroutine
		var orgID *uuid.UUID
		if authOrgID := middleware.GetOrganizationID(ctx); authOrgID != uuid.Nil {
			orgID = &authOrgID
		}

		var respStatus *int32
		if status > 0 {
			s := int32(status)
			respStatus = &s
		}

		var reqPreview, respPreview *string
		if len(reqBody) > 0 {
			s := string(reqBody)
			if len(s) > 4000 {
				s = s[:4000]
			}
			reqPreview = &s
		}
		if len(respBody) > 0 {
			s := string(respBody)
			if len(s) > 4000 {
				s = s[:4000]
			}
			respPreview = &s
		}

		var errCode, errMsg *string
		if callErr != nil {
			ec := "PROVIDER_ERROR"
			em := callErr.Error()
			errCode = &ec
			errMsg = &em
		}

		// Persist asynchronously so it doesn't slow down the provider call
		go func() {
			_, err := providerAPILogRepo.Create(context.Background(), &postgres.CreateProviderAPILogParams{
				OrganizationID:      orgID,
				ProviderCode:        providerCode,
				Operation:           operation,
				RequestMethod:       "POST",
				RequestURL:          endpoint,
				RequestBodyPreview:  reqPreview,
				ResponseStatus:      respStatus,
				ResponseBodyPreview: respPreview,
				LatencyMs:           int32(latencyMs),
				Success:             success,
				ErrorCode:           errCode,
				ErrorMessage:        errMsg,
			})
			if err != nil {
				logger.Error("failed to persist provider API log", "error", err, "provider", providerCode, "operation", operation)
			}
		}()
	})

	// Configure Aras adapter endpoint from ARAS_WSDL_URL if set
	if cfg.Providers.Aras.WSDLURL != "" {
		adapterFactory.SetConfig(domain.ProviderCodeAras, provider.AdapterConfig{
			BaseURL: cfg.Providers.Aras.WSDLURL,
			Timeout: 30 * time.Second,
		})
		logger.Info("aras adapter endpoint configured", "url", cfg.Providers.Aras.WSDLURL)
	}

	adapterPool := provider.NewAdapterPool(adapterFactory, slog.Default())
	logger.Info("provider adapter pool initialized (with API logging)")

	// Initialize billing services
	walletService := billing.NewWalletService(queries, logger)
	ledgerService := billing.NewLedgerService(queries, logger)
	logger.Info("billing services initialized")

	// Build platform credentials for marketplace pricing and shipment creation
	platformCreds := make(map[domain.ProviderCode]*domain.ProviderCredentials)
	if cfg.Providers.Aras.Username != "" && cfg.Providers.Aras.Password != "" && cfg.Providers.Aras.CustomerCode != "" {
		platformCreds[domain.ProviderCodeAras] = &domain.ProviderCredentials{
			Username:     cfg.Providers.Aras.Username,
			Password:     cfg.Providers.Aras.Password,
			CustomerCode: cfg.Providers.Aras.CustomerCode,
		}
		logger.Info("platform credentials configured for Aras")
	}
	if cfg.Features.YurticiEnabled {
		platformCreds[domain.ProviderCodeYurtici] = &domain.ProviderCredentials{
			Username: cfg.Providers.Yurtici.Username,
			Password: cfg.Providers.Yurtici.Password,
		}
		logger.Info("platform credentials configured for Yurtici")
	}

	// Initialize shipment service
	shipmentService := shipment.NewService(queries, adapterPool, platformCreds, ledgerService, walletService, logger)
	logger.Info("shipment service initialized")

	// Initialize organization service
	organizationService := organization.NewService(queries)
	logger.Info("organization service initialized")

	// Initialize quote service (US1: Multi-carrier rate shopping)
	var quoteService *quote.Service
	if len(platformCreds) > 0 {
		quoteService = quote.NewServiceWithPlatformCreds(queries, adapterPool, platformCreds, logger)
		logger.Info("quote service initialized with live marketplace pricing")
	} else {
		quoteService = quote.NewService(queries, logger)
		logger.Info("quote service initialized (no platform credentials, marketplace offers disabled)")
	}

	// Initialize tracking service (US5: Tracking Updates)
	trackingService := tracking.NewService(dbPool, queries, logger)
	logger.Info("tracking service initialized")

	// Initialize webhook services (US5: Webhooks)
	webhookPublisher := webhook.NewPublisher(dbPool, queries, logger)
	webhookDeliveryService := webhook.NewDeliveryService(dbPool, queries, logger, nil)
	logger.Info("webhook services initialized")

	// Initialize billing sub-services (COD, Topup, Refund)
	codService := billing.NewCODService(queries, ledgerService, logger)
	refundService := billing.NewRefundService(queries, ledgerService, logger)
	logger.Info("billing sub-services initialized (COD, refund)")

	// Initialize topup service (US4: Wallet Top-Up via PayTR)
	var paytrClient *paytr.Client
	if cfg.PayTR.MerchantID != "" {
		paytrClient = paytr.NewClient(paytr.Config{
			MerchantID:   cfg.PayTR.MerchantID,
			MerchantKey:  cfg.PayTR.MerchantKey,
			MerchantSalt: cfg.PayTR.MerchantSalt,
			CallbackURL:  cfg.PayTR.CallbackURL,
			OkURL:        cfg.PayTR.OkURL,
			FailURL:      cfg.PayTR.FailURL,
			TestMode:     cfg.PayTR.TestMode,
		}, logger)
		logger.Info("paytr client initialized")
	}
	topupService := billing.NewTopupService(queries, paytrClient, ledgerService, billing.DefaultTopupConfig(), logger)
	logger.Info("topup service initialized")

	// Initialize API key service (US6: API Key Management)
	apiKeyService := auth.NewAPIKeyService(queries)
	logger.Info("api key service initialized")

	// Initialize provider account services (US2: BYOA - Bring Your Own Account)
	var providerAccountService *providerservice.AccountService
	var providerVerificationService *providerservice.VerificationService
	if cfg.Features.BYOAEnabled && len(cfg.Security.EncryptionKey) == 32 {
		var err error
		providerAccountService, err = providerservice.NewAccountService(queries, cfg.Security.EncryptionKey, logger)
		if err != nil {
			logger.Warn("failed to initialize provider account service, BYOA feature disabled", "error", err)
		} else {
			providerVerificationService = providerservice.NewVerificationService(
				providerAccountService,
				adapterPool,
				queries,
				logger,
			)
			logger.Info("provider account services initialized (BYOA enabled)")

			// Upgrade quote service with BYOA support (keep platform creds if configured)
			if len(platformCreds) > 0 {
				quoteService = quote.NewServiceWithPlatformCredsAndBYOA(queries, adapterPool, providerAccountService, platformCreds, logger)
			} else {
				quoteService = quote.NewServiceWithBYOA(queries, adapterPool, providerAccountService, logger)
			}
			logger.Info("quote service upgraded with BYOA support")
		}
	} else if cfg.Features.BYOAEnabled {
		logger.Warn("BYOA feature enabled but ENCRYPTION_KEY not configured, provider accounts disabled")
	}

	// Create router configuration
	routerCfg := api.RouterConfig{
		Logger:                      logger,
		Metrics:                     metrics,
		Tracer:                      tracer,
		DBPool:                      dbPool,
		Queries:                     queries,
		RedisCache:                  redisCache,
		RateLimiter:                 rateLimiter,
		SessionService:              sessionService,
		SessionValidator:            sessionValidator,
		QuoteService:                quoteService,
		ShipmentService:             shipmentService,
		ProviderAccountService:      providerAccountService,
		ProviderVerificationService: providerVerificationService,
		WalletService:               walletService,
		LedgerService:               ledgerService,
		OrganizationService:         organizationService,
		TrackingService:             trackingService,
		WebhookPublisher:            webhookPublisher,
		WebhookDeliveryService:      webhookDeliveryService,
		CODService:                  codService,
		TopupService:                topupService,
		RefundService:               refundService,
		PayTRClient:                 paytrClient,
		APIKeyService:               apiKeyService,
		AuditLogRepository:          auditLogRepo,
		ProviderAPILogRepository:    providerAPILogRepo,
		CORSAllowedOrigins:          cfg.CORS.AllowedOrigins,
		RateLimitPerMinute:          cfg.RateLimit.RequestsPerSecond * 60,
		IdempotencyTTL:              cfg.Idempotency.TTL,
		Environment:                 string(cfg.App.Env),
	}

	// Create router
	router := api.NewRouter(routerCfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.APIAddr(),
		Handler:      router,
		ReadTimeout:  cfg.API.ReadTimeout,
		WriteTimeout: cfg.API.WriteTimeout,
		IdleTimeout:  cfg.API.IdleTimeout,
	}

	// Start metrics server if enabled
	var metricsServer *http.Server
	if cfg.Observability.MetricsEnabled && metrics != nil {
		metricsServer = startMetricsServer(cfg, metrics, logger)
	}

	// Channel to receive server errors
	serverErrors := make(chan error, 1)

	// Start HTTP server in goroutine
	go func() {
		logger.Info("server listening",
			"address", cfg.APIAddr(),
			"read_timeout", cfg.API.ReadTimeout,
			"write_timeout", cfg.API.WriteTimeout,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownTimeout)
	defer cancel()

	// Shutdown main server
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Shutdown metrics server if running
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			logger.Warn("metrics server shutdown error", "error", err)
		}
	}

	logger.Info("server stopped gracefully")
	return nil
}

// initDatabase initializes the PostgreSQL connection pool.
func initDatabase(ctx context.Context, cfg *config.Config, logger *observability.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.Database.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// initRedis initializes the Redis client, cache, and rate limiter.
func initRedis(ctx context.Context, cfg *config.Config, logger *observability.Logger) (*redis.Client, *redisrepo.Cache, *redisrepo.RateLimiter, error) {
	var opts *redis.Options
	var err error

	if cfg.Redis.URL != "" {
		opts, err = redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse Redis URL: %w", err)
		}
	} else {
		opts = &redis.Options{
			Addr:       cfg.Redis.Addr(),
			Password:   cfg.Redis.Password,
			DB:         cfg.Redis.DB,
			MaxRetries: cfg.Redis.MaxRetries,
			PoolSize:   cfg.Redis.PoolSize,
		}
	}

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	// Create cache using the existing client
	cache := redisrepo.NewCacheWithClient(client, redisrepo.CacheConfig{
		DefaultTTL: 5 * time.Minute,
		KeyPrefix:  "shipping:",
	}, slog.Default())

	// Create rate limiter
	rateLimiter := redisrepo.NewRateLimiter(client, redisrepo.RateLimitConfig{
		KeyPrefix:       "ratelimit:",
		DefaultLimit:    cfg.RateLimit.RequestsPerSecond,
		DefaultWindow:   time.Second,
		BurstMultiplier: 1.0, // Must be set, otherwise effectiveLimit becomes 0
	}, slog.Default())

	return client, cache, rateLimiter, nil
}

// startMetricsServer starts a separate HTTP server for Prometheus metrics.
func startMetricsServer(cfg *config.Config, metrics *observability.Metrics, logger *observability.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(cfg.Observability.MetricsPath, metrics.Handler())

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Observability.MetricsPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("metrics server listening",
			"address", srv.Addr,
			"path", cfg.Observability.MetricsPath,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server error", "error", err)
		}
	}()

	return srv
}
