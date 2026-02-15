// Package tasks provides background task handlers for the worker.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	"github.com/knid/kargonet/internal/service/tracking"
	"github.com/knid/kargonet/internal/service/webhook"
)

// TrackingPollHandler handles tracking poll tasks.
// It polls carrier APIs for tracking updates on active shipments.
type TrackingPollHandler struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	trackingService *tracking.Service
	webhookPub      *webhook.Publisher
	adapterPool     *provider.AdapterPool
	logger          *observability.Logger
	client          *asynq.Client
}

// NewTrackingPollHandler creates a new tracking poll handler.
func NewTrackingPollHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	trackingService *tracking.Service,
	webhookPub *webhook.Publisher,
	adapterPool *provider.AdapterPool,
	logger *observability.Logger,
	client *asynq.Client,
) *TrackingPollHandler {
	return &TrackingPollHandler{
		pool:            pool,
		queries:         queries,
		trackingService: trackingService,
		webhookPub:      webhookPub,
		adapterPool:     adapterPool,
		logger:          logger,
		client:          client,
	}
}

// ProcessTask handles the tracking poll task.
func (h *TrackingPollHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseTrackingPollPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse tracking poll payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting tracking poll",
		"provider_code", payload.ProviderCode,
		"batch_size", payload.BatchSize,
	)

	// Get shipments that need tracking updates
	var providerCodePtr *string
	if payload.ProviderCode != "" {
		providerCodePtr = &payload.ProviderCode
	}

	shipments, err := h.queries.ListShipmentsForTrackingPoll(ctx, db.ListShipmentsForTrackingPollParams{
		ProviderCode: providerCodePtr,
		LimitCount:   int32(payload.BatchSize),
	})
	if err != nil {
		h.logger.Error("Failed to get shipments for tracking", "error", err)
		return fmt.Errorf("failed to get shipments: %w", err)
	}

	if len(shipments) == 0 {
		h.logger.Info("No shipments to poll for tracking")
		return nil
	}

	h.logger.Info("Found shipments for tracking poll", "count", len(shipments))

	// Process each shipment
	var processedCount, errorCount int
	for _, shipment := range shipments {
		if err := h.pollShipmentTracking(ctx, shipment); err != nil {
			h.logger.Warn("Failed to poll tracking for shipment",
				"shipment_id", shipment.ID,
				"error", err,
			)
			errorCount++
			continue
		}
		processedCount++
	}

	h.logger.Info("Completed tracking poll",
		"processed", processedCount,
		"errors", errorCount,
		"total", len(shipments),
	)

	return nil
}

// ProcessStaleTask handles the stale tracking poll task.
// It polls shipments that haven't been updated in 14-30 days, running less frequently.
func (h *TrackingPollHandler) ProcessStaleTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseTrackingPollPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse stale tracking poll payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting stale tracking poll",
		"provider_code", payload.ProviderCode,
		"batch_size", payload.BatchSize,
	)

	var providerCodePtr *string
	if payload.ProviderCode != "" {
		providerCodePtr = &payload.ProviderCode
	}

	shipments, err := h.queries.ListStaleShipmentsForTrackingPoll(ctx, db.ListStaleShipmentsForTrackingPollParams{
		ProviderCode: providerCodePtr,
		LimitCount:   int32(payload.BatchSize),
	})
	if err != nil {
		h.logger.Error("Failed to get stale shipments for tracking", "error", err)
		return fmt.Errorf("failed to get stale shipments: %w", err)
	}

	if len(shipments) == 0 {
		h.logger.Info("No stale shipments to poll for tracking")
		return nil
	}

	h.logger.Info("Found stale shipments for tracking poll", "count", len(shipments))

	var processedCount, errorCount int
	for _, shipment := range shipments {
		// Reuse the same polling logic via the shared row type
		row := &db.ListShipmentsForTrackingPollRow{
			ID:                     shipment.ID,
			OrganizationID:         shipment.OrganizationID,
			ProviderCode:           shipment.ProviderCode,
			ProviderTrackingNumber: shipment.ProviderTrackingNumber,
			Status:                 shipment.Status,
		}
		if err := h.pollShipmentTracking(ctx, row); err != nil {
			h.logger.Warn("Failed to poll tracking for stale shipment",
				"shipment_id", shipment.ID,
				"error", err,
			)
			errorCount++
			continue
		}
		processedCount++
	}

	h.logger.Info("Completed stale tracking poll",
		"processed", processedCount,
		"errors", errorCount,
		"total", len(shipments),
	)

	return nil
}

