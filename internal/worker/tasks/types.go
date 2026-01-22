// Package tasks defines task types, payloads, and constants for background job processing.
// All background tasks in the shipping aggregator are defined here with their
// type constants, payload structures, and configuration values.
package tasks

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

// Queue names for task prioritization.
// Tasks are processed based on queue priority weights defined in the worker config.
const (
	// QueueCritical is for time-sensitive tasks that must be processed immediately.
	// Examples: webhook delivery, label generation, payment processing.
	QueueCritical = "critical"

	// QueueDefault is for standard priority tasks.
	// Examples: tracking updates, email notifications.
	QueueDefault = "default"

	// QueueLow is for background maintenance tasks that can be delayed.
	// Examples: cleanup jobs, reconciliation, analytics.
	QueueLow = "low"
)

// Task type constants.
// Each task type corresponds to a specific background operation.
const (
	// Webhook tasks
	TypeWebhookDelivery = "webhook:delivery" // Deliver a single webhook event
	TypeWebhookRetry    = "webhook:retry"    // Retry failed webhook deliveries
	TypeWebhookDLQ      = "webhook:dlq"      // Process dead-letter queue entries

	// Tracking tasks
	TypeTrackingPoll   = "tracking:poll"   // Poll providers for tracking updates
	TypeTrackingUpdate = "tracking:update" // Process a single tracking update

	// Outbox tasks
	TypeOutboxPublish = "outbox:publish" // Publish pending outbox events

	// Cleanup tasks
	TypeIdempotencyCleanup = "cleanup:idempotency" // Remove expired idempotency records
	TypeRetentionCleanup   = "cleanup:retention"   // Enforce data retention policies
	TypeQuoteExpiry        = "cleanup:quote"       // Mark expired quotes

	// Billing tasks
	TypeReconciliation = "billing:reconciliation" // Daily financial reconciliation
	TypeCODSettlement  = "billing:cod_settlement" // Process COD settlement

	// Provider tasks
	TypeProviderHealthCheck = "provider:health_check" // Check provider API health
	TypeLabelGeneration     = "label:generation"      // Generate shipping label

	// Notification tasks
	TypeAPIKeyExpiryNotification = "notification:apikey_expiry" // Notify about expiring API keys
	TypeWebhookFailureAlert      = "notification:webhook_fail"  // Alert on webhook delivery failures
)

// Task timeout constants.
// These define the maximum execution time for each task type.
const (
	TimeoutWebhookDelivery = 30 * time.Second
	TimeoutWebhookRetry    = 5 * time.Minute
	TimeoutTrackingPoll    = 10 * time.Minute
	TimeoutTrackingUpdate  = 2 * time.Minute
	TimeoutOutboxPublish   = 30 * time.Second
	TimeoutIdempotency     = 30 * time.Minute
	TimeoutRetention       = 1 * time.Hour
	TimeoutReconciliation  = 1 * time.Hour
	TimeoutProviderHealth  = 5 * time.Minute
	TimeoutLabelGeneration = 2 * time.Minute
	TimeoutQuoteExpiry     = 15 * time.Minute
)

// Task retry constants.
// These define the maximum number of retries for each task type.
const (
	MaxRetryWebhookDelivery = 7 // Matches webhook retry schedule
	MaxRetryWebhookRetry    = 3 // Retry the retry job itself
	MaxRetryTrackingPoll    = 3 // Polling can be retried
	MaxRetryTrackingUpdate  = 5 // Individual updates can retry more
	MaxRetryOutboxPublish   = 3 // Outbox publishing
	MaxRetryIdempotency     = 3 // Cleanup jobs
	MaxRetryRetention       = 3 // Cleanup jobs
	MaxRetryReconciliation  = 3 // Reconciliation
	MaxRetryProviderHealth  = 1 // Health checks shouldn't retry much
	MaxRetryLabelGeneration = 3 // Label generation
	MaxRetryQuoteExpiry     = 3 // Quote expiry cleanup
)

// WebhookDeliveryPayload is the payload for webhook delivery tasks.
type WebhookDeliveryPayload struct {
	DeliveryID string `json:"delivery_id"`
}

