// Package main is the entry point for the background worker.
// It initializes all dependencies and starts the Asynq worker with graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/e-port/shipping-aggregator/internal/config"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/provider"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	redisrepo "github.com/e-port/shipping-aggregator/internal/repository/redis"
	"github.com/e-port/shipping-aggregator/internal/service/tracking"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
	"github.com/e-port/shipping-aggregator/internal/worker"
	"github.com/e-port/shipping-aggregator/internal/worker/tasks"
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
		ServiceName: cfg.App.Name + "-worker",
		Version:     Version,
		Environment: string(cfg.App.Env),
		AddSource:   cfg.App.Debug,
	})
	observability.SetGlobalLogger(logger)

	logger.Info("starting background worker",
		"version", Version,
		"commit", Commit,
		"build_time", BuildTime,
		"environment", cfg.App.Env,
		"concurrency", cfg.Worker.Concurrency,
	)

	// Initialize metrics
	var metrics *observability.Metrics
	if cfg.Observability.MetricsEnabled {
		metrics = observability.NewMetrics(observability.MetricsConfig{
			Enabled:     true,
			Namespace:   "shipping",
			Subsystem:   "worker",
			ServiceName: cfg.App.Name + "-worker",
			Environment: string(cfg.App.Env),
		})
		logger.Info("metrics enabled")
	}

	// Initialize tracer
	var tracer *observability.Tracer
	if cfg.Observability.OTELEnabled {
		var err error
		tracer, err = observability.NewTracer(context.Background(), observability.TracerConfig{
			Enabled:     true,
			ServiceName: cfg.Observability.OTELServiceName + "-worker",
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

	// Initialize Redis client
	redisClient, err := initRedis(context.Background(), cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize Redis: %w", err)
	}
	defer redisClient.Close()
	logger.Info("redis connection established")

	// Create cache for task handlers
	cache := redisrepo.NewCacheWithClient(redisClient, redisrepo.CacheConfig{
		DefaultTTL: cfg.Idempotency.TTL,
		KeyPrefix:  "shipping:",
	}, slog.Default())

	// Create Asynq client for enqueueing tasks
	asynqClient, err := createAsynqClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create asynq client: %w", err)
	}
	defer asynqClient.Close()

	// Create worker server
	workerServer, err := worker.NewServer(cfg, worker.WithServerLogger(slog.Default()))
	if err != nil {
		return fmt.Errorf("failed to create worker server: %w", err)
	}

	// Create queries instance
	queries := db.New(dbPool)

	// Register task handlers
	registerTaskHandlers(workerServer.Mux(), &taskHandlerDeps{
		cfg:         cfg,
		logger:      logger,
		metrics:     metrics,
		tracer:      tracer,
		dbPool:      dbPool,
		queries:     queries,
		cache:       cache,
		asynqClient: asynqClient,
	})

	// Create scheduler for periodic tasks
	scheduler, err := worker.NewScheduler(cfg, worker.WithSchedulerLogger(slog.Default()))
	if err != nil {
		return fmt.Errorf("failed to create scheduler: %w", err)
	}

	// Register scheduled tasks
	if err := scheduler.RegisterTasks(); err != nil {
		return fmt.Errorf("failed to register scheduled tasks: %w", err)
	}

	// Channel to receive errors
	errChan := make(chan error, 2)

	// Start worker server in goroutine
	go func() {
		logger.Info("starting worker server...")
		if err := workerServer.Start(); err != nil {
			errChan <- fmt.Errorf("worker server error: %w", err)
		}
	}()

	// Start scheduler in goroutine
	go func() {
		logger.Info("starting scheduler...")
		if err := scheduler.Start(); err != nil {
			errChan <- fmt.Errorf("scheduler error: %w", err)
		}
	}()

	// Wait for interrupt signal or error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig.String())
	}

	// Graceful shutdown
	logger.Info("initiating graceful shutdown...")

	// Shutdown scheduler first (stops enqueueing new tasks)
	scheduler.Shutdown()
	logger.Info("scheduler stopped")

	// Shutdown worker server (waits for in-progress tasks)
	workerServer.Shutdown()
	logger.Info("worker server stopped")

	logger.Info("worker stopped gracefully")
	return nil
}

// taskHandlerDeps holds dependencies for task handlers.
type taskHandlerDeps struct {
	cfg         *config.Config
	logger      *observability.Logger
	metrics     *observability.Metrics
	tracer      *observability.Tracer
	dbPool      *pgxpool.Pool
	queries     *db.Queries
	cache       *redisrepo.Cache
	asynqClient *asynq.Client
}