// pollShipmentTracking polls tracking for a single shipment.
func (h *TrackingPollHandler) pollShipmentTracking(ctx context.Context, shipment *db.ListShipmentsForTrackingPollRow) error {
	// Skip if no tracking number
	if shipment.ProviderTrackingNumber == nil || *shipment.ProviderTrackingNumber == "" {
		return nil
	}

	// Get provider adapter
	adapter, err := h.adapterPool.Get(domain.ProviderCode(shipment.ProviderCode))
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	// Get credentials for the shipment
	creds, err := h.getCredentialsForShipment(ctx, shipment)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Call provider tracking API
	trackingResp, err := adapter.GetTracking(ctx, provider.GetTrackingRequest{
		TrackingNumber: *shipment.ProviderTrackingNumber,
		Credentials:    creds,
	})
	if err != nil {
		return fmt.Errorf("failed to get tracking: %w", err)
	}

	// Process tracking events
	for _, event := range trackingResp.Events {
		result, err := h.trackingService.ProcessEvent(ctx, &tracking.ProcessEventRequest{
			ShipmentID:        shipment.ID,
			ProviderCode:      domain.ProviderCode(shipment.ProviderCode),
			StatusCode:        event.StatusCode,
			StatusDescription: event.StatusDescription,
			LocationCity:      event.LocationCity,
			LocationDetail:    event.LocationDetail,
			OccurredAt:        event.OccurredAt,
			RawStatusCode:     event.RawStatusCode,
			RawData:           event.RawData,
			Source:            domain.TrackingEventSourcePoll,
		})
		if err != nil {
			h.logger.Warn("Failed to process tracking event",
				"shipment_id", shipment.ID,
				"error", err,
			)
			continue
		}

		// Publish webhook if status changed
		if result.IsNewEvent && result.StatusChanged {
			h.publishStatusChangeWebhook(ctx, shipment, result)
		}
	}

	return nil
}