// NewWebhookDeliveryTask creates a new webhook delivery task.
func NewWebhookDeliveryTask(deliveryID string) (*asynq.Task, error) {
	payload, err := json.Marshal(WebhookDeliveryPayload{
		DeliveryID: deliveryID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWebhookDelivery, payload), nil
}

// ParseWebhookDeliveryPayload parses the payload of a webhook delivery task.
func ParseWebhookDeliveryPayload(task *asynq.Task) (*WebhookDeliveryPayload, error) {
	var payload WebhookDeliveryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// TrackingUpdatePayload is the payload for tracking update tasks.
type TrackingUpdatePayload struct {
	ShipmentID   string `json:"shipment_id"`
	ProviderCode string `json:"provider_code"`
}

// NewTrackingUpdateTask creates a new tracking update task.
func NewTrackingUpdateTask(shipmentID, providerCode string) (*asynq.Task, error) {
	payload, err := json.Marshal(TrackingUpdatePayload{
		ShipmentID:   shipmentID,
		ProviderCode: providerCode,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeTrackingUpdate, payload), nil
}

// ParseTrackingUpdatePayload parses the payload of a tracking update task.
func ParseTrackingUpdatePayload(task *asynq.Task) (*TrackingUpdatePayload, error) {
	var payload TrackingUpdatePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// LabelGenerationPayload is the payload for label generation tasks.
type LabelGenerationPayload struct {
	ShipmentID string `json:"shipment_id"`
}

// NewLabelGenerationTask creates a new label generation task.
func NewLabelGenerationTask(shipmentID string) (*asynq.Task, error) {
	payload, err := json.Marshal(LabelGenerationPayload{
		ShipmentID: shipmentID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeLabelGeneration, payload), nil
}

// ParseLabelGenerationPayload parses the payload of a label generation task.
func ParseLabelGenerationPayload(task *asynq.Task) (*LabelGenerationPayload, error) {
	var payload LabelGenerationPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// OutboxPublishPayload is the payload for outbox publish tasks.
// This task typically runs without a payload (scheduled job).
type OutboxPublishPayload struct {
	BatchSize int `json:"batch_size,omitempty"`
}

// NewOutboxPublishTask creates a new outbox publish task.
func NewOutboxPublishTask(batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(OutboxPublishPayload{
		BatchSize: batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeOutboxPublish, payload), nil
}

// ParseOutboxPublishPayload parses the payload of an outbox publish task.
func ParseOutboxPublishPayload(task *asynq.Task) (*OutboxPublishPayload, error) {
	var payload OutboxPublishPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &OutboxPublishPayload{BatchSize: 100}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 100
	}
	return &payload, nil
}

// TrackingPollPayload is the payload for tracking poll tasks.
type TrackingPollPayload struct {
	ProviderCode string `json:"provider_code,omitempty"` // Empty means all providers
	BatchSize    int    `json:"batch_size,omitempty"`
}

// NewTrackingPollTask creates a new tracking poll task.
func NewTrackingPollTask(providerCode string, batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(TrackingPollPayload{
		ProviderCode: providerCode,
		BatchSize:    batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeTrackingPoll, payload), nil
}

// ParseTrackingPollPayload parses the payload of a tracking poll task.
func ParseTrackingPollPayload(task *asynq.Task) (*TrackingPollPayload, error) {
	var payload TrackingPollPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &TrackingPollPayload{BatchSize: 100}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 100
	}
	return &payload, nil
}

// WebhookRetryPayload is the payload for webhook retry tasks.
type WebhookRetryPayload struct {
	BatchSize int `json:"batch_size,omitempty"`
}

// NewWebhookRetryTask creates a new webhook retry task.
func NewWebhookRetryTask(batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(WebhookRetryPayload{
		BatchSize: batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWebhookRetry, payload), nil
}

// ParseWebhookRetryPayload parses the payload of a webhook retry task.
func ParseWebhookRetryPayload(task *asynq.Task) (*WebhookRetryPayload, error) {
	var payload WebhookRetryPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &WebhookRetryPayload{BatchSize: 100}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 100
	}
	return &payload, nil
}

// IdempotencyCleanupPayload is the payload for idempotency cleanup tasks.
type IdempotencyCleanupPayload struct {
	MaxAge    time.Duration `json:"max_age,omitempty"`
	BatchSize int           `json:"batch_size,omitempty"`
}

// NewIdempotencyCleanupTask creates a new idempotency cleanup task.
func NewIdempotencyCleanupTask(maxAge time.Duration, batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(IdempotencyCleanupPayload{
		MaxAge:    maxAge,
		BatchSize: batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeIdempotencyCleanup, payload), nil
}

// ParseIdempotencyCleanupPayload parses the payload of an idempotency cleanup task.
func ParseIdempotencyCleanupPayload(task *asynq.Task) (*IdempotencyCleanupPayload, error) {
	var payload IdempotencyCleanupPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &IdempotencyCleanupPayload{
			MaxAge:    24 * time.Hour,
			BatchSize: 1000,
		}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.MaxAge == 0 {
		payload.MaxAge = 24 * time.Hour
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 1000
	}
	return &payload, nil
}

// RetentionCleanupPayload is the payload for retention cleanup tasks.
type RetentionCleanupPayload struct {
	TableName string `json:"table_name,omitempty"` // Empty means all tables
	BatchSize int    `json:"batch_size,omitempty"`
}

// NewRetentionCleanupTask creates a new retention cleanup task.
func NewRetentionCleanupTask(tableName string, batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(RetentionCleanupPayload{
		TableName: tableName,
		BatchSize: batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeRetentionCleanup, payload), nil
}

// ParseRetentionCleanupPayload parses the payload of a retention cleanup task.
func ParseRetentionCleanupPayload(task *asynq.Task) (*RetentionCleanupPayload, error) {
	var payload RetentionCleanupPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &RetentionCleanupPayload{BatchSize: 1000}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 1000
	}
	return &payload, nil
}

// ReconciliationPayload is the payload for reconciliation tasks.
type ReconciliationPayload struct {
	Date      string `json:"date,omitempty"` // YYYY-MM-DD format, empty means yesterday
	FullCheck bool   `json:"full_check,omitempty"`
}

// NewReconciliationTask creates a new reconciliation task.
func NewReconciliationTask(date string, fullCheck bool) (*asynq.Task, error) {
	payload, err := json.Marshal(ReconciliationPayload{
		Date:      date,
		FullCheck: fullCheck,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeReconciliation, payload), nil
}

// ParseReconciliationPayload parses the payload of a reconciliation task.
func ParseReconciliationPayload(task *asynq.Task) (*ReconciliationPayload, error) {
	var payload ReconciliationPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		// Default to yesterday
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		return &ReconciliationPayload{Date: yesterday}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.Date == "" {
		payload.Date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	return &payload, nil
}

// ProviderHealthCheckPayload is the payload for provider health check tasks.
type ProviderHealthCheckPayload struct {
	ProviderCode string `json:"provider_code,omitempty"` // Empty means all providers
}

// NewProviderHealthCheckTask creates a new provider health check task.
func NewProviderHealthCheckTask(providerCode string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProviderHealthCheckPayload{
		ProviderCode: providerCode,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProviderHealthCheck, payload), nil
}

// ParseProviderHealthCheckPayload parses the payload of a provider health check task.
func ParseProviderHealthCheckPayload(task *asynq.Task) (*ProviderHealthCheckPayload, error) {
	var payload ProviderHealthCheckPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &ProviderHealthCheckPayload{}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// QuoteExpiryPayload is the payload for quote expiry tasks.
type QuoteExpiryPayload struct {
	BatchSize int `json:"batch_size,omitempty"`
}

// NewQuoteExpiryTask creates a new quote expiry task.
func NewQuoteExpiryTask(batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(QuoteExpiryPayload{
		BatchSize: batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeQuoteExpiry, payload), nil
}

// ParseQuoteExpiryPayload parses the payload of a quote expiry task.
func ParseQuoteExpiryPayload(task *asynq.Task) (*QuoteExpiryPayload, error) {
	var payload QuoteExpiryPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &QuoteExpiryPayload{BatchSize: 1000}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 1000
	}
	return &payload, nil
}

// APIKeyExpiryNotificationPayload is the payload for API key expiry notification tasks.
type APIKeyExpiryNotificationPayload struct {
	DaysBeforeExpiry int `json:"days_before_expiry,omitempty"`
}

// NewAPIKeyExpiryNotificationTask creates a new API key expiry notification task.
func NewAPIKeyExpiryNotificationTask(daysBeforeExpiry int) (*asynq.Task, error) {
	payload, err := json.Marshal(APIKeyExpiryNotificationPayload{
		DaysBeforeExpiry: daysBeforeExpiry,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAPIKeyExpiryNotification, payload), nil
}

// ParseAPIKeyExpiryNotificationPayload parses the payload of an API key expiry notification task.
func ParseAPIKeyExpiryNotificationPayload(task *asynq.Task) (*APIKeyExpiryNotificationPayload, error) {
	var payload APIKeyExpiryNotificationPayload
	// Handle empty payload (scheduled job)
	if len(task.Payload()) == 0 {
		return &APIKeyExpiryNotificationPayload{DaysBeforeExpiry: 7}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.DaysBeforeExpiry == 0 {
		payload.DaysBeforeExpiry = 7
	}
	return &payload, nil
}

// CODSettlementPayload is the payload for COD settlement tasks.
type CODSettlementPayload struct {
	SettlementID string `json:"settlement_id"`
}

// NewCODSettlementTask creates a new COD settlement task.
func NewCODSettlementTask(settlementID string) (*asynq.Task, error) {
	payload, err := json.Marshal(CODSettlementPayload{
		SettlementID: settlementID,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCODSettlement, payload), nil
}

// ParseCODSettlementPayload parses the payload of a COD settlement task.
func ParseCODSettlementPayload(task *asynq.Task) (*CODSettlementPayload, error) {
	var payload CODSettlementPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// WebhookDLQPayload is the payload for webhook DLQ processing tasks.
type WebhookDLQPayload struct {
	DeliveryID string `json:"delivery_id,omitempty"` // Empty means process all DLQ entries
	BatchSize  int    `json:"batch_size,omitempty"`
}

// NewWebhookDLQTask creates a new webhook DLQ task.
func NewWebhookDLQTask(deliveryID string, batchSize int) (*asynq.Task, error) {
	payload, err := json.Marshal(WebhookDLQPayload{
		DeliveryID: deliveryID,
		BatchSize:  batchSize,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWebhookDLQ, payload), nil
}

// ParseWebhookDLQPayload parses the payload of a webhook DLQ task.
func ParseWebhookDLQPayload(task *asynq.Task) (*WebhookDLQPayload, error) {
	var payload WebhookDLQPayload
	if len(task.Payload()) == 0 {
		return &WebhookDLQPayload{BatchSize: 100}, nil
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, err
	}
	if payload.BatchSize == 0 {
		payload.BatchSize = 100
	}
	return &payload, nil
}
