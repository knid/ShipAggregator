// Package worker provides background job processing using Asynq.
// It includes a scheduler for periodic tasks and task handlers for various
// background operations like webhook delivery, tracking polling, and cleanup.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/knid/kargonet/internal/config"
	"github.com/knid/kargonet/internal/worker/tasks"
)

// Scheduler manages periodic task scheduling using Asynq.
type Scheduler struct {
	scheduler *asynq.Scheduler
	logger    *slog.Logger
	cfg       *config.Config
}

// SchedulerOption is a functional option for configuring the Scheduler.
type SchedulerOption func(*Scheduler)

// WithSchedulerLogger sets a custom logger for the scheduler.
func WithSchedulerLogger(logger *slog.Logger) SchedulerOption {
	return func(s *Scheduler) {
		s.logger = logger
	}
}

// NewScheduler creates a new Asynq scheduler for periodic tasks.
func NewScheduler(cfg *config.Config, opts ...SchedulerOption) (*Scheduler, error) {
	// Parse Redis options
	redisOpts, err := parseRedisConfig(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis config: %w", err)
	}

	// Create Asynq scheduler with location set to Turkey timezone
	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		// Fallback to UTC if Turkey timezone is not available
		loc = time.UTC
	}

	scheduler := asynq.NewScheduler(
		redisOpts,
		&asynq.SchedulerOpts{
			Location: loc,
			Logger:   newAsynqLogger(slog.Default()),
			LogLevel: asynq.InfoLevel,
			PreEnqueueFunc: func(task *asynq.Task, opts []asynq.Option) {
				slog.Debug("enqueueing scheduled task",
					"type", task.Type(),
					"payload_size", len(task.Payload()),
				)
			},
			PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
				if err != nil {
					slog.Error("failed to enqueue scheduled task",
						"type", info.Type,
						"error", err,
					)
					return
				}
				slog.Debug("scheduled task enqueued",
					"type", info.Type,
					"queue", info.Queue,
					"task_id", info.ID,
				)
			},
		},
	)

	s := &Scheduler{
		scheduler: scheduler,
		logger:    slog.Default(),
		cfg:       cfg,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// RegisterTasks registers all periodic tasks with the scheduler.
// This should be called before Start().
func (s *Scheduler) RegisterTasks() error {
	s.logger.Info("registering scheduled tasks")

	// Outbox publisher - runs every 5 seconds to publish pending events
	if err := s.registerTask(
		"@every 5s",
		tasks.TypeOutboxPublish,
		nil,
		asynq.Queue(tasks.QueueCritical),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	); err != nil {
		return fmt.Errorf("failed to register outbox publisher: %w", err)
	}

	// Webhook delivery retry - runs every minute to retry failed deliveries
	if err := s.registerTask(
		"@every 1m",
		tasks.TypeWebhookRetry,
		nil,
		asynq.Queue(tasks.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(5*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register webhook retry: %w", err)
	}

	// Tracking poll - runs every 5 minutes for providers without webhooks
	if err := s.registerTask(
		"@every 5m",
		tasks.TypeTrackingPoll,
		nil,
		asynq.Queue(tasks.QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register tracking poll: %w", err)
	}

	// Idempotency cleanup - runs daily at 3:00 AM Turkey time
	// Removes expired idempotency records (older than 24 hours)
	if err := s.registerTask(
		"0 3 * * *", // 3:00 AM daily
		tasks.TypeIdempotencyCleanup,
		nil,
		asynq.Queue(tasks.QueueLow),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register idempotency cleanup: %w", err)
	}

	// Financial reconciliation - runs daily at 4:00 AM Turkey time
	// Verifies ledger invariants and reconciles with payment providers
	if err := s.registerTask(
		"0 4 * * *", // 4:00 AM daily
		tasks.TypeReconciliation,
		nil,
		asynq.Queue(tasks.QueueLow),
		asynq.MaxRetry(3),
		asynq.Timeout(1*time.Hour),
	); err != nil {
		return fmt.Errorf("failed to register reconciliation: %w", err)
	}

	// Data retention cleanup - runs daily at 2:00 AM Turkey time
	// Enforces data retention policies (audit logs, provider logs, etc.)
	if err := s.registerTask(
		"0 2 * * *", // 2:00 AM daily
		tasks.TypeRetentionCleanup,
		nil,
		asynq.Queue(tasks.QueueLow),
		asynq.MaxRetry(3),
		asynq.Timeout(1*time.Hour),
	); err != nil {
		return fmt.Errorf("failed to register retention cleanup: %w", err)
	}

	// Provider health check - runs every 10 minutes
	// Checks provider API availability and updates health status
	if err := s.registerTask(
		"@every 10m",
		tasks.TypeProviderHealthCheck,
		nil,
		asynq.Queue(tasks.QueueDefault),
		asynq.MaxRetry(1),
		asynq.Timeout(5*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register provider health check: %w", err)
	}

	// Quote expiry cleanup - runs every hour
	// Marks expired quotes and cleans up associated data
	if err := s.registerTask(
		"@every 1h",
		tasks.TypeQuoteExpiry,
		nil,
		asynq.Queue(tasks.QueueLow),
		asynq.MaxRetry(3),
		asynq.Timeout(15*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register quote expiry: %w", err)
	}

	// API key expiry notification - runs daily at 9:00 AM Turkey time
	// Sends notifications for API keys expiring within 7 days
	if err := s.registerTask(
		"0 9 * * *", // 9:00 AM daily
		tasks.TypeAPIKeyExpiryNotification,
		nil,
		asynq.Queue(tasks.QueueLow),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
	); err != nil {
		return fmt.Errorf("failed to register API key expiry notification: %w", err)
	}

	s.logger.Info("all scheduled tasks registered successfully")
	return nil
}

// registerTask is a helper to register a single periodic task.
func (s *Scheduler) registerTask(cronspec string, taskType string, payload []byte, opts ...asynq.Option) error {
	task := asynq.NewTask(taskType, payload)
	entryID, err := s.scheduler.Register(cronspec, task, opts...)
	if err != nil {
		return err
	}
	s.logger.Info("registered scheduled task",
		"type", taskType,
		"cronspec", cronspec,
		"entry_id", entryID,
	)
	return nil
}

// Start starts the scheduler. This is a blocking call.
func (s *Scheduler) Start() error {
	s.logger.Info("starting scheduler")
	return s.scheduler.Run()
}

// Shutdown gracefully shuts down the scheduler.
func (s *Scheduler) Shutdown() {
	s.logger.Info("shutting down scheduler")
	s.scheduler.Shutdown()
}

// Server manages the Asynq worker server for processing tasks.
type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
	logger *slog.Logger
	cfg    *config.Config
}

// ServerOption is a functional option for configuring the Server.
type ServerOption func(*Server)

// WithServerLogger sets a custom logger for the server.
func WithServerLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

// NewServer creates a new Asynq server for processing tasks.
func NewServer(cfg *config.Config, opts ...ServerOption) (*Server, error) {
	// Parse Redis options
	redisOpts, err := parseRedisConfig(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis config: %w", err)
	}

	// Create Asynq server with configuration from config
	server := asynq.NewServer(
		redisOpts,
		asynq.Config{
			Concurrency: cfg.Worker.Concurrency,
			Queues: map[string]int{
				tasks.QueueCritical: cfg.Worker.QueueCritical,
				tasks.QueueDefault:  cfg.Worker.QueueDefault,
				tasks.QueueLow:      cfg.Worker.QueueLow,
			},
			Logger:   newAsynqLogger(slog.Default()),
			LogLevel: asynq.InfoLevel,
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("task processing failed",
					"type", task.Type(),
					"error", err,
				)
			}),
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff with jitter
				// 1m, 5m, 30m, 2h, 8h, 24h
				delays := []time.Duration{
					1 * time.Minute,
					5 * time.Minute,
					30 * time.Minute,
					2 * time.Hour,
					8 * time.Hour,
					24 * time.Hour,
				}
				if n >= len(delays) {
					return delays[len(delays)-1]
				}
				return delays[n]
			},
		},
	)

	s := &Server{
		server: server,
		mux:    asynq.NewServeMux(),
		logger: slog.Default(),
		cfg:    cfg,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Mux returns the ServeMux for registering task handlers.
func (s *Server) Mux() *asynq.ServeMux {
	return s.mux
}

// Start starts the server. This is a blocking call.
func (s *Server) Start() error {
	s.logger.Info("starting worker server",
		"concurrency", s.cfg.Worker.Concurrency,
	)
	return s.server.Run(s.mux)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() {
	s.logger.Info("shutting down worker server")
	s.server.Shutdown()
}

// Client provides methods for enqueueing tasks.
type Client struct {
	client *asynq.Client
	logger *slog.Logger
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithClientLogger sets a custom logger for the client.
func WithClientLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewClient creates a new Asynq client for enqueueing tasks.
func NewClient(cfg *config.Config, opts ...ClientOption) (*Client, error) {
	// Parse Redis options
	redisOpts, err := parseRedisConfig(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis config: %w", err)
	}

	client := asynq.NewClient(redisOpts)

	c := &Client{
		client: client,
		logger: slog.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Enqueue enqueues a task for processing.
func (c *Client) Enqueue(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	info, err := c.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		c.logger.Error("failed to enqueue task",
			"type", task.Type(),
			"error", err,
		)
		return nil, err
	}
	c.logger.Debug("task enqueued",
		"type", task.Type(),
		"queue", info.Queue,
		"task_id", info.ID,
	)
	return info, nil
}

// EnqueueWebhookDelivery enqueues a webhook delivery task.
func (c *Client) EnqueueWebhookDelivery(ctx context.Context, deliveryID string) (*asynq.TaskInfo, error) {
	task, err := tasks.NewWebhookDeliveryTask(deliveryID)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, task,
		asynq.Queue(tasks.QueueCritical),
		asynq.MaxRetry(tasks.MaxRetryWebhookDelivery),
		asynq.Timeout(tasks.TimeoutWebhookDelivery),
	)
}

// EnqueueTrackingUpdate enqueues a tracking update task.
func (c *Client) EnqueueTrackingUpdate(ctx context.Context, shipmentID string, providerCode string) (*asynq.TaskInfo, error) {
	task, err := tasks.NewTrackingUpdateTask(shipmentID, providerCode)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, task,
		asynq.Queue(tasks.QueueDefault),
		asynq.MaxRetry(tasks.MaxRetryTrackingUpdate),
		asynq.Timeout(tasks.TimeoutTrackingUpdate),
	)
}

// EnqueueLabelGeneration enqueues a label generation task.
func (c *Client) EnqueueLabelGeneration(ctx context.Context, shipmentID string) (*asynq.TaskInfo, error) {
	task, err := tasks.NewLabelGenerationTask(shipmentID)
	if err != nil {
		return nil, err
	}
	return c.Enqueue(ctx, task,
		asynq.Queue(tasks.QueueCritical),
		asynq.MaxRetry(tasks.MaxRetryLabelGeneration),
		asynq.Timeout(tasks.TimeoutLabelGeneration),
	)
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// parseRedisConfig converts config.RedisConfig to asynq.RedisClientOpt.
func parseRedisConfig(cfg config.RedisConfig) (asynq.RedisClientOpt, error) {
	// If URL is provided, parse it
	if cfg.URL != "" {
		redisOpts, err := redis.ParseURL(cfg.URL)
		if err != nil {
			return asynq.RedisClientOpt{}, fmt.Errorf("failed to parse Redis URL: %w", err)
		}
		return asynq.RedisClientOpt{
			Addr:     redisOpts.Addr,
			Password: redisOpts.Password,
			DB:       redisOpts.DB,
		}, nil
	}

	// Otherwise use individual settings
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	}, nil
}

// asynqLogger adapts slog.Logger to asynq.Logger interface.
type asynqLogger struct {
	logger *slog.Logger
}

func newAsynqLogger(logger *slog.Logger) *asynqLogger {
	return &asynqLogger{logger: logger}
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug(fmt.Sprint(args...))
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn(fmt.Sprint(args...))
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error(fmt.Sprint(args...))
}