// getCredentialsForShipment retrieves credentials for a shipment.
// For tracking poll, we use the default provider account for the organization.
func (h *TrackingPollHandler) getCredentialsForShipment(ctx context.Context, shipment *db.ListShipmentsForTrackingPollRow) (*domain.ProviderCredentials, error) {
	// Get the default provider account for this organization and provider
	account, err := h.queries.GetDefaultProviderAccount(ctx, db.GetDefaultProviderAccountParams{
		OrganizationID: shipment.OrganizationID,
		ProviderCode:   shipment.ProviderCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No BYOA account, return empty credentials (will use adapter defaults)
			return &domain.ProviderCredentials{}, nil
		}
		return nil, fmt.Errorf("failed to get provider account: %w", err)
	}

	var creds domain.ProviderCredentials
	if err := json.Unmarshal(account.CredentialsEncrypted, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

// publishStatusChangeWebhook publishes a webhook for status changes.
func (h *TrackingPollHandler) publishStatusChangeWebhook(ctx context.Context, shipment *db.ListShipmentsForTrackingPollRow, result *tracking.ProcessEventResult) {
	// Determine event type based on new status
	eventType := getWebhookEventTypeForStatus(result.Shipment.Status)
	if eventType == "" {
		return
	}

	// Build webhook payload
	payload := webhook.ShipmentEventPayload{
		Shipment: map[string]interface{}{
			"id":                       shipment.ID.String(),
			"provider_code":            shipment.ProviderCode,
			"provider_tracking_number": safeDerefString(shipment.ProviderTrackingNumber),
			"previous_status":          result.PreviousStatus,
			"status":                   result.Shipment.Status,
		},
	}

	if result.Event != nil {
		payload.TrackingEvent = map[string]interface{}{
			"id":                 result.Event.ID.String(),
			"status_code":        result.Event.StatusCode,
			"status_description": result.Event.StatusDescription,
			"location_city":      safeDerefString(result.Event.LocationCity),
			"location_detail":    safeDerefString(result.Event.LocationDetail),
			"occurred_at":        result.Event.OccurredAt.Time.Format(time.RFC3339),
			"source":             result.Event.Source,
		}
	}

	// Publish webhook event
	_, err := h.webhookPub.PublishEvent(ctx, &webhook.PublishEventRequest{
		OrganizationID: shipment.OrganizationID,
		EventType:      eventType,
		ResourceID:     shipment.ID,
		Payload:        payload,
	})
	if err != nil {
		h.logger.Warn("Failed to publish webhook event",
			"shipment_id", shipment.ID,
			"event_type", eventType,
			"error", err,
		)
	}
}

// getWebhookEventTypeForStatus returns the webhook event type for a shipment status.
func getWebhookEventTypeForStatus(status string) domain.WebhookEventType {
	switch domain.ShipmentStatus(status) {
	case domain.ShipmentStatusPickedUp:
		return domain.WebhookEventShipmentPickedUp
	case domain.ShipmentStatusInTransit:
		return domain.WebhookEventShipmentInTransit
	case domain.ShipmentStatusOutForDelivery:
		return domain.WebhookEventShipmentOutForDelivery
	case domain.ShipmentStatusDelivered:
		return domain.WebhookEventShipmentDelivered
	case domain.ShipmentStatusDeliveryFailed:
		return domain.WebhookEventShipmentDeliveryFailed
	case domain.ShipmentStatusReturning:
		return domain.WebhookEventShipmentReturning
	case domain.ShipmentStatusReturnedToSender:
		return domain.WebhookEventShipmentReturned
	case domain.ShipmentStatusException:
		return domain.WebhookEventShipmentException
	default:
		return domain.WebhookEventTrackingUpdated
	}
}

// TrackingUpdateHandler handles individual tracking update tasks.
type TrackingUpdateHandler struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	trackingService *tracking.Service
	webhookPub      *webhook.Publisher
	adapterPool     *provider.AdapterPool
	logger          *observability.Logger
}

// NewTrackingUpdateHandler creates a new tracking update handler.
func NewTrackingUpdateHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	trackingService *tracking.Service,
	webhookPub *webhook.Publisher,
	adapterPool *provider.AdapterPool,
	logger *observability.Logger,
) *TrackingUpdateHandler {
	return &TrackingUpdateHandler{
		pool:            pool,
		queries:         queries,
		trackingService: trackingService,
		webhookPub:      webhookPub,
		adapterPool:     adapterPool,
		logger:          logger,
	}
}

// ProcessTask handles a single tracking update task.
func (h *TrackingUpdateHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseTrackingUpdatePayload(task)
	if err != nil {
		h.logger.Error("Failed to parse tracking update payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	shipmentID, err := uuid.Parse(payload.ShipmentID)
	if err != nil {
		return fmt.Errorf("invalid shipment ID: %w", err)
	}

	h.logger.Info("Processing tracking update",
		"shipment_id", shipmentID,
		"provider_code", payload.ProviderCode,
	)

	// Get shipment
	shipment, err := h.queries.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.logger.Warn("Shipment not found for tracking update", "shipment_id", shipmentID)
			return nil // Don't retry for missing shipments
		}
		return fmt.Errorf("failed to get shipment: %w", err)
	}

	// Skip if no tracking number
	if shipment.ProviderTrackingNumber == nil || *shipment.ProviderTrackingNumber == "" {
		h.logger.Warn("Shipment has no tracking number", "shipment_id", shipmentID)
		return nil
	}

	// Get provider adapter
	adapter, err := h.adapterPool.Get(domain.ProviderCode(shipment.ProviderCode))
	if err != nil {
		return fmt.Errorf("failed to get adapter: %w", err)
	}

	// Get credentials
	creds, err := h.getCredentialsForShipment(ctx, shipment)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Call provider tracking API
	trackingResp, err := adapter.GetTracking(ctx, provider.GetTrackingRequest{
		TrackingNumber:     *shipment.ProviderTrackingNumber,
		ProviderShipmentID: safeDerefString(shipment.ProviderShipmentID),
		Credentials:        creds,
	})
	if err != nil {
		return fmt.Errorf("failed to get tracking: %w", err)
	}

	// Process tracking events
	for _, event := range trackingResp.Events {
		result, err := h.trackingService.ProcessEvent(ctx, &tracking.ProcessEventRequest{
			ShipmentID:        shipment.ID,
			ProviderCode:      domain.ProviderCode(shipment.ProviderCode),
			StatusCode:        event.StatusCode,
			StatusDescription: event.StatusDescription,
			LocationCity:      event.LocationCity,
			LocationDetail:    event.LocationDetail,
			OccurredAt:        event.OccurredAt,
			RawStatusCode:     event.RawStatusCode,
			RawData:           event.RawData,
			Source:            domain.TrackingEventSourcePoll,
		})
		if err != nil {
			h.logger.Warn("Failed to process tracking event",
				"shipment_id", shipment.ID,
				"error", err,
			)
			continue
		}

		// Publish webhook if status changed
		if result.IsNewEvent && result.StatusChanged {
			h.publishStatusChangeWebhook(ctx, shipment, result)
		}
	}

	h.logger.Info("Completed tracking update", "shipment_id", shipmentID)
	return nil
}