// registerTaskHandlers registers all task handlers with the mux.
func registerTaskHandlers(mux *asynq.ServeMux, deps *taskHandlerDeps) {
	// Create services needed by handlers
	// Note: In a production system, these would be properly initialized with all dependencies
	trackingService := tracking.NewService(deps.dbPool, deps.queries, deps.logger)
	webhookPublisher := webhook.NewPublisher(deps.dbPool, deps.queries, deps.logger)
	adapterFactory := provider.NewAdapterFactory(slog.Default())
	adapterPool := provider.NewAdapterPool(adapterFactory, slog.Default())

	// Create task handlers
	webhookDeliveryHandler := tasks.NewWebhookDeliveryHandler(deps.dbPool, deps.queries, deps.logger)
	webhookRetryHandler := tasks.NewWebhookRetryHandler(deps.dbPool, deps.queries, deps.asynqClient, deps.logger)
	webhookDLQHandler := tasks.NewWebhookDLQHandler(deps.dbPool, deps.queries, deps.logger)
	trackingPollHandler := tasks.NewTrackingPollHandler(deps.dbPool, deps.queries, trackingService, webhookPublisher, adapterPool, deps.logger, deps.asynqClient)
	trackingUpdateHandler := tasks.NewTrackingUpdateHandler(deps.dbPool, deps.queries, trackingService, webhookPublisher, adapterPool, deps.logger)
	outboxPublisherHandler := tasks.NewOutboxPublisherHandler(deps.dbPool, deps.queries, deps.asynqClient, deps.logger)
	outboxCleanupHandler := tasks.NewOutboxCleanupHandler(deps.dbPool, deps.queries, deps.logger)
	idempotencyCleanupHandler := tasks.NewIdempotencyCleanupHandler(deps.dbPool, deps.queries, deps.logger)
	retentionCleanupHandler := tasks.NewRetentionCleanupHandler(deps.dbPool, deps.queries, deps.logger)
	quoteExpiryHandler := tasks.NewQuoteExpiryHandler(deps.dbPool, deps.queries, deps.logger)
	reconciliationHandler := tasks.NewReconciliationHandler(deps.dbPool, deps.queries, deps.logger)
	codSettlementHandler := tasks.NewCODSettlementHandler(deps.dbPool, deps.queries, deps.logger)

	// Register webhook handlers
	mux.HandleFunc(tasks.TypeWebhookDelivery, webhookDeliveryHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeWebhookRetry, webhookRetryHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeWebhookDLQ, webhookDLQHandler.ProcessTask)

	// Register tracking handlers
	mux.HandleFunc(tasks.TypeTrackingPoll, trackingPollHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeTrackingUpdate, trackingUpdateHandler.ProcessTask)

	// Register outbox handlers
	mux.HandleFunc(tasks.TypeOutboxPublish, outboxPublisherHandler.ProcessTask)

	// Register cleanup handlers
	mux.HandleFunc(tasks.TypeIdempotencyCleanup, idempotencyCleanupHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeRetentionCleanup, retentionCleanupHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeQuoteExpiry, quoteExpiryHandler.ProcessTask)

	// Register billing handlers
	mux.HandleFunc(tasks.TypeReconciliation, reconciliationHandler.ProcessTask)
	mux.HandleFunc(tasks.TypeCODSettlement, codSettlementHandler.ProcessTask)

	// Register provider health check handler (placeholder)
	mux.HandleFunc(tasks.TypeProviderHealthCheck, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseProviderHealthCheckPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse provider health check payload: %w", err)
		}
		deps.logger.Info("processing provider health check",
			"provider_code", payload.ProviderCode,
		)
		// TODO: Implement provider health check logic
		return nil
	})

	// Register API key expiry notification handler (placeholder)
	mux.HandleFunc(tasks.TypeAPIKeyExpiryNotification, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseAPIKeyExpiryNotificationPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse API key expiry notification payload: %w", err)
		}
		deps.logger.Info("processing API key expiry notification",
			"days_before_expiry", payload.DaysBeforeExpiry,
		)
		// TODO: Implement API key expiry notification logic
		return nil
	})

	// Register label generation handler (placeholder)
	mux.HandleFunc(tasks.TypeLabelGeneration, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseLabelGenerationPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse label generation payload: %w", err)
		}
		deps.logger.Info("processing label generation",
			"shipment_id", payload.ShipmentID,
		)
		// TODO: Implement label generation logic
		return nil
	})

	// Register outbox cleanup handler (internal task)
	mux.HandleFunc("outbox:cleanup", outboxCleanupHandler.ProcessTask)

	deps.logger.Info("all task handlers registered")
}

// createAsynqClient creates an Asynq client for enqueueing tasks.
func createAsynqClient(cfg *config.Config) (*asynq.Client, error) {
	var redisOpt asynq.RedisClientOpt

	if cfg.Redis.URL != "" {
		opts, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
		}
		redisOpt = asynq.RedisClientOpt{
			Addr:     opts.Addr,
			Password: opts.Password,
			DB:       opts.DB,
		}
	} else {
		redisOpt = asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		}
	}

	return asynq.NewClient(redisOpt), nil
}

// initDatabase initializes the PostgreSQL connection pool.
func initDatabase(ctx context.Context, cfg *config.Config, logger *observability.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure pool settings (smaller pool for worker)
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns / 2)
	if poolConfig.MaxConns < 5 {
		poolConfig.MaxConns = 5
	}
	poolConfig.MinConns = 2
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

// initRedis initializes the Redis client.
func initRedis(ctx context.Context, cfg *config.Config, logger *observability.Logger) (*redis.Client, error) {
	var opts *redis.Options
	var err error

	if cfg.Redis.URL != "" {
		opts, err = redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
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
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}
