// Package integration provides webhook handlers for carrier integrations.
package integration

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	"github.com/knid/kargonet/internal/service/tracking"
	"github.com/knid/kargonet/internal/service/webhook"
)

// YurticiWebhookHandler handles tracking webhooks from Yurtiçi Kargo.
type YurticiWebhookHandler struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	trackingService *tracking.Service
	webhookPub      *webhook.Publisher
	logger          *observability.Logger
}

// NewYurticiWebhookHandler creates a new Yurtiçi webhook handler.
func NewYurticiWebhookHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	trackingService *tracking.Service,
	webhookPub *webhook.Publisher,
	logger *observability.Logger,
) *YurticiWebhookHandler {
	return &YurticiWebhookHandler{
		pool:            pool,
		queries:         queries,
		trackingService: trackingService,
		webhookPub:      webhookPub,
		logger:          logger,
	}
}

// YurticiTrackingEvent represents a tracking event from Yurtiçi Kargo.
type YurticiTrackingEvent struct {
	XMLName           xml.Name `xml:"ShipmentEvent"`
	CargoKey          string   `xml:"CargoKey"`
	TrackingNumber    string   `xml:"TrackingNumber"`
	EventCode         string   `xml:"EventCode"`
	EventDescription  string   `xml:"EventDescription"`
	EventDate         string   `xml:"EventDate"`
	EventTime         string   `xml:"EventTime"`
	UnitName          string   `xml:"UnitName"`
	CityName          string   `xml:"CityName"`
	ReceiverName      string   `xml:"ReceiverName,omitempty"`
	ReceiverRelation  string   `xml:"ReceiverRelation,omitempty"`
	DeliverySignature string   `xml:"DeliverySignature,omitempty"`
}

// YurticiWebhookPayload represents the webhook payload from Yurtiçi Kargo.
type YurticiWebhookPayload struct {
	XMLName xml.Name               `xml:"YurticiKargoWebhook"`
	Events  []YurticiTrackingEvent `xml:"Events>ShipmentEvent"`
}

// HandleWebhook handles incoming tracking webhooks from Yurtiçi Kargo.
// TODO(security): Add webhook signature verification for Yurtiçi carrier webhooks.
// Currently, any request with a valid XML payload is accepted without authenticating
// the sender. Yurtiçi should provide an HMAC signature or shared secret mechanism
// to verify that webhooks originate from their servers.
func (h *YurticiWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		h.logger.Error("Failed to read Yurtiçi webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	h.logger.Info("Received Yurtiçi tracking webhook",
		"content_length", len(body),
		"content_type", r.Header.Get("Content-Type"),
	)

	// Parse the webhook payload
	var payload YurticiWebhookPayload
	if err := xml.Unmarshal(body, &payload); err != nil {
		h.logger.Error("Failed to parse Yurtiçi webhook payload", "error", err)
		http.Error(w, "Invalid XML payload", http.StatusBadRequest)
		return
	}

	// Process each tracking event
	var processedCount, errorCount int
	for _, event := range payload.Events {
		if err := h.processEvent(ctx, &event, body); err != nil {
			h.logger.Warn("Failed to process Yurtiçi tracking event",
				"tracking_number", event.TrackingNumber,
				"error", err,
			)
			errorCount++
			continue
		}
		processedCount++
	}

	h.logger.Info("Processed Yurtiçi tracking webhook",
		"processed", processedCount,
		"errors", errorCount,
		"total", len(payload.Events),
	)

	// Return success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// processEvent processes a single tracking event from Yurtiçi.
func (h *YurticiWebhookHandler) processEvent(ctx context.Context, event *YurticiTrackingEvent, rawData []byte) error {
	// Find the shipment by tracking number
	shipment, err := h.queries.GetShipmentByTrackingNumber(ctx, db.GetShipmentByTrackingNumberParams{
		ProviderCode:           string(domain.ProviderCodeYurtici),
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

	// Map Yurtiçi status code to normalized status
	normalizedStatus := h.mapStatusCode(event.EventCode)

	// Process the tracking event
	result, err := h.trackingService.ProcessEvent(ctx, &tracking.ProcessEventRequest{
		ShipmentID:        shipment.ID,
		ProviderCode:      domain.ProviderCodeYurtici,
		StatusCode:        domain.TrackingStatus(normalizedStatus),
		StatusDescription: event.EventDescription,
		LocationCity:      event.CityName,
		LocationDetail:    event.UnitName,
		OccurredAt:        occurredAt,
		RawStatusCode:     event.EventCode,
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

// parseEventTime parses the event date and time from Yurtiçi format.
func (h *YurticiWebhookHandler) parseEventTime(dateStr, timeStr string) (time.Time, error) {
	// Yurtiçi typically uses DD.MM.YYYY and HH:MM format
	combined := fmt.Sprintf("%s %s", dateStr, timeStr)

	// Try common formats
	formats := []string{
		"02.01.2006 15:04",
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

// mapStatusCode maps Yurtiçi status codes to normalized status codes.
func (h *YurticiWebhookHandler) mapStatusCode(yurticiCode string) string {
	// Yurtiçi status code mapping
	// These codes are based on Yurtiçi Kargo's API documentation
	statusMap := map[string]string{
		"KABUL":            string(domain.ShipmentStatusPickedUp),
		"TRANSFER":         string(domain.ShipmentStatusInTransit),
		"DAGITIMDA":        string(domain.ShipmentStatusOutForDelivery),
		"TESLIM":           string(domain.ShipmentStatusDelivered),
		"IADE":             string(domain.ShipmentStatusReturning),
		"IADE_TESLIM":      string(domain.ShipmentStatusReturnedToSender),
		"TESLIM_EDILEMEDI": string(domain.ShipmentStatusDeliveryFailed),
		"IPTAL":            string(domain.ShipmentStatusCancelled),
		"HASAR":            string(domain.ShipmentStatusException),
		"KAYIP":            string(domain.ShipmentStatusException),
		// Numeric codes
		"1":  string(domain.ShipmentStatusPickedUp),
		"2":  string(domain.ShipmentStatusInTransit),
		"3":  string(domain.ShipmentStatusAtHub),
		"4":  string(domain.ShipmentStatusOutForDelivery),
		"5":  string(domain.ShipmentStatusDelivered),
		"6":  string(domain.ShipmentStatusDeliveryFailed),
		"7":  string(domain.ShipmentStatusReturning),
		"8":  string(domain.ShipmentStatusReturnedToSender),
		"9":  string(domain.ShipmentStatusException),
		"10": string(domain.ShipmentStatusCancelled),
	}

	if normalized, ok := statusMap[yurticiCode]; ok {
		return normalized
	}

	// Default to in_transit for unknown codes
	return string(domain.ShipmentStatusInTransit)
}

// publishStatusChangeWebhook publishes a webhook for status changes.
func (h *YurticiWebhookHandler) publishStatusChangeWebhook(ctx context.Context, shipment *db.Shipment, result *tracking.ProcessEventResult) {
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