// getCredentialsForShipment retrieves credentials for a shipment.
func (h *TrackingUpdateHandler) getCredentialsForShipment(ctx context.Context, shipment *db.Shipment) (*domain.ProviderCredentials, error) {
	// If BYOA shipment, get credentials from provider account
	if shipment.ProviderAccountID.Valid {
		accountID := uuid.UUID(shipment.ProviderAccountID.Bytes)
		account, err := h.queries.GetProviderAccountByID(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to get provider account: %w", err)
		}

		var creds domain.ProviderCredentials
		if err := json.Unmarshal(account.CredentialsEncrypted, &creds); err != nil {
			return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
		}

		return &creds, nil
	}

	// For marketplace shipments, try to get default provider account
	account, err := h.queries.GetDefaultProviderAccount(ctx, db.GetDefaultProviderAccountParams{
		OrganizationID: shipment.OrganizationID,
		ProviderCode:   shipment.ProviderCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No BYOA account, return empty credentials (will use adapter defaults)
			return &domain.ProviderCredentials{}, nil
		}
		return nil, fmt.Errorf("failed to get provider account: %w", err)
	}

	var creds domain.ProviderCredentials
	if err := json.Unmarshal(account.CredentialsEncrypted, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

// publishStatusChangeWebhook publishes a webhook for status changes.
func (h *TrackingUpdateHandler) publishStatusChangeWebhook(ctx context.Context, shipment *db.Shipment, result *tracking.ProcessEventResult) {
	eventType := getWebhookEventTypeForStatus(result.Shipment.Status)
	if eventType == "" {
		return
	}

	payload := webhook.ShipmentEventPayload{
		Shipment: map[string]interface{}{
			"id":                       shipment.ID.String(),
			"provider_code":            shipment.ProviderCode,
			"provider_tracking_number": safeDerefString(shipment.ProviderTrackingNumber),
			"previous_status":          result.PreviousStatus,
			"status":                   result.Shipment.Status,
		},
	}

	if result.Event != nil {
		payload.TrackingEvent = map[string]interface{}{
			"id":                 result.Event.ID.String(),
			"status_code":        result.Event.StatusCode,
			"status_description": result.Event.StatusDescription,
			"location_city":      safeDerefString(result.Event.LocationCity),
			"location_detail":    safeDerefString(result.Event.LocationDetail),
			"occurred_at":        result.Event.OccurredAt.Time.Format(time.RFC3339),
			"source":             result.Event.Source,
		}
	}

	_, err := h.webhookPub.PublishEvent(ctx, &webhook.PublishEventRequest{
		OrganizationID: shipment.OrganizationID,
		EventType:      eventType,
		ResourceID:     shipment.ID,
		Payload:        payload,
	})
	if err != nil {
		h.logger.Warn("Failed to publish webhook event",
			"shipment_id", shipment.ID,
			"event_type", eventType,
			"error", err,
		)
	}
}

// Helper functions

func safeDerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
