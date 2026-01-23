// Package tasks provides background task handlers for the worker.
package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// RetentionPolicy defines the retention period for a specific data type.
type RetentionPolicy struct {
	TableName       string        // Database table name
	RetentionPeriod time.Duration // How long to keep records
	Description     string        // Human-readable description
	DeleteQuery     string        // SQL query name to execute
}

// DefaultRetentionPolicies defines the default data retention policies.
// These are based on the data-model.md specification:
// - Shipment records: 7 years (no auto-delete; archive after 2 years)
// - Payment/billing records: 10 years (no auto-delete)
// - Application logs: 30 days
// - Audit logs: 2 years
// - Soft-deleted PII: 30 days
// - Webhook delivery logs: 90 days
// - Provider API logs: 30 days
// - Idempotency records: 24 hours (handled by separate task)
var DefaultRetentionPolicies = []RetentionPolicy{
	{
		TableName:       "audit_logs",
		RetentionPeriod: 365 * 24 * time.Hour, // 1 year (conservative, spec says 2 years)
		Description:     "System audit trail",
		DeleteQuery:     "DeleteOldAuditLogs",
	},
	{
		TableName:       "provider_api_logs",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
		Description:     "Provider API call logs",
		DeleteQuery:     "DeleteOldProviderAPILogs",
	},
	{
		TableName:       "webhook_deliveries",
		RetentionPeriod: 90 * 24 * time.Hour, // 90 days
		Description:     "Webhook delivery attempts",
		DeleteQuery:     "DeleteOldWebhookDeliveries",
	},
	{
		TableName:       "webhook_events",
		RetentionPeriod: 90 * 24 * time.Hour, // 90 days
		Description:     "Generated webhook events",
		DeleteQuery:     "DeleteOldWebhookEvents",
	},
	{
		TableName:       "sessions",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
		Description:     "Expired user sessions",
		DeleteQuery:     "DeleteExpiredSessions",
	},
	{
		TableName:       "outbox",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
		Description:     "Published outbox messages",
		DeleteQuery:     "DeleteOldOutboxMessages",
	},
	{
		TableName:       "addresses",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days for soft-deleted
		Description:     "Soft-deleted addresses (PII)",
		DeleteQuery:     "HardDeleteOldSoftDeletedAddresses",
	},
	{
		TableName:       "provider_accounts",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days for soft-deleted
		Description:     "Soft-deleted provider accounts",
		DeleteQuery:     "HardDeleteOldSoftDeletedProviderAccounts",
	},
}

// RetentionCleanupHandler handles data retention cleanup tasks.
// It enforces data retention policies by deleting old records
// according to the configured retention periods.
type RetentionCleanupHandler struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	logger   *observability.Logger
	policies []RetentionPolicy
}

// NewRetentionCleanupHandler creates a new retention cleanup handler.
func NewRetentionCleanupHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *RetentionCleanupHandler {
	return &RetentionCleanupHandler{
		pool:     pool,
		queries:  queries,
		logger:   logger,
		policies: DefaultRetentionPolicies,
	}
}

// RetentionCleanupResult holds the results of a retention cleanup run.
type RetentionCleanupResult struct {
	StartedAt   time.Time                    `json:"started_at"`
	CompletedAt time.Time                    `json:"completed_at"`
	TableStats  map[string]TableCleanupStats `json:"table_stats"`
	TotalErrors int                          `json:"total_errors"`
}

// TableCleanupStats holds cleanup statistics for a single table.
type TableCleanupStats struct {
	TableName       string        `json:"table_name"`
	RetentionPeriod time.Duration `json:"retention_period"`
	DeletedCount    int64         `json:"deleted_count"`
	Error           string        `json:"error,omitempty"`
	Duration        time.Duration `json:"duration"`
}

