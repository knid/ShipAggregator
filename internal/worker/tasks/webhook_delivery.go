package tasks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
)

const (
	// WebhookDeliveryTimeout is the timeout for webhook HTTP requests.
	WebhookDeliveryTimeout = 10 * time.Second

	// WebhookMaxResponseBodySize is the maximum response body size to store.
	WebhookMaxResponseBodySize = 1000

	// WebhookSignatureHeader is the header name for the webhook signature.
	WebhookSignatureHeader = "X-Webhook-Signature"

	// WebhookTimestampHeader is the header name for the webhook timestamp.
	WebhookTimestampHeader = "X-Webhook-Timestamp"

	// WebhookEventIDHeader is the header name for the webhook event ID.
	WebhookEventIDHeader = "X-Webhook-Event-ID"

	// WebhookEventTypeHeader is the header name for the webhook event type.
	WebhookEventTypeHeader = "X-Webhook-Event-Type"
)

// WebhookDeliveryHandler handles webhook delivery tasks.
type WebhookDeliveryHandler struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	httpClient *http.Client
	logger     *observability.Logger
}

// NewWebhookDeliveryHandler creates a new webhook delivery handler.
func NewWebhookDeliveryHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{
		pool:    pool,
		queries: queries,
		httpClient: &http.Client{
			Timeout: WebhookDeliveryTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects for webhooks
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// ProcessTask handles a webhook delivery task.
func (h *WebhookDeliveryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseWebhookDeliveryPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse webhook delivery payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	deliveryID, err := uuid.Parse(payload.DeliveryID)
	if err != nil {
		return fmt.Errorf("invalid delivery ID: %w", err)
	}

	h.logger.Info("Processing webhook delivery", "delivery_id", deliveryID)

	// Get delivery record
	delivery, err := h.queries.GetWebhookDeliveryByID(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.logger.Warn("Webhook delivery not found", "delivery_id", deliveryID)
			return nil // Don't retry for missing deliveries
		}
		return fmt.Errorf("failed to get delivery: %w", err)
	}

	// Skip if already delivered or in DLQ
	if delivery.Status == string(domain.WebhookDeliveryStatusDelivered) ||
		delivery.Status == string(domain.WebhookDeliveryStatusDLQ) {
		h.logger.Info("Webhook delivery already processed",
			"delivery_id", deliveryID,
			"status", delivery.Status,
		)
		return nil
	}

	// Get the webhook event
	event, err := h.queries.GetWebhookEventByID(ctx, delivery.EventID)
	if err != nil {
		return fmt.Errorf("failed to get webhook event: %w", err)
	}

	// Get the subscription to get the endpoint secret
	subscription, err := h.queries.GetWebhookSubscriptionByID(ctx, delivery.SubscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Get the endpoint for the secret
	endpoint, err := h.queries.GetWebhookEndpointByID(ctx, subscription.EndpointID)
	if err != nil {
		return fmt.Errorf("failed to get endpoint: %w", err)
	}

	// Build the event envelope
	envelope := webhook.BuildEventEnvelope(event)
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	// Deliver the webhook
	result := h.deliverWebhook(ctx, delivery.EndpointUrl, envelopeJSON, event, endpoint.SecretHash)

	// Update delivery record
	if err := h.updateDeliveryResult(ctx, delivery, result); err != nil {
		h.logger.Error("Failed to update delivery result",
			"delivery_id", deliveryID,
			"error", err,
		)
		return err
	}

	// If delivery failed and can retry, return error to trigger retry
	if !result.Success && result.CanRetry {
		return fmt.Errorf("webhook delivery failed: %s", result.ErrorMessage)
	}

	h.logger.Info("Webhook delivery completed",
		"delivery_id", deliveryID,
		"success", result.Success,
		"status_code", result.StatusCode,
	)

	return nil
}

// deliveryResult holds the result of a webhook delivery attempt.
type deliveryResult struct {
	Success        bool
	StatusCode     int
	ResponseBody   string
	ResponseTimeMs int
	ErrorMessage   string
	CanRetry       bool
}

// deliverWebhook performs the actual HTTP request to deliver the webhook.
func (h *WebhookDeliveryHandler) deliverWebhook(
	ctx context.Context,
	url string,
	payload []byte,
	event *db.WebhookEvent,
	secretHash string,
) *deliveryResult {
	startTime := time.Now()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return &deliveryResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create request: %v", err),
			CanRetry:     false,
		}
	}

	// Set headers
	timestamp := time.Now().Unix()
	signature := h.computeSignature(payload, timestamp, secretHash)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "E-Port-Webhook/1.0")
	req.Header.Set(WebhookEventIDHeader, fmt.Sprintf("evt_%s", event.ID.String()))
	req.Header.Set(WebhookEventTypeHeader, event.EventType)
	req.Header.Set(WebhookTimestampHeader, fmt.Sprintf("%d", timestamp))
	req.Header.Set(WebhookSignatureHeader, signature)

	// Send request
	resp, err := h.httpClient.Do(req)
	responseTimeMs := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return &deliveryResult{
			Success:        false,
			ResponseTimeMs: responseTimeMs,
			ErrorMessage:   fmt.Sprintf("request failed: %v", err),
			CanRetry:       true, // Network errors are retryable
		}
	}
	defer resp.Body.Close()

	// Read response body (limited)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, WebhookMaxResponseBodySize))
	responseBody := string(bodyBytes)

	// Check if successful (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Determine if retryable
	canRetry := !success && (resp.StatusCode >= 500 || resp.StatusCode == 429)

	result := &deliveryResult{
		Success:        success,
		StatusCode:     resp.StatusCode,
		ResponseBody:   responseBody,
		ResponseTimeMs: responseTimeMs,
		CanRetry:       canRetry,
	}

	if !success {
		result.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return result
}

