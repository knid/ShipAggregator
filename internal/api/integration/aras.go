// Package integration provides webhook handlers for carrier integrations.
package integration

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/tracking"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
)

// ArasWebhookHandler handles tracking webhooks from Aras Kargo.
type ArasWebhookHandler struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	trackingService *tracking.Service
	webhookPub      *webhook.Publisher
	logger          *observability.Logger
}

// NewArasWebhookHandler creates a new Aras webhook handler.
func NewArasWebhookHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	trackingService *tracking.Service,
	webhookPub *webhook.Publisher,
	logger *observability.Logger,
) *ArasWebhookHandler {
	return &ArasWebhookHandler{
		pool:            pool,
		queries:         queries,
		trackingService: trackingService,
		webhookPub:      webhookPub,
		logger:          logger,
	}
}

// ArasTrackingEvent represents a tracking event from Aras Kargo.
type ArasTrackingEvent struct {
	XMLName           xml.Name `xml:"TrackingEvent"`
	TrackingNumber    string   `xml:"TrackingNumber"`
	ShipmentID        string   `xml:"ShipmentID"`
	StatusCode        string   `xml:"StatusCode"`
	StatusDescription string   `xml:"StatusDescription"`
	EventDate         string   `xml:"EventDate"`
	EventTime         string   `xml:"EventTime"`
	LocationCity      string   `xml:"LocationCity"`
	LocationBranch    string   `xml:"LocationBranch"`
	ReceiverName      string   `xml:"ReceiverName,omitempty"`
	DeliveryType      string   `xml:"DeliveryType,omitempty"`
}

// ArasWebhookPayload represents the webhook payload from Aras Kargo.
type ArasWebhookPayload struct {
	XMLName xml.Name            `xml:"ArasWebhook"`
	Events  []ArasTrackingEvent `xml:"Events>TrackingEvent"`
}

