// Package webhook provides webhook publishing and delivery services.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/crypto"
	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// Delivery errors
var (
	ErrDeliveryNotFound = errors.New("webhook: delivery not found")
	ErrEventNotFound    = errors.New("webhook: event not found")
	ErrDeliveryFailed   = errors.New("webhook: delivery failed")
	ErrNotInDLQ         = errors.New("webhook: delivery is not in DLQ")
)

// DeliveryService handles webhook delivery operations.
type DeliveryService struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	httpClient *http.Client
	logger     *observability.Logger
}

// DeliveryConfig contains configuration for the delivery service.
type DeliveryConfig struct {
	// Timeout is the HTTP request timeout.
	Timeout time.Duration
	// MaxResponseBodySize is the maximum response body size to read.
	MaxResponseBodySize int64
}

// DefaultDeliveryConfig returns the default delivery configuration.
func DefaultDeliveryConfig() *DeliveryConfig {
	return &DeliveryConfig{
		Timeout:             30 * time.Second,
		MaxResponseBodySize: 10 * 1024, // 10KB
	}
}

// NewDeliveryService creates a new webhook delivery service.
func NewDeliveryService(pool *pgxpool.Pool, queries *db.Queries, logger *observability.Logger, config *DeliveryConfig) *DeliveryService {
	if config == nil {
		config = DefaultDeliveryConfig()
	}

	return &DeliveryService{
		pool:    pool,
		queries: queries,
		httpClient: &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger,
	}
}

// DeliverRequest represents a request to deliver a webhook.
type DeliverRequest struct {
	// DeliveryID is the delivery to process.
	DeliveryID uuid.UUID
}

// DeliverResult represents the result of a webhook delivery attempt.
type DeliverResult struct {
	// Delivery is the updated delivery record.
	Delivery *db.WebhookDelivery
	// Success indicates if the delivery was successful.
	Success bool
	// StatusCode is the HTTP response status code.
	StatusCode int
	// ResponseTime is the response time in milliseconds.
	ResponseTime int
	// Error is the error message if delivery failed.
	Error string
}

// Deliver attempts to deliver a webhook.
func (s *DeliveryService) Deliver(ctx context.Context, req *DeliverRequest) (*DeliverResult, error) {
	// Get delivery
	delivery, err := s.queries.GetWebhookDeliveryByID(ctx, req.DeliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}

	// Get event
	event, err := s.queries.GetWebhookEventByID(ctx, delivery.EventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	// Get subscription to get the secret
	subscription, err := s.queries.GetWebhookSubscriptionByID(ctx, delivery.SubscriptionID)
	if err != nil {
		s.logger.Warn("Failed to get subscription", "error", err, "subscription_id", delivery.SubscriptionID)
		// Continue without subscription - use endpoint URL from delivery
	}

	// Get endpoint for secret
	var secretHash string
	if subscription != nil {
		endpoint, err := s.queries.GetWebhookEndpointByID(ctx, subscription.EndpointID)
		if err == nil {
			secretHash = endpoint.SecretHash
		}
	}

	// Build event envelope
	envelope := BuildEventEnvelope(event)
	payloadJSON, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to marshal envelope: %w", err)
	}

	// Perform HTTP request
	result := s.performDelivery(ctx, delivery.EndpointUrl, payloadJSON, secretHash)

	// Update delivery record
	var updatedDelivery *db.WebhookDelivery
	if result.Success {
		statusCode := int32(result.StatusCode)
		responseTime := int32(result.ResponseTime)
		var responsePreview *string
		if result.Error != "" {
			responsePreview = &result.Error
		}
		updatedDelivery, err = s.queries.UpdateWebhookDeliverySuccess(ctx, db.UpdateWebhookDeliverySuccessParams{
			ID:                  delivery.ID,
			ResponseStatus:      &statusCode,
			ResponseBodyPreview: responsePreview,
			ResponseTimeMs:      &responseTime,
		})
	} else {
		statusCode := int32(result.StatusCode)
		responseTime := int32(result.ResponseTime)
		var statusCodePtr *int32
		var responseTimePtr *int32
		if result.StatusCode > 0 {
			statusCodePtr = &statusCode
		}
		if result.ResponseTime > 0 {
			responseTimePtr = &responseTime
		}
		var errorMsg *string
		if result.Error != "" {
			errorMsg = &result.Error
		}
		updatedDelivery, err = s.queries.UpdateWebhookDeliveryFailure(ctx, db.UpdateWebhookDeliveryFailureParams{
			ID:                  delivery.ID,
			ResponseStatus:      statusCodePtr,
			ResponseBodyPreview: nil,
			ResponseTimeMs:      responseTimePtr,
			ErrorMessage:        errorMsg,
		})
	}

	if err != nil {
		s.logger.Error("Failed to update delivery", "error", err, "delivery_id", delivery.ID)
		return nil, err
	}

	s.logger.Info("Webhook delivery completed",
		"delivery_id", delivery.ID,
		"success", result.Success,
		"status_code", result.StatusCode,
		"response_time_ms", result.ResponseTime,
	)

	return &DeliverResult{
		Delivery:     updatedDelivery,
		Success:      result.Success,
		StatusCode:   result.StatusCode,
		ResponseTime: result.ResponseTime,
		Error:        result.Error,
	}, nil
}

