// Package integration provides webhook handlers for carrier integrations.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/tracking"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
)

// HepsijetWebhookHandler handles tracking webhooks from Hepsijet.
type HepsijetWebhookHandler struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	trackingService *tracking.Service
	webhookPub      *webhook.Publisher
	logger          *observability.Logger
}

// NewHepsijetWebhookHandler creates a new Hepsijet webhook handler.
func NewHepsijetWebhookHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	trackingService *tracking.Service,
	webhookPub *webhook.Publisher,
	logger *observability.Logger,
) *HepsijetWebhookHandler {
	return &HepsijetWebhookHandler{
		pool:            pool,
		queries:         queries,
		trackingService: trackingService,
		webhookPub:      webhookPub,
		logger:          logger,
	}
}

// HepsijetTrackingEvent represents a tracking event from Hepsijet.
type HepsijetTrackingEvent struct {
	TrackingNumber    string `json:"trackingNumber"`
	OrderNumber       string `json:"orderNumber,omitempty"`
	StatusCode        string `json:"statusCode"`
	StatusDescription string `json:"statusDescription"`
	EventDateTime     string `json:"eventDateTime"`
	LocationCity      string `json:"locationCity,omitempty"`
	LocationDistrict  string `json:"locationDistrict,omitempty"`
	BranchName        string `json:"branchName,omitempty"`
	ReceiverName      string `json:"receiverName,omitempty"`
	ReceiverRelation  string `json:"receiverRelation,omitempty"`
	DeliveryNote      string `json:"deliveryNote,omitempty"`
}

// HepsijetWebhookPayload represents the webhook payload from Hepsijet.
type HepsijetWebhookPayload struct {
	EventType string                  `json:"eventType"`
	Timestamp string                  `json:"timestamp"`
	Events    []HepsijetTrackingEvent `json:"events"`
}

// HandleWebhook handles incoming tracking webhooks from Hepsijet.
func (h *HepsijetWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		h.logger.Error("Failed to read Hepsijet webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	h.logger.Info("Received Hepsijet tracking webhook",
		"content_length", len(body),
		"content_type", r.Header.Get("Content-Type"),
	)

	// Parse the webhook payload
	var payload HepsijetWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("Failed to parse Hepsijet webhook payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Process each tracking event
	var processedCount, errorCount int
	for _, event := range payload.Events {
		if err := h.processEvent(ctx, &event, body); err != nil {
			h.logger.Warn("Failed to process Hepsijet tracking event",
				"tracking_number", event.TrackingNumber,
				"error", err,
			)
			errorCount++
			continue
		}
		processedCount++
	}

	h.logger.Info("Processed Hepsijet tracking webhook",
		"processed", processedCount,
		"errors", errorCount,
		"total", len(payload.Events),
	)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"processed": processedCount,
		"errors":    errorCount,
	})
}

// processEvent processes a single tracking event from Hepsijet.
func (h *HepsijetWebhookHandler) processEvent(ctx context.Context, event *HepsijetTrackingEvent, rawData []byte) error {
	// Find the shipment by tracking number
	shipment, err := h.queries.GetShipmentByTrackingNumber(ctx, db.GetShipmentByTrackingNumberParams{
		ProviderCode:           string(domain.ProviderCodeHepsijet),
		ProviderTrackingNumber: &event.TrackingNumber,
	})
	if err != nil {
		return fmt.Errorf("shipment not found for tracking number %s: %w", event.TrackingNumber, err)
	}

	// Parse event timestamp
	occurredAt, err := h.parseEventTime(event.EventDateTime)
	if err != nil {
		h.logger.Warn("Failed to parse event time, using current time",
			"event_datetime", event.EventDateTime,
			"error", err,
		)
		occurredAt = time.Now()
	}

	// Map Hepsijet status code to normalized status
	normalizedStatus := h.mapStatusCode(event.StatusCode)

	// Build location detail
	locationDetail := event.BranchName
	if event.LocationDistrict != "" {
		if locationDetail != "" {
			locationDetail = event.LocationDistrict + " - " + locationDetail
		} else {
			locationDetail = event.LocationDistrict
		}
	}

	// Process the tracking event
	result, err := h.trackingService.ProcessEvent(ctx, &tracking.ProcessEventRequest{
		ShipmentID:        shipment.ID,
		ProviderCode:      domain.ProviderCodeHepsijet,
		StatusCode:        domain.TrackingStatus(normalizedStatus),
		StatusDescription: event.StatusDescription,
		LocationCity:      event.LocationCity,
		LocationDetail:    locationDetail,
		OccurredAt:        occurredAt,
		RawStatusCode:     event.StatusCode,
		RawData:           rawData,
		Source:            domain.TrackingEventSourceWebhook,
	})
	if err != nil {
		return fmt.Errorf("failed to process tracking event: %w", err)
	}

	// Publish webhook if status changed
	if result.IsNewEvent && result.StatusChanged {
		h.publishStatusChangeWebhook(ctx, shipment, result)
	}

	return nil
}

