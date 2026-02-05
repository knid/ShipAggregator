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

// CODSettlementHandler handles COD (Cash on Delivery) settlement tasks.
// It processes COD settlements by crediting merchant wallets.
type CODSettlementHandler struct {
	dbPool  *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewCODSettlementHandler creates a new CODSettlementHandler.
func NewCODSettlementHandler(
	dbPool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *CODSettlementHandler {
	return &CODSettlementHandler{
		dbPool:  dbPool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask processes a COD settlement task.
func (h *CODSettlementHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseCODSettlementPayload(task)
	if err != nil {
		return fmt.Errorf("failed to parse COD settlement payload: %w", err)
	}

	h.logger.Info("processing COD settlement task",
		"settlement_id", payload.SettlementID,
	)

	startTime := time.Now()

	// Process the COD settlement
	if err := h.processSettlement(ctx, payload.SettlementID); err != nil {
		h.logger.Error("failed to process COD settlement",
			"settlement_id", payload.SettlementID,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to process COD settlement: %w", err)
	}

	duration := time.Since(startTime)
	h.logger.Info("COD settlement task completed",
		"settlement_id", payload.SettlementID,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

// processSettlement processes a single COD settlement.
// TODO: Implement full COD settlement processing. The steps should be:
//  1. Fetch the settlement record from the database
//  2. Verify the settlement is in 'pending' status
//  3. Calculate the settlement amount (COD amount - platform fees)
//  4. Create a ledger entry to credit the merchant's wallet
//  5. Update the settlement status to 'completed'
//  6. Trigger a webhook notification to the merchant
func (h *CODSettlementHandler) processSettlement(ctx context.Context, settlementID string) error {
	return fmt.Errorf("COD settlement processing not yet implemented (settlement_id=%s)", settlementID)
}
