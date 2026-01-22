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
	redisrepo "github.com/e-port/shipping-aggregator/internal/repository/redis"
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

	// Create worker server
	workerServer, err := worker.NewServer(cfg, worker.WithServerLogger(slog.Default()))
	if err != nil {
		return fmt.Errorf("failed to create worker server: %w", err)
	}

	// Register task handlers
	registerTaskHandlers(workerServer.Mux(), &taskHandlerDeps{
		cfg:     cfg,
		logger:  logger,
		metrics: metrics,
		tracer:  tracer,
		dbPool:  dbPool,
		cache:   cache,
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
	cfg     *config.Config
	logger  *observability.Logger
	metrics *observability.Metrics
	tracer  *observability.Tracer
	dbPool  *pgxpool.Pool
	cache   *redisrepo.Cache
}

// registerTaskHandlers registers all task handlers with the mux.
func registerTaskHandlers(mux *asynq.ServeMux, deps *taskHandlerDeps) {
	// Webhook delivery task
	mux.HandleFunc(tasks.TypeWebhookDelivery, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseWebhookDeliveryPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse webhook delivery payload: %w", err)
		}
		deps.logger.Info("processing webhook delivery",
			"delivery_id", payload.DeliveryID,
		)
		// TODO: Implement webhook delivery logic
		return nil
	})

	// Webhook retry task
	mux.HandleFunc(tasks.TypeWebhookRetry, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseWebhookRetryPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse webhook retry payload: %w", err)
		}
		deps.logger.Info("processing webhook retry",
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement webhook retry logic
		return nil
	})

	// Tracking poll task
	mux.HandleFunc(tasks.TypeTrackingPoll, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseTrackingPollPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse tracking poll payload: %w", err)
		}
		deps.logger.Info("processing tracking poll",
			"provider_code", payload.ProviderCode,
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement tracking poll logic
		return nil
	})

	// Tracking update task
	mux.HandleFunc(tasks.TypeTrackingUpdate, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseTrackingUpdatePayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse tracking update payload: %w", err)
		}
		deps.logger.Info("processing tracking update",
			"shipment_id", payload.ShipmentID,
			"provider_code", payload.ProviderCode,
		)
		// TODO: Implement tracking update logic
		return nil
	})

	// Outbox publish task
	mux.HandleFunc(tasks.TypeOutboxPublish, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseOutboxPublishPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse outbox publish payload: %w", err)
		}
		deps.logger.Debug("processing outbox publish",
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement outbox publish logic
		return nil
	})

	// Idempotency cleanup task
	mux.HandleFunc(tasks.TypeIdempotencyCleanup, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseIdempotencyCleanupPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse idempotency cleanup payload: %w", err)
		}
		deps.logger.Info("processing idempotency cleanup",
			"max_age", payload.MaxAge,
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement idempotency cleanup logic
		return nil
	})

	// Retention cleanup task
	mux.HandleFunc(tasks.TypeRetentionCleanup, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseRetentionCleanupPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse retention cleanup payload: %w", err)
		}
		deps.logger.Info("processing retention cleanup",
			"table_name", payload.TableName,
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement retention cleanup logic
		return nil
	})

	// Reconciliation task
	mux.HandleFunc(tasks.TypeReconciliation, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseReconciliationPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse reconciliation payload: %w", err)
		}
		deps.logger.Info("processing reconciliation",
			"date", payload.Date,
			"full_check", payload.FullCheck,
		)
		// TODO: Implement reconciliation logic
		return nil
	})

	// Provider health check task
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

	// Quote expiry task
	mux.HandleFunc(tasks.TypeQuoteExpiry, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseQuoteExpiryPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse quote expiry payload: %w", err)
		}
		deps.logger.Info("processing quote expiry",
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement quote expiry logic
		return nil
	})

	// API key expiry notification task
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

	// Label generation task
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

	// COD settlement task
	mux.HandleFunc(tasks.TypeCODSettlement, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseCODSettlementPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse COD settlement payload: %w", err)
		}
		deps.logger.Info("processing COD settlement",
			"settlement_id", payload.SettlementID,
		)
		// TODO: Implement COD settlement logic
		return nil
	})

	// Webhook DLQ task
	mux.HandleFunc(tasks.TypeWebhookDLQ, func(ctx context.Context, task *asynq.Task) error {
		payload, err := tasks.ParseWebhookDLQPayload(task)
		if err != nil {
			return fmt.Errorf("failed to parse webhook DLQ payload: %w", err)
		}
		deps.logger.Info("processing webhook DLQ",
			"delivery_id", payload.DeliveryID,
			"batch_size", payload.BatchSize,
		)
		// TODO: Implement webhook DLQ logic
		return nil
	})

	deps.logger.Info("all task handlers registered")
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