// computeSignature computes the HMAC-SHA256 signature for the webhook payload.
func (h *WebhookDeliveryHandler) computeSignature(payload []byte, timestamp int64, secretHash string) string {
	// Create the signed payload: timestamp.payload
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))

	// Compute HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(secretHash))
	mac.Write([]byte(signedPayload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("v1=%s", signature)
}

// updateDeliveryResult updates the delivery record with the result.
func (h *WebhookDeliveryHandler) updateDeliveryResult(ctx context.Context, delivery *db.WebhookDelivery, result *deliveryResult) error {
	var responseStatus *int32
	if result.StatusCode > 0 {
		status := int32(result.StatusCode)
		responseStatus = &status
	}

	var responseTimeMs *int32
	if result.ResponseTimeMs > 0 {
		ms := int32(result.ResponseTimeMs)
		responseTimeMs = &ms
	}

	var responseBody *string
	if result.ResponseBody != "" {
		responseBody = &result.ResponseBody
	}

	var errorMessage *string
	if result.ErrorMessage != "" {
		errorMessage = &result.ErrorMessage
	}

	if result.Success {
		// Mark as delivered using UpdateWebhookDeliverySuccess
		_, err := h.queries.UpdateWebhookDeliverySuccess(ctx, db.UpdateWebhookDeliverySuccessParams{
			ID:                  delivery.ID,
			ResponseStatus:      responseStatus,
			ResponseBodyPreview: responseBody,
			ResponseTimeMs:      responseTimeMs,
		})
		return err
	}

	// Mark as failed using UpdateWebhookDeliveryFailure
	// The query handles retry logic and DLQ automatically based on attempt count
	_, err := h.queries.UpdateWebhookDeliveryFailure(ctx, db.UpdateWebhookDeliveryFailureParams{
		ID:                  delivery.ID,
		ResponseStatus:      responseStatus,
		ResponseBodyPreview: responseBody,
		ResponseTimeMs:      responseTimeMs,
		ErrorMessage:        errorMessage,
	})
	return err
}

// WebhookRetryHandler handles webhook retry tasks.
// It finds pending deliveries that are due for retry and re-enqueues them.
type WebhookRetryHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	client  *asynq.Client
	logger  *observability.Logger
}

