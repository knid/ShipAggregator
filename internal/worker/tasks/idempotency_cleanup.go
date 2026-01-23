package tasks

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// IdempotencyCleanupHandler handles idempotency record cleanup tasks.
// It removes expired idempotency records from the database to prevent
// unbounded growth of the idempotency table.
type IdempotencyCleanupHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewIdempotencyCleanupHandler creates a new idempotency cleanup handler.
func NewIdempotencyCleanupHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *IdempotencyCleanupHandler {
	return &IdempotencyCleanupHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles an idempotency cleanup task.
func (h *IdempotencyCleanupHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseIdempotencyCleanupPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse idempotency cleanup payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting idempotency cleanup",
		"max_age", payload.MaxAge,
		"batch_size", payload.BatchSize,
	)

	// Delete expired idempotency records
	// The query deletes records where expires_at < NOW()
	err = h.queries.DeleteExpiredIdempotencyRecords(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete expired idempotency records: %w", err)
	}

	h.logger.Info("Completed idempotency cleanup")
	return nil
}
