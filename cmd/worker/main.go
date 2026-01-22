// Package main is the entry point for the background worker.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// Build information (set via ldflags).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	logger.Info("starting background worker",
		"version", Version,
		"commit", Commit,
		"build_time", BuildTime,
	)

	// Get Redis URL from environment
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Parse Redis options
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("failed to parse Redis URL", "error", err)
		os.Exit(1)
	}

	// Create Asynq server
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     redisOpts.Addr,
			Password: redisOpts.Password,
			DB:       redisOpts.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("task failed",
					"type", task.Type(),
					"error", err,
				)
			}),
		},
	)

	// Create task handler mux
	mux := asynq.NewServeMux()

	// TODO: Register task handlers
	// mux.HandleFunc("webhook:deliver", handleWebhookDelivery)
	// mux.HandleFunc("tracking:poll", handleTrackingPoll)
	// mux.HandleFunc("outbox:publish", handleOutboxPublish)

	// Start server in goroutine
	go func() {
		logger.Info("worker starting...")
		if err := srv.Run(mux); err != nil {
			logger.Error("worker error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down worker...")

	// Graceful shutdown
	srv.Shutdown()

	logger.Info("worker stopped")
}
