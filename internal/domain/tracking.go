package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// TrackingStatus represents a normalized tracking status.
type TrackingStatus string

const (
	// TrackingStatusLabelCreated indicates the label was created.
	TrackingStatusLabelCreated TrackingStatus = "label_created"
	// TrackingStatusPickupScheduled indicates pickup was scheduled.
	TrackingStatusPickupScheduled TrackingStatus = "pickup_scheduled"
	// TrackingStatusPickedUp indicates the shipment was picked up.
	TrackingStatusPickedUp TrackingStatus = "picked_up"
	// TrackingStatusInTransit indicates the shipment is in transit.
	TrackingStatusInTransit TrackingStatus = "in_transit"
	// TrackingStatusAtHub indicates the shipment is at a hub.
	TrackingStatusAtHub TrackingStatus = "at_hub"
	// TrackingStatusOutForDelivery indicates the shipment is out for delivery.
	TrackingStatusOutForDelivery TrackingStatus = "out_for_delivery"
	// TrackingStatusDelivered indicates the shipment was delivered.
	TrackingStatusDelivered TrackingStatus = "delivered"
	// TrackingStatusDeliveryFailed indicates delivery failed.
	TrackingStatusDeliveryFailed TrackingStatus = "delivery_failed"
	// TrackingStatusReturning indicates the shipment is being returned.
	TrackingStatusReturning TrackingStatus = "returning"
	// TrackingStatusReturnedToSender indicates the shipment was returned.
	TrackingStatusReturnedToSender TrackingStatus = "returned_to_sender"
	// TrackingStatusException indicates an exception occurred.
	TrackingStatusException TrackingStatus = "exception"
	// TrackingStatusCancelled indicates the shipment was cancelled.
	TrackingStatusCancelled TrackingStatus = "cancelled"
	// TrackingStatusUnknown indicates an unknown status.
	TrackingStatusUnknown TrackingStatus = "unknown"
)

// IsValid checks if the tracking status is valid.
func (s TrackingStatus) IsValid() bool {
	switch s {
	case TrackingStatusLabelCreated, TrackingStatusPickupScheduled, TrackingStatusPickedUp,
		TrackingStatusInTransit, TrackingStatusAtHub, TrackingStatusOutForDelivery,
		TrackingStatusDelivered, TrackingStatusDeliveryFailed, TrackingStatusReturning,
		TrackingStatusReturnedToSender, TrackingStatusException, TrackingStatusCancelled,
		TrackingStatusUnknown:
		return true
	default:
		return false
	}
}

// String returns the string representation of the tracking status.
func (s TrackingStatus) String() string {
	return string(s)
}

// ToShipmentStatus converts a tracking status to a shipment status.
func (s TrackingStatus) ToShipmentStatus() ShipmentStatus {
	switch s {
	case TrackingStatusLabelCreated:
		return ShipmentStatusLabelPurchased
	case TrackingStatusPickupScheduled:
		return ShipmentStatusPickupScheduled
	case TrackingStatusPickedUp:
		return ShipmentStatusPickedUp
	case TrackingStatusInTransit:
		return ShipmentStatusInTransit
	case TrackingStatusAtHub:
		return ShipmentStatusAtHub
	case TrackingStatusOutForDelivery:
		return ShipmentStatusOutForDelivery
	case TrackingStatusDelivered:
		return ShipmentStatusDelivered
	case TrackingStatusDeliveryFailed:
		return ShipmentStatusDeliveryFailed
	case TrackingStatusReturning:
		return ShipmentStatusReturning
	case TrackingStatusReturnedToSender:
		return ShipmentStatusReturnedToSender
	case TrackingStatusCancelled:
		return ShipmentStatusCancelled
	case TrackingStatusException, TrackingStatusUnknown:
		return ShipmentStatusException
	default:
		return ShipmentStatusException
	}
}

// IsFinal returns true if the tracking status is a final state.
func (s TrackingStatus) IsFinal() bool {
	switch s {
	case TrackingStatusDelivered, TrackingStatusReturnedToSender, TrackingStatusCancelled:
		return true
	default:
		return false
	}
}

// TrackingEventSource represents the source of a tracking event.
type TrackingEventSource string

const (
	// TrackingEventSourceWebhook indicates the event came from a provider webhook.
	TrackingEventSourceWebhook TrackingEventSource = "webhook"
	// TrackingEventSourcePoll indicates the event came from polling the provider.
	TrackingEventSourcePoll TrackingEventSource = "poll"
	// TrackingEventSourceManual indicates the event was manually created.
	TrackingEventSourceManual TrackingEventSource = "manual"
)

// IsValid checks if the tracking event source is valid.
func (s TrackingEventSource) IsValid() bool {
	switch s {
	case TrackingEventSourceWebhook, TrackingEventSourcePoll, TrackingEventSourceManual:
		return true
	default:
		return false
	}
}

// String returns the string representation of the tracking event source.
func (s TrackingEventSource) String() string {
	return string(s)
}

// TrackingEvent represents a shipment tracking event.
type TrackingEvent struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// ShipmentID is the parent shipment ID.
	ShipmentID uuid.UUID `json:"shipment_id"`
	// ProviderCode is the source provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// StatusCode is the normalized status code.
	StatusCode TrackingStatus `json:"status_code"`
	// StatusDescription is the human-readable description.
	StatusDescription string `json:"status_description"`
	// LocationCity is the event location city.
	LocationCity *string `json:"location_city,omitempty"`
	// LocationDetail is the detailed location.
	LocationDetail *string `json:"location_detail,omitempty"`
	// OccurredAt is when the event occurred.
	OccurredAt time.Time `json:"occurred_at"`
	// RawStatusCode is the provider's original status code.
	RawStatusCode *string `json:"raw_status_code,omitempty"`
	// RawData is the full provider response.
	RawData json.RawMessage `json:"raw_data,omitempty"`
	// Source is the event source (webhook, poll, manual).
	Source TrackingEventSource `json:"source"`
	// CreatedAt is the record creation timestamp.
	CreatedAt time.Time `json:"created_at"`
}