// HandleWebhook handles incoming tracking webhooks from Aras Kargo.
func (h *ArasWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		h.logger.Error("Failed to read Aras webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	h.logger.Info("Received Aras tracking webhook",
		"content_length", len(body),
		"content_type", r.Header.Get("Content-Type"),
	)

	// Parse the webhook payload
	var payload ArasWebhookPayload
	if err := xml.Unmarshal(body, &payload); err != nil {
		h.logger.Error("Failed to parse Aras webhook payload", "error", err)
		http.Error(w, "Invalid XML payload", http.StatusBadRequest)
		return
	}

	// Process each tracking event
	var processedCount, errorCount int
	for _, event := range payload.Events {
		if err := h.processEvent(ctx, &event, body); err != nil {
			h.logger.Warn("Failed to process Aras tracking event",
				"tracking_number", event.TrackingNumber,
				"error", err,
			)
			errorCount++
			continue
		}
		processedCount++
	}

	h.logger.Info("Processed Aras tracking webhook",
		"processed", processedCount,
		"errors", errorCount,
		"total", len(payload.Events),
	)

	// Return success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// processEvent processes a single tracking event from Aras.
func (h *ArasWebhookHandler) processEvent(ctx context.Context, event *ArasTrackingEvent, rawData []byte) error {
	// Find the shipment by tracking number
	shipment, err := h.queries.GetShipmentByTrackingNumber(ctx, db.GetShipmentByTrackingNumberParams{
		ProviderCode:           string(domain.ProviderCodeAras),
		ProviderTrackingNumber: &event.TrackingNumber,
	})
	if err != nil {
		return fmt.Errorf("shipment not found for tracking number %s: %w", event.TrackingNumber, err)
	}

	// Parse event timestamp
	occurredAt, err := h.parseEventTime(event.EventDate, event.EventTime)
	if err != nil {
		h.logger.Warn("Failed to parse event time, using current time",
			"event_date", event.EventDate,
			"event_time", event.EventTime,
			"error", err,
		)
		occurredAt = time.Now()
	}

	// Map Aras status code to normalized status
	normalizedStatus := h.mapStatusCode(event.StatusCode)

	// Process the tracking event
	result, err := h.trackingService.ProcessEvent(ctx, &tracking.ProcessEventRequest{
		ShipmentID:        shipment.ID,
		ProviderCode:      domain.ProviderCodeAras,
		StatusCode:        domain.TrackingStatus(normalizedStatus),
		StatusDescription: event.StatusDescription,
		LocationCity:      event.LocationCity,
		LocationDetail:    event.LocationBranch,
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

// parseEventTime parses the event date and time from Aras format.
func (h *ArasWebhookHandler) parseEventTime(dateStr, timeStr string) (time.Time, error) {
	// Aras typically uses DD.MM.YYYY and HH:MM:SS format
	combined := fmt.Sprintf("%s %s", dateStr, timeStr)

	// Try common formats
	formats := []string{
		"02.01.2006 15:04:05",
		"2006-01-02 15:04:05",
		"02/01/2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, combined); err == nil {
			// Set Turkey timezone
			loc, _ := time.LoadLocation("Europe/Istanbul")
			return t.In(loc), nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date/time: %s %s", dateStr, timeStr)
}

// mapStatusCode maps Aras status codes to normalized status codes.
func (h *ArasWebhookHandler) mapStatusCode(arasCode string) string {
	// Aras status code mapping
	statusMap := map[string]string{
		"1":   string(domain.ShipmentStatusPickedUp),
		"2":   string(domain.ShipmentStatusInTransit),
		"3":   string(domain.ShipmentStatusAtHub),
		"4":   string(domain.ShipmentStatusOutForDelivery),
		"5":   string(domain.ShipmentStatusDelivered),
		"6":   string(domain.ShipmentStatusDeliveryFailed),
		"7":   string(domain.ShipmentStatusReturning),
		"8":   string(domain.ShipmentStatusReturnedToSender),
		"9":   string(domain.ShipmentStatusException),
		"10":  string(domain.ShipmentStatusCancelled),
		"100": string(domain.ShipmentStatusLabelPurchased),
		"101": string(domain.ShipmentStatusPickupScheduled),
	}

	if normalized, ok := statusMap[arasCode]; ok {
		return normalized
	}

	// Default to in_transit for unknown codes
	return string(domain.ShipmentStatusInTransit)
}

// publishStatusChangeWebhook publishes a webhook for status changes.
func (h *ArasWebhookHandler) publishStatusChangeWebhook(ctx context.Context, shipment *db.Shipment, result *tracking.ProcessEventResult) {
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

// safeDerefString safely dereferences a string pointer.
func safeDerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ArasTrackingResponse represents the response for Aras tracking queries.
type ArasTrackingResponse struct {
	Success bool                `json:"success"`
	Events  []TrackingEventJSON `json:"events,omitempty"`
	Error   string              `json:"error,omitempty"`
}

// TrackingEventJSON represents a tracking event in JSON format.
type TrackingEventJSON struct {
	ID                string    `json:"id"`
	StatusCode        string    `json:"status_code"`
	StatusDescription string    `json:"status_description"`
	LocationCity      string    `json:"location_city,omitempty"`
	LocationDetail    string    `json:"location_detail,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
	Source            string    `json:"source"`
}

// HandleTrackingQuery handles tracking queries for Aras shipments.
func (h *ArasWebhookHandler) HandleTrackingQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get shipment ID from URL
	shipmentIDStr := r.PathValue("id")
	if shipmentIDStr == "" {
		http.Error(w, "Missing shipment ID", http.StatusBadRequest)
		return
	}

	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		http.Error(w, "Invalid shipment ID", http.StatusBadRequest)
		return
	}

	// Get tracking events for the shipment
	events, err := h.queries.ListTrackingEventsByShipment(ctx, shipmentID)
	if err != nil {
		h.logger.Error("Failed to get tracking events", "shipment_id", shipmentID, "error", err)
		http.Error(w, "Failed to get tracking events", http.StatusInternalServerError)
		return
	}

	// Convert to JSON response
	response := ArasTrackingResponse{
		Success: true,
		Events:  make([]TrackingEventJSON, len(events)),
	}

	for i, event := range events {
		response.Events[i] = TrackingEventJSON{
			ID:                event.ID.String(),
			StatusCode:        event.StatusCode,
			StatusDescription: event.StatusDescription,
			LocationCity:      safeDerefString(event.LocationCity),
			LocationDetail:    safeDerefString(event.LocationDetail),
			OccurredAt:        event.OccurredAt.Time,
			Source:            event.Source,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