// parseEventTime parses the event datetime from Hepsijet format.
func (h *HepsijetWebhookHandler) parseEventTime(dateTimeStr string) (time.Time, error) {
	// Hepsijet typically uses ISO 8601 format
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"02.01.2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateTimeStr); err == nil {
			// Set Turkey timezone if not already set
			if t.Location() == time.UTC {
				loc, _ := time.LoadLocation("Europe/Istanbul")
				return t.In(loc), nil
			}
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse datetime: %s", dateTimeStr)
}

// mapStatusCode maps Hepsijet status codes to normalized status codes.
func (h *HepsijetWebhookHandler) mapStatusCode(hepsijetCode string) string {
	// Hepsijet status code mapping
	// These codes are based on Hepsijet's API documentation
	statusMap := map[string]string{
		// Text-based codes
		"CREATED":          string(domain.ShipmentStatusLabelPurchased),
		"PICKED_UP":        string(domain.ShipmentStatusPickedUp),
		"IN_TRANSIT":       string(domain.ShipmentStatusInTransit),
		"AT_BRANCH":        string(domain.ShipmentStatusAtHub),
		"OUT_FOR_DELIVERY": string(domain.ShipmentStatusOutForDelivery),
		"DELIVERED":        string(domain.ShipmentStatusDelivered),
		"DELIVERY_FAILED":  string(domain.ShipmentStatusDeliveryFailed),
		"RETURNING":        string(domain.ShipmentStatusReturning),
		"RETURNED":         string(domain.ShipmentStatusReturnedToSender),
		"CANCELLED":        string(domain.ShipmentStatusCancelled),
		"EXCEPTION":        string(domain.ShipmentStatusException),
		"PICKUP_SCHEDULED": string(domain.ShipmentStatusPickupScheduled),
		// Numeric codes
		"100": string(domain.ShipmentStatusLabelPurchased),
		"200": string(domain.ShipmentStatusPickupScheduled),
		"300": string(domain.ShipmentStatusPickedUp),
		"400": string(domain.ShipmentStatusInTransit),
		"500": string(domain.ShipmentStatusAtHub),
		"600": string(domain.ShipmentStatusOutForDelivery),
		"700": string(domain.ShipmentStatusDelivered),
		"800": string(domain.ShipmentStatusDeliveryFailed),
		"900": string(domain.ShipmentStatusReturning),
		"901": string(domain.ShipmentStatusReturnedToSender),
		"999": string(domain.ShipmentStatusCancelled),
	}

	if normalized, ok := statusMap[hepsijetCode]; ok {
		return normalized
	}

	// Default to in_transit for unknown codes
	return string(domain.ShipmentStatusInTransit)
}

// publishStatusChangeWebhook publishes a webhook for status changes.
func (h *HepsijetWebhookHandler) publishStatusChangeWebhook(ctx context.Context, shipment *db.Shipment, result *tracking.ProcessEventResult) {
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
