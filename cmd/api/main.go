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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/knid/kargonet/internal/api"
	"github.com/knid/kargonet/internal/api/handler"
	"github.com/knid/kargonet/internal/config"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	redisrepo "github.com/knid/kargonet/internal/repository/redis"
	"github.com/knid/kargonet/internal/service/auth"
	"github.com/knid/kargonet/internal/service/billing"
	providerservice "github.com/knid/kargonet/internal/service/provider"
	"github.com/knid/kargonet/internal/service/shipment"

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

	// Initialize provider adapter pool using the global factory
	adapterFactory := provider.GlobalFactory()
	adapterPool := provider.NewAdapterPool(adapterFactory, slog.Default())
	logger.Info("provider adapter pool initialized")

	// Initialize billing services
	walletService := billing.NewWalletService(queries, logger)
	ledgerService := billing.NewLedgerService(queries, logger)
	logger.Info("billing services initialized")

	// Initialize shipment service
	shipmentService := shipment.NewService(queries, adapterPool, ledgerService, walletService, logger)
	logger.Info("shipment service initialized")

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
		ShipmentService:             shipmentService,
		ProviderAccountService:      providerAccountService,
		ProviderVerificationService: providerVerificationService,
		WalletService:               walletService,
		LedgerService:               ledgerService,
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
