// Package tasks provides background task handlers for the shipping aggregator.
package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// QuoteExpiryHandler handles quote expiry tasks.
// It marks expired quotes as expired in the database.
type QuoteExpiryHandler struct {
	dbPool  *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewQuoteExpiryHandler creates a new QuoteExpiryHandler.
func NewQuoteExpiryHandler(
	dbPool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *QuoteExpiryHandler {
	return &QuoteExpiryHandler{
		dbPool:  dbPool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask processes a quote expiry task.
func (h *QuoteExpiryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseQuoteExpiryPayload(task)
	if err != nil {
		return fmt.Errorf("failed to parse quote expiry payload: %w", err)
	}

	h.logger.Info("processing quote expiry task",
		"batch_size", payload.BatchSize,
	)

	startTime := time.Now()

	// Mark expired quotes
	// In a real implementation, this would call a query to update quotes
	// where expires_at < NOW() and status = 'pending'
	expiredCount, err := h.markExpiredQuotes(ctx, payload.BatchSize)
	if err != nil {
		h.logger.Error("failed to mark expired quotes",
			"error", err.Error(),
		)
		return fmt.Errorf("failed to mark expired quotes: %w", err)
	}

	duration := time.Since(startTime)
	h.logger.Info("quote expiry task completed",
		"expired_count", expiredCount,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

// markExpiredQuotes marks expired quotes in the database.
func (h *QuoteExpiryHandler) markExpiredQuotes(ctx context.Context, batchSize int) (int64, error) {
	// This is a placeholder implementation
	// In a real implementation, this would execute a query like:
	// UPDATE quotes SET status = 'expired', updated_at = NOW()
	// WHERE expires_at < NOW() AND status = 'pending'
	// LIMIT $1
	// RETURNING id

	h.logger.Debug("marking expired quotes",
		"batch_size", batchSize,
	)

	// For now, return 0 as no actual database operation is performed
	// This would be replaced with actual sqlc-generated query
	return 0, nil
}
