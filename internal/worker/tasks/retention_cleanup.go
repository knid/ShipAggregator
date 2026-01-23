package tasks

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// RetentionCleanupHandler handles data retention cleanup tasks.
// It enforces data retention policies by removing old records from various tables.
type RetentionCleanupHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewRetentionCleanupHandler creates a new retention cleanup handler.
func NewRetentionCleanupHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *RetentionCleanupHandler {
	return &RetentionCleanupHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles a retention cleanup task.
func (h *RetentionCleanupHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseRetentionCleanupPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse retention cleanup payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting retention cleanup",
		"table_name", payload.TableName,
		"batch_size", payload.BatchSize,
	)

	// If a specific table is specified, clean only that table
	if payload.TableName != "" {
		if err := h.cleanupTable(ctx, payload.TableName); err != nil {
			return fmt.Errorf("failed to cleanup table %s: %w", payload.TableName, err)
		}
	} else {
		// Clean all tables with retention policies
		if err := h.cleanupAllTables(ctx); err != nil {
			return err
		}
	}

	h.logger.Info("Completed retention cleanup")
	return nil
}

// cleanupTable cleans up a specific table based on its retention policy.
func (h *RetentionCleanupHandler) cleanupTable(ctx context.Context, tableName string) error {
	switch tableName {
	case "audit_logs":
		return h.cleanupAuditLogs(ctx)
	case "provider_api_logs":
		return h.cleanupProviderAPILogs(ctx)
	case "webhook_events":
		return h.cleanupWebhookEvents(ctx)
	case "webhook_deliveries":
		return h.cleanupWebhookDeliveries(ctx)
	case "tracking_events":
		return h.cleanupTrackingEvents(ctx)
	case "outbox":
		return h.cleanupOutbox(ctx)
	case "sessions":
		return h.cleanupSessions(ctx)
	default:
		h.logger.Warn("Unknown table for retention cleanup", "table_name", tableName)
		return nil
	}
}

// cleanupAllTables cleans up all tables with retention policies.
func (h *RetentionCleanupHandler) cleanupAllTables(ctx context.Context) error {
	tables := []struct {
		name    string
		cleanup func(context.Context) error
	}{
		{"audit_logs", h.cleanupAuditLogs},
		{"provider_api_logs", h.cleanupProviderAPILogs},
		{"webhook_events", h.cleanupWebhookEvents},
		{"webhook_deliveries", h.cleanupWebhookDeliveries},
		{"tracking_events", h.cleanupTrackingEvents},
		{"outbox", h.cleanupOutbox},
		{"sessions", h.cleanupSessions},
	}

	var errors []error
	for _, table := range tables {
		h.logger.Info("Cleaning up table", "table_name", table.name)
		if err := table.cleanup(ctx); err != nil {
			h.logger.Error("Failed to cleanup table",
				"table_name", table.name,
				"error", err,
			)
			errors = append(errors, fmt.Errorf("%s: %w", table.name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup failed for %d tables", len(errors))
	}

	return nil
}

// cleanupAuditLogs removes audit logs older than the retention period (90 days).
func (h *RetentionCleanupHandler) cleanupAuditLogs(ctx context.Context) error {
	return h.queries.DeleteOldAuditLogs(ctx)
}

// cleanupProviderAPILogs removes provider API logs older than the retention period (30 days).
func (h *RetentionCleanupHandler) cleanupProviderAPILogs(ctx context.Context) error {
	// Execute raw SQL since the query may not be generated
	_, err := h.pool.Exec(ctx, "DELETE FROM provider_api_logs WHERE created_at < NOW() - INTERVAL '30 days'")
	return err
}

// cleanupWebhookEvents removes webhook events older than the retention period (90 days).
func (h *RetentionCleanupHandler) cleanupWebhookEvents(ctx context.Context) error {
	return h.queries.DeleteOldWebhookEvents(ctx)
}

// cleanupWebhookDeliveries removes webhook deliveries older than the retention period (90 days).
func (h *RetentionCleanupHandler) cleanupWebhookDeliveries(ctx context.Context) error {
	return h.queries.DeleteOldWebhookDeliveries(ctx)
}

// cleanupTrackingEvents removes tracking events older than the retention period (365 days).
func (h *RetentionCleanupHandler) cleanupTrackingEvents(ctx context.Context) error {
	// Execute raw SQL since the query may not be generated
	_, err := h.pool.Exec(ctx, "DELETE FROM tracking_events WHERE created_at < NOW() - INTERVAL '365 days'")
	return err
}

// cleanupOutbox removes old outbox entries.
func (h *RetentionCleanupHandler) cleanupOutbox(ctx context.Context) error {
	// First delete published entries older than 7 days
	if err := h.queries.DeletePublishedOutboxMessages(ctx); err != nil {
		return err
	}
	// Then delete all entries older than 30 days
	return h.queries.DeleteOldOutboxMessages(ctx)
}

// cleanupSessions removes expired sessions.
func (h *RetentionCleanupHandler) cleanupSessions(ctx context.Context) error {
	return h.queries.DeleteExpiredSessions(ctx)
}

// QuoteExpiryHandler handles quote expiry tasks.
// It marks expired quotes as expired to prevent them from being used.
type QuoteExpiryHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewQuoteExpiryHandler creates a new quote expiry handler.
func NewQuoteExpiryHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *QuoteExpiryHandler {
	return &QuoteExpiryHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles a quote expiry task.
func (h *QuoteExpiryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseQuoteExpiryPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse quote expiry payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting quote expiry processing", "batch_size", payload.BatchSize)

	// Mark expired quotes using raw SQL
	_, err = h.pool.Exec(ctx, "UPDATE quotes SET status = 'expired' WHERE status = 'pending' AND expires_at < NOW()")
	if err != nil {
		return fmt.Errorf("failed to mark expired quotes: %w", err)
	}

	h.logger.Info("Completed quote expiry processing")
	return nil
}