// NewWebhookRetryHandler creates a new webhook retry handler.
func NewWebhookRetryHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	client *asynq.Client,
	logger *observability.Logger,
) *WebhookRetryHandler {
	return &WebhookRetryHandler{
		pool:    pool,
		queries: queries,
		client:  client,
		logger:  logger,
	}
}

// ProcessTask handles a webhook retry task.
func (h *WebhookRetryHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseWebhookRetryPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse webhook retry payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Processing webhook retry", "batch_size", payload.BatchSize)

	// Get pending deliveries that are due for retry
	deliveries, err := h.queries.ListPendingWebhookDeliveries(ctx, int32(payload.BatchSize))
	if err != nil {
		return fmt.Errorf("failed to list pending deliveries: %w", err)
	}

	if len(deliveries) == 0 {
		h.logger.Info("No webhook deliveries to retry")
		return nil
	}

	h.logger.Info("Found webhook deliveries for retry", "count", len(deliveries))

	// Enqueue each delivery for processing
	var enqueuedCount, errorCount int
	for _, delivery := range deliveries {
		deliveryTask, err := NewWebhookDeliveryTask(delivery.ID.String())
		if err != nil {
			h.logger.Warn("Failed to create delivery task",
				"delivery_id", delivery.ID,
				"error", err,
			)
			errorCount++
			continue
		}

		_, err = h.client.EnqueueContext(ctx, deliveryTask,
			asynq.Queue(QueueCritical),
			asynq.MaxRetry(MaxRetryWebhookDelivery),
			asynq.Timeout(TimeoutWebhookDelivery),
		)
		if err != nil {
			h.logger.Warn("Failed to enqueue delivery task",
				"delivery_id", delivery.ID,
				"error", err,
			)
			errorCount++
			continue
		}
		enqueuedCount++
	}

	h.logger.Info("Completed webhook retry processing",
		"enqueued", enqueuedCount,
		"errors", errorCount,
		"total", len(deliveries),
	)

	return nil
}

// WebhookDLQHandler handles webhook DLQ processing tasks.
type WebhookDLQHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewWebhookDLQHandler creates a new webhook DLQ handler.
func NewWebhookDLQHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *WebhookDLQHandler {
	return &WebhookDLQHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles a webhook DLQ task.
// This is typically used for manual review or alerting on failed deliveries.
func (h *WebhookDLQHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseWebhookDLQPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse webhook DLQ payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Processing webhook DLQ", "batch_size", payload.BatchSize)

	// Get DLQ deliveries
	deliveries, err := h.queries.ListDLQWebhookDeliveries(ctx, db.ListDLQWebhookDeliveriesParams{
		OrganizationID: pgtype.UUID{Valid: false}, // All organizations
		LimitCount:     int32(payload.BatchSize),
		OffsetCount:    0,
	})
	if err != nil {
		return fmt.Errorf("failed to list DLQ deliveries: %w", err)
	}

	if len(deliveries) == 0 {
		h.logger.Info("No webhook deliveries in DLQ")
		return nil
	}

	h.logger.Warn("Found webhook deliveries in DLQ",
		"count", len(deliveries),
	)

	// Log details for each DLQ entry (for alerting/monitoring)
	for _, delivery := range deliveries {
		h.logger.Warn("Webhook delivery in DLQ",
			"delivery_id", delivery.ID,
			"event_id", delivery.EventID,
			"organization_id", delivery.OrganizationID,
			"event_type", delivery.EventType,
			"endpoint_url", delivery.EndpointUrl,
			"attempts", delivery.Attempts,
			"error_message", safeDerefString(delivery.ErrorMessage),
		)
	}

	// In a production system, you might:
	// 1. Send alerts to Slack/PagerDuty
	// 2. Create support tickets
	// 3. Notify the organization owner
	// 4. Store metrics for dashboards

	return nil
}