// NewTrackingEvent creates a new TrackingEvent.
func NewTrackingEvent(
	shipmentID uuid.UUID,
	providerCode ProviderCode,
	statusCode TrackingStatus,
	statusDescription string,
	occurredAt time.Time,
	source TrackingEventSource,
) *TrackingEvent {
	return &TrackingEvent{
		ID:                uuid.New(),
		ShipmentID:        shipmentID,
		ProviderCode:      providerCode,
		StatusCode:        statusCode,
		StatusDescription: statusDescription,
		OccurredAt:        occurredAt,
		Source:            source,
		CreatedAt:         time.Now().UTC(),
	}
}

// SetLocation sets the location information.
func (e *TrackingEvent) SetLocation(city, detail string) {
	if city != "" {
		e.LocationCity = &city
	}
	if detail != "" {
		e.LocationDetail = &detail
	}
}

// SetRawData sets the raw provider data.
func (e *TrackingEvent) SetRawData(rawStatusCode string, rawData json.RawMessage) {
	if rawStatusCode != "" {
		e.RawStatusCode = &rawStatusCode
	}
	e.RawData = rawData
}

// IsFinal returns true if this event represents a final state.
func (e *TrackingEvent) IsFinal() bool {
	return e.StatusCode.IsFinal()
}

// GetShipmentStatus returns the corresponding shipment status.
func (e *TrackingEvent) GetShipmentStatus() ShipmentStatus {
	return e.StatusCode.ToShipmentStatus()
}

// TrackingHistory represents the complete tracking history for a shipment.
type TrackingHistory struct {
	// ShipmentID is the shipment ID.
	ShipmentID uuid.UUID `json:"shipment_id"`
	// TrackingNumber is the provider tracking number.
	TrackingNumber string `json:"tracking_number"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// CurrentStatus is the current tracking status.
	CurrentStatus TrackingStatus `json:"current_status"`
	// Events is the list of tracking events (newest first).
	Events []TrackingEvent `json:"events"`
	// EstimatedDelivery is the estimated delivery date.
	EstimatedDelivery *time.Time `json:"estimated_delivery,omitempty"`
	// DeliveredAt is the actual delivery timestamp.
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// NewTrackingHistory creates a new TrackingHistory.
func NewTrackingHistory(shipmentID uuid.UUID, trackingNumber string, providerCode ProviderCode) *TrackingHistory {
	return &TrackingHistory{
		ShipmentID:     shipmentID,
		TrackingNumber: trackingNumber,
		ProviderCode:   providerCode,
		CurrentStatus:  TrackingStatusUnknown,
		Events:         []TrackingEvent{},
	}
}

// AddEvent adds a tracking event to the history.
func (h *TrackingHistory) AddEvent(event TrackingEvent) {
	h.Events = append([]TrackingEvent{event}, h.Events...)
	h.CurrentStatus = event.StatusCode

	if event.StatusCode == TrackingStatusDelivered {
		h.DeliveredAt = &event.OccurredAt
	}
}

// GetLatestEvent returns the most recent tracking event.
func (h *TrackingHistory) GetLatestEvent() *TrackingEvent {
	if len(h.Events) == 0 {
		return nil
	}
	return &h.Events[0]
}

// IsDelivered returns true if the shipment has been delivered.
func (h *TrackingHistory) IsDelivered() bool {
	return h.CurrentStatus == TrackingStatusDelivered
}

// IsFinal returns true if the tracking is in a final state.
func (h *TrackingHistory) IsFinal() bool {
	return h.CurrentStatus.IsFinal()
}

// ProviderStatusMapping represents a mapping from provider status codes to normalized status.
type ProviderStatusMapping struct {
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// RawStatusCode is the provider's raw status code.
	RawStatusCode string `json:"raw_status_code"`
	// NormalizedStatus is the normalized tracking status.
	NormalizedStatus TrackingStatus `json:"normalized_status"`
	// Description is the default description for this status.
	Description string `json:"description"`
}

// StatusMapper provides status code mapping for a provider.
type StatusMapper interface {
	// MapStatus maps a provider status code to a normalized status.
	MapStatus(rawStatusCode string) (TrackingStatus, string)
	// ProviderCode returns the provider code.
	ProviderCode() ProviderCode
}

// DefaultStatusMapper provides a default implementation of StatusMapper.
type DefaultStatusMapper struct {
	providerCode ProviderCode
	mappings     map[string]ProviderStatusMapping
}

// NewDefaultStatusMapper creates a new DefaultStatusMapper.
func NewDefaultStatusMapper(providerCode ProviderCode, mappings []ProviderStatusMapping) *DefaultStatusMapper {
	m := &DefaultStatusMapper{
		providerCode: providerCode,
		mappings:     make(map[string]ProviderStatusMapping),
	}
	for _, mapping := range mappings {
		m.mappings[mapping.RawStatusCode] = mapping
	}
	return m
}

// MapStatus maps a provider status code to a normalized status.
func (m *DefaultStatusMapper) MapStatus(rawStatusCode string) (TrackingStatus, string) {
	if mapping, ok := m.mappings[rawStatusCode]; ok {
		return mapping.NormalizedStatus, mapping.Description
	}
	return TrackingStatusUnknown, "Unknown status: " + rawStatusCode
}

// ProviderCode returns the provider code.
func (m *DefaultStatusMapper) ProviderCode() ProviderCode {
	return m.providerCode
}
