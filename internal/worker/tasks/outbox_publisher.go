package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// OutboxPublisherHandler handles outbox publishing tasks.
// It reads unpublished events from the outbox table and publishes them
// to the appropriate message queue or webhook system.
type OutboxPublisherHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	client  *asynq.Client
	logger  *observability.Logger
}

// NewOutboxPublisherHandler creates a new outbox publisher handler.
func NewOutboxPublisherHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	client *asynq.Client,
	logger *observability.Logger,
) *OutboxPublisherHandler {
	return &OutboxPublisherHandler{
		pool:    pool,
		queries: queries,
		client:  client,
		logger:  logger,
	}
}

// ProcessTask handles an outbox publishing task.
func (h *OutboxPublisherHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseOutboxPublishPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse outbox publish payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting outbox publishing", "batch_size", payload.BatchSize)

	// Get unpublished outbox entries using ListPendingOutboxMessages
	entries, err := h.queries.ListPendingOutboxMessages(ctx, int32(payload.BatchSize))
	if err != nil {
		return fmt.Errorf("failed to list outbox entries: %w", err)
	}

	if len(entries) == 0 {
		h.logger.Info("No outbox entries to publish")
		return nil
	}

	h.logger.Info("Found outbox entries to publish", "count", len(entries))

	// Process each entry
	var publishedCount, errorCount int
	for _, entry := range entries {
		if err := h.publishEntry(ctx, entry); err != nil {
			h.logger.Warn("Failed to publish outbox entry",
				"entry_id", entry.ID,
				"event_type", entry.EventType,
				"error", err,
			)
			errorCount++

			// Increment retry count
			if _, err := h.queries.UpdateOutboxMessageRetry(ctx, entry.ID); err != nil {
				h.logger.Error("Failed to increment retry count",
					"entry_id", entry.ID,
					"error", err,
				)
			}
			continue
		}

		// Mark as published
		if _, err := h.queries.UpdateOutboxMessagePublished(ctx, entry.ID); err != nil {
			h.logger.Error("Failed to mark entry as published",
				"entry_id", entry.ID,
				"error", err,
			)
			errorCount++
			continue
		}

		publishedCount++
	}

	h.logger.Info("Completed outbox publishing",
		"published", publishedCount,
		"errors", errorCount,
		"total", len(entries),
	)

	return nil
}

// publishEntry publishes a single outbox entry.
func (h *OutboxPublisherHandler) publishEntry(ctx context.Context, entry *db.Outbox) error {
	// Parse the payload
	var payload map[string]interface{}
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Route based on aggregate type
	switch entry.AggregateType {
	case "shipment":
		return h.publishShipmentEvent(ctx, entry, payload)
	case "billing":
		return h.publishBillingEvent(ctx, entry, payload)
	case "webhook":
		return h.publishWebhookEvent(ctx, entry, payload)
	default:
		h.logger.Warn("Unknown aggregate type",
			"aggregate_type", entry.AggregateType,
			"event_type", entry.EventType,
		)
		// Mark as published to avoid infinite retries
		return nil
	}
}

// publishShipmentEvent publishes a shipment-related event.
func (h *OutboxPublisherHandler) publishShipmentEvent(ctx context.Context, entry *db.Outbox, payload map[string]interface{}) error {
	// Create an async task for shipment event processing
	eventTask, err := newShipmentEventTask(entry.ID.String(), entry.EventType, payload)
	if err != nil {
		return fmt.Errorf("failed to create shipment event task: %w", err)
	}

	_, err = h.client.EnqueueContext(ctx, eventTask,
		asynq.Queue(QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue shipment event task: %w", err)
	}

	return nil
}

// publishBillingEvent publishes a billing-related event.
func (h *OutboxPublisherHandler) publishBillingEvent(ctx context.Context, entry *db.Outbox, payload map[string]interface{}) error {
	// Create an async task for billing event processing
	eventTask, err := newBillingEventTask(entry.ID.String(), entry.EventType, payload)
	if err != nil {
		return fmt.Errorf("failed to create billing event task: %w", err)
	}

	_, err = h.client.EnqueueContext(ctx, eventTask,
		asynq.Queue(QueueDefault),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue billing event task: %w", err)
	}

	return nil
}

// publishWebhookEvent publishes a webhook event.
func (h *OutboxPublisherHandler) publishWebhookEvent(ctx context.Context, entry *db.Outbox, payload map[string]interface{}) error {
	// Extract webhook delivery ID from payload
	deliveryID, ok := payload["delivery_id"].(string)
	if !ok {
		return fmt.Errorf("missing delivery_id in webhook payload")
	}

	// Create webhook delivery task
	deliveryTask, err := NewWebhookDeliveryTask(deliveryID)
	if err != nil {
		return fmt.Errorf("failed to create webhook delivery task: %w", err)
	}

	_, err = h.client.EnqueueContext(ctx, deliveryTask,
		asynq.Queue(QueueCritical),
		asynq.MaxRetry(MaxRetryWebhookDelivery),
		asynq.Timeout(TimeoutWebhookDelivery),
	)
	if err != nil {
		return fmt.Errorf("failed to enqueue webhook delivery task: %w", err)
	}

	return nil
}

// Internal task type constants for outbox events
const (
	typeShipmentEvent = "event:shipment"
	typeBillingEvent  = "event:billing"
)

// newShipmentEventTask creates a new shipment event task.
func newShipmentEventTask(outboxID, eventType string, payload map[string]interface{}) (*asynq.Task, error) {
	data := map[string]interface{}{
		"outbox_id":  outboxID,
		"event_type": eventType,
		"payload":    payload,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(typeShipmentEvent, payloadBytes), nil
}

// newBillingEventTask creates a new billing event task.
func newBillingEventTask(outboxID, eventType string, payload map[string]interface{}) (*asynq.Task, error) {
	data := map[string]interface{}{
		"outbox_id":  outboxID,
		"event_type": eventType,
		"payload":    payload,
	}
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(typeBillingEvent, payloadBytes), nil
}

// OutboxCleanupHandler handles outbox cleanup tasks.
// It removes old published entries from the outbox table.
type OutboxCleanupHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewOutboxCleanupHandler creates a new outbox cleanup handler.
func NewOutboxCleanupHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *OutboxCleanupHandler {
	return &OutboxCleanupHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles an outbox cleanup task.
func (h *OutboxCleanupHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	h.logger.Info("Starting outbox cleanup")

	// Delete old published entries (7 days retention)
	err := h.queries.DeletePublishedOutboxMessages(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete old outbox entries: %w", err)
	}

	// Also delete very old entries (30 days) regardless of status
	err = h.queries.DeleteOldOutboxMessages(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete old outbox entries: %w", err)
	}

	h.logger.Info("Completed outbox cleanup")
	return nil
}