// performDelivery performs the actual HTTP delivery.
func (s *DeliveryService) performDelivery(ctx context.Context, url string, payload []byte, secretHash string) *DeliverResult {
	start := time.Now()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return &DeliverResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "E-Port-Webhook/1.0")

	// Generate webhook ID
	webhookID := uuid.New().String()
	req.Header.Set("X-Webhook-Id", webhookID)

	// Sign the payload if we have a secret
	if secretHash != "" {
		timestamp := time.Now().Unix()
		signature := crypto.SignWebhook(payload, secretHash, timestamp)
		req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Perform request
	resp, err := s.httpClient.Do(req)
	responseTime := int(time.Since(start).Milliseconds())

	if err != nil {
		return &DeliverResult{
			Success:      false,
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Read response body (limited)
	bodyReader := io.LimitReader(resp.Body, 10*1024) // 10KB limit
	body, _ := io.ReadAll(bodyReader)

	// Check if successful (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	result := &DeliverResult{
		Success:      success,
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
	}

	if !success {
		result.Error = string(body)
		if len(result.Error) > 1000 {
			result.Error = result.Error[:1000]
		}
	}

	return result
}

// ProcessPendingDeliveries processes pending webhook deliveries.
func (s *DeliveryService) ProcessPendingDeliveries(ctx context.Context, limit int32) (int, int, error) {
	// Get pending deliveries
	deliveries, err := s.queries.ListPendingWebhookDeliveries(ctx, limit)
	if err != nil {
		return 0, 0, err
	}

	successCount := 0
	failureCount := 0

	for _, delivery := range deliveries {
		result, err := s.Deliver(ctx, &DeliverRequest{DeliveryID: delivery.ID})
		if err != nil {
			s.logger.Error("Failed to process delivery", "error", err, "delivery_id", delivery.ID)
			failureCount++
			continue
		}

		if result.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	return successCount, failureCount, nil
}

// GetDelivery retrieves a webhook delivery by ID.
func (s *DeliveryService) GetDelivery(ctx context.Context, deliveryID uuid.UUID) (*db.WebhookDelivery, error) {
	delivery, err := s.queries.GetWebhookDeliveryByID(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}
	return delivery, nil
}

// ListDeliveriesByOrganization retrieves deliveries for an organization.
func (s *DeliveryService) ListDeliveriesByOrganization(ctx context.Context, orgID uuid.UUID, status *string, offset, limit int32) ([]*db.ListWebhookDeliveriesByOrganizationRow, int64, error) {
	deliveries, err := s.queries.ListWebhookDeliveriesByOrganization(ctx, db.ListWebhookDeliveriesByOrganizationParams{
		OrganizationID: orgID,
		Status:         status,
		OffsetCount:    offset,
		LimitCount:     limit,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountWebhookDeliveriesByOrganization(ctx, db.CountWebhookDeliveriesByOrganizationParams{
		OrganizationID: orgID,
		Status:         status,
	})
	if err != nil {
		return nil, 0, err
	}

	return deliveries, total, nil
}

// ListDeliveriesByEvent retrieves deliveries for an event.
func (s *DeliveryService) ListDeliveriesByEvent(ctx context.Context, eventID uuid.UUID) ([]*db.WebhookDelivery, error) {
	return s.queries.ListWebhookDeliveriesByEvent(ctx, eventID)
}

// ReplayDelivery replays a failed delivery from the DLQ.
func (s *DeliveryService) ReplayDelivery(ctx context.Context, deliveryID uuid.UUID) (*db.WebhookDelivery, error) {
	// Get delivery to verify it's in DLQ
	delivery, err := s.queries.GetWebhookDeliveryByID(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeliveryNotFound
		}
		return nil, err
	}

	if delivery.Status != string(domain.WebhookDeliveryStatusDLQ) {
		return nil, ErrNotInDLQ
	}

	// Replay the delivery
	return s.queries.ReplayWebhookDelivery(ctx, deliveryID)
}

// GetEvent retrieves a webhook event by ID.
func (s *DeliveryService) GetEvent(ctx context.Context, eventID uuid.UUID) (*db.WebhookEvent, error) {
	event, err := s.queries.GetWebhookEventByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}
	return event, nil
}

// ListEventsByOrganization retrieves events for an organization.
func (s *DeliveryService) ListEventsByOrganization(ctx context.Context, orgID uuid.UUID, eventType, resourceType *string, offset, limit int32) ([]*db.WebhookEvent, int64, error) {
	events, err := s.queries.ListWebhookEventsByOrganization(ctx, db.ListWebhookEventsByOrganizationParams{
		OrganizationID: orgID,
		EventType:      eventType,
		ResourceType:   resourceType,
		OffsetCount:    offset,
		LimitCount:     limit,
	})
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountWebhookEventsByOrganization(ctx, db.CountWebhookEventsByOrganizationParams{
		OrganizationID: orgID,
		EventType:      eventType,
		ResourceType:   resourceType,
	})
	if err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// TestEndpoint sends a test webhook to an endpoint.
func (s *DeliveryService) TestEndpoint(ctx context.Context, endpointID uuid.UUID) (*DeliverResult, error) {
	// Get endpoint
	endpoint, err := s.queries.GetWebhookEndpointByID(ctx, endpointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEndpointNotFound
		}
		return nil, err
	}

	// Create test payload
	testPayload := TestWebhookPayload{
		ID:             fmt.Sprintf("evt_test_%s", uuid.New().String()),
		Type:           "webhook.test",
		APIVersion:     "2026-01-01",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		OrganizationID: fmt.Sprintf("org_%s", endpoint.OrganizationID.String()),
		Data: map[string]interface{}{
			"message":     "This is a test webhook from E-Port",
			"endpoint_id": endpoint.ID.String(),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		},
	}

	payloadJSON, err := json.Marshal(testPayload)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to marshal test payload: %w", err)
	}

	// Perform delivery
	result := s.performDelivery(ctx, endpoint.Url, payloadJSON, endpoint.SecretHash)

	s.logger.Info("Test webhook sent",
		"endpoint_id", endpointID,
		"success", result.Success,
		"status_code", result.StatusCode,
	)

	return result, nil
}

// TestWebhookPayload represents the payload for a test webhook.
type TestWebhookPayload struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	APIVersion     string                 `json:"api_version"`
	CreatedAt      string                 `json:"created_at"`
	OrganizationID string                 `json:"organization_id"`
	Data           map[string]interface{} `json:"data"`
}

// GetDeliveryStats returns delivery statistics for an organization.
func (s *DeliveryService) GetDeliveryStats(ctx context.Context, orgID uuid.UUID) (*DeliveryStats, error) {
	rows, err := s.queries.CountWebhookDeliveriesByStatus(ctx, orgID)
	if err != nil {
		return nil, err
	}

	stats := &DeliveryStats{
		ByStatus: make(map[string]int64),
	}

	for _, row := range rows {
		stats.ByStatus[row.Status] = row.Count
		stats.Total += row.Count
		if row.Status == string(domain.WebhookDeliveryStatusDelivered) {
			stats.Delivered = row.Count
		} else if row.Status == string(domain.WebhookDeliveryStatusDLQ) {
			stats.InDLQ = row.Count
		} else if row.Status == string(domain.WebhookDeliveryStatusPending) {
			stats.Pending = row.Count
		}
	}

	return stats, nil
}

// DeliveryStats represents webhook delivery statistics.
type DeliveryStats struct {
	Total     int64            `json:"total"`
	Delivered int64            `json:"delivered"`
	Pending   int64            `json:"pending"`
	InDLQ     int64            `json:"in_dlq"`
	ByStatus  map[string]int64 `json:"by_status"`
}