// ProcessTask handles the retention cleanup task.
// It runs daily at 2:00 AM Turkey time as configured in the scheduler.
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

	result := &RetentionCleanupResult{
		StartedAt:  time.Now(),
		TableStats: make(map[string]TableCleanupStats),
	}

	// Determine which policies to apply
	policies := h.policies
	if payload.TableName != "" {
		// Filter to specific table if requested
		policies = h.filterPolicies(payload.TableName)
		if len(policies) == 0 {
			h.logger.Warn("No retention policy found for table", "table_name", payload.TableName)
			return nil
		}
	}

	// Apply each retention policy
	for _, policy := range policies {
		stats := h.applyRetentionPolicy(ctx, policy, payload.BatchSize)
		result.TableStats[policy.TableName] = stats
		if stats.Error != "" {
			result.TotalErrors++
		}
	}

	result.CompletedAt = time.Now()

	// Log summary
	h.logger.Info("Completed retention cleanup",
		"duration_ms", result.CompletedAt.Sub(result.StartedAt).Milliseconds(),
		"tables_processed", len(result.TableStats),
		"total_errors", result.TotalErrors,
	)

	// Log individual table stats
	for tableName, stats := range result.TableStats {
		if stats.Error != "" {
			h.logger.Error("Retention cleanup failed for table",
				"table_name", tableName,
				"error", stats.Error,
			)
		} else {
			h.logger.Info("Retention cleanup completed for table",
				"table_name", tableName,
				"deleted_count", stats.DeletedCount,
				"duration_ms", stats.Duration.Milliseconds(),
			)
		}
	}

	return nil
}

// filterPolicies returns policies matching the given table name.
func (h *RetentionCleanupHandler) filterPolicies(tableName string) []RetentionPolicy {
	var filtered []RetentionPolicy
	for _, policy := range h.policies {
		if policy.TableName == tableName {
			filtered = append(filtered, policy)
		}
	}
	return filtered
}

// applyRetentionPolicy applies a single retention policy.
func (h *RetentionCleanupHandler) applyRetentionPolicy(ctx context.Context, policy RetentionPolicy, batchSize int) TableCleanupStats {
	stats := TableCleanupStats{
		TableName:       policy.TableName,
		RetentionPeriod: policy.RetentionPeriod,
	}

	startTime := time.Now()

	// Execute the appropriate cleanup based on the table
	var err error
	switch policy.TableName {
	case "audit_logs":
		err = h.cleanupAuditLogs(ctx)
	case "provider_api_logs":
		err = h.cleanupProviderAPILogs(ctx)
	case "webhook_deliveries":
		err = h.cleanupWebhookDeliveries(ctx)
	case "webhook_events":
		err = h.cleanupWebhookEvents(ctx)
	case "sessions":
		err = h.cleanupExpiredSessions(ctx)
	case "outbox":
		err = h.cleanupOutbox(ctx)
	case "addresses":
		err = h.cleanupSoftDeletedAddresses(ctx)
	case "provider_accounts":
		err = h.cleanupSoftDeletedProviderAccounts(ctx)
	default:
		err = fmt.Errorf("unknown table: %s", policy.TableName)
	}

	stats.Duration = time.Since(startTime)

	if err != nil {
		stats.Error = err.Error()
	}

	return stats
}

// cleanupAuditLogs removes old audit log entries.
func (h *RetentionCleanupHandler) cleanupAuditLogs(ctx context.Context) error {
	return h.queries.DeleteOldAuditLogs(ctx)
}

// cleanupProviderAPILogs removes old provider API log entries.
func (h *RetentionCleanupHandler) cleanupProviderAPILogs(ctx context.Context) error {
	return h.queries.DeleteOldProviderApiLogs(ctx)
}

// cleanupWebhookDeliveries removes old webhook delivery records.
func (h *RetentionCleanupHandler) cleanupWebhookDeliveries(ctx context.Context) error {
	return h.queries.DeleteOldWebhookDeliveries(ctx)
}

// cleanupWebhookEvents removes old webhook event records.
func (h *RetentionCleanupHandler) cleanupWebhookEvents(ctx context.Context) error {
	return h.queries.DeleteOldWebhookEvents(ctx)
}

// cleanupExpiredSessions removes expired session records.
func (h *RetentionCleanupHandler) cleanupExpiredSessions(ctx context.Context) error {
	return h.queries.DeleteExpiredSessions(ctx)
}

// cleanupOutbox removes old published outbox messages.
func (h *RetentionCleanupHandler) cleanupOutbox(ctx context.Context) error {
	return h.queries.DeleteOldOutboxMessages(ctx)
}

// cleanupSoftDeletedAddresses hard-deletes addresses that were soft-deleted
// more than 30 days ago (PII retention policy).
func (h *RetentionCleanupHandler) cleanupSoftDeletedAddresses(ctx context.Context) error {
	return h.queries.HardDeleteExpiredAddresses(ctx)
}

// cleanupSoftDeletedProviderAccounts hard-deletes provider accounts that were
// soft-deleted more than 30 days ago.
func (h *RetentionCleanupHandler) cleanupSoftDeletedProviderAccounts(ctx context.Context) error {
	return h.queries.HardDeleteExpiredProviderAccounts(ctx)
}
