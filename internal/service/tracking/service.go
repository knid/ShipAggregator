// Package tracking provides tracking event processing and normalization services.
package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// Service errors
var (
	ErrShipmentNotFound     = errors.New("tracking: shipment not found")
	ErrEventAlreadyExists   = errors.New("tracking: event already exists")
	ErrInvalidTrackingEvent = errors.New("tracking: invalid tracking event")
)

// Service provides tracking event processing and normalization.
type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewService creates a new tracking service.
func NewService(pool *pgxpool.Pool, queries *db.Queries, logger *observability.Logger) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessEventRequest represents a request to process a tracking event.
type ProcessEventRequest struct {
	// ShipmentID is the shipment to update.
	ShipmentID uuid.UUID
	// ProviderCode is the carrier provider code.
	ProviderCode domain.ProviderCode
	// StatusCode is the normalized tracking status.
	StatusCode domain.TrackingStatus
	// StatusDescription is the human-readable description.
	StatusDescription string
	// LocationCity is the event location city.
	LocationCity string
	// LocationDetail is the detailed location.
	LocationDetail string
	// OccurredAt is when the event occurred.
	OccurredAt time.Time
	// RawStatusCode is the provider's original status code.
	RawStatusCode string
	// RawData is the full provider response.
	RawData json.RawMessage
	// Source is the event source (webhook, poll, manual).
	Source domain.TrackingEventSource
}

// ProcessEventResult represents the result of processing a tracking event.
type ProcessEventResult struct {
	// Event is the created tracking event.
	Event *db.TrackingEvent
	// Shipment is the updated shipment.
	Shipment *db.Shipment
	// StatusChanged indicates if the shipment status changed.
	StatusChanged bool
	// PreviousStatus is the previous shipment status.
	PreviousStatus string
	// IsNewEvent indicates if this is a new event (not duplicate).
	IsNewEvent bool
}

// ProcessEvent processes a tracking event and updates the shipment status.
// It handles deduplication and status normalization.
func (s *Service) ProcessEvent(ctx context.Context, req *ProcessEventRequest) (*ProcessEventResult, error) {
	// Validate request
	if req.ShipmentID == uuid.Nil {
		return nil, ErrInvalidTrackingEvent
	}
	if !req.StatusCode.IsValid() {
		return nil, ErrInvalidTrackingEvent
	}
	if !req.Source.IsValid() {
		return nil, ErrInvalidTrackingEvent
	}

	// Start transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.Error("Failed to begin transaction", "error", err)
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// Get shipment
	shipment, err := qtx.GetShipmentByID(ctx, req.ShipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		s.logger.Error("Failed to get shipment", "error", err, "shipment_id", req.ShipmentID)
		return nil, err
	}

	// Check for duplicate event
	occurredAt := pgtype.Timestamptz{Time: req.OccurredAt, Valid: true}
	var rawStatusCode *string
	if req.RawStatusCode != "" {
		rawStatusCode = &req.RawStatusCode
	}

	exists, err := qtx.CheckTrackingEventExists(ctx, db.CheckTrackingEventExistsParams{
		ShipmentID:    req.ShipmentID,
		ProviderCode:  string(req.ProviderCode),
		RawStatusCode: rawStatusCode,
		OccurredAt:    occurredAt,
	})
	if err != nil {
		s.logger.Error("Failed to check event exists", "error", err)
		return nil, err
	}

	if exists {
		// Return existing state without creating duplicate
		return &ProcessEventResult{
			Shipment:      shipment,
			StatusChanged: false,
			IsNewEvent:    false,
		}, nil
	}

	// Prepare location fields
	var locationCity, locationDetail *string
	if req.LocationCity != "" {
		locationCity = &req.LocationCity
	}
	if req.LocationDetail != "" {
		locationDetail = &req.LocationDetail
	}

	// Create tracking event
	event, err := qtx.CreateTrackingEvent(ctx, db.CreateTrackingEventParams{
		ShipmentID:        req.ShipmentID,
		ProviderCode:      string(req.ProviderCode),
		StatusCode:        string(req.StatusCode),
		StatusDescription: req.StatusDescription,
		LocationCity:      locationCity,
		LocationDetail:    locationDetail,
		OccurredAt:        occurredAt,
		RawStatusCode:     rawStatusCode,
		RawData:           req.RawData,
		Source:            string(req.Source),
	})
	if err != nil {
		s.logger.Error("Failed to create tracking event", "error", err)
		return nil, err
	}

	// Determine if shipment status should be updated
	previousStatus := shipment.Status
	newShipmentStatus := req.StatusCode.ToShipmentStatus()
	statusChanged := shouldUpdateShipmentStatus(previousStatus, string(newShipmentStatus))

	if statusChanged {
		// Update shipment status
		updatedShipment, err := s.updateShipmentStatus(ctx, qtx, shipment, newShipmentStatus, req.OccurredAt)
		if err != nil {
			s.logger.Error("Failed to update shipment status", "error", err)
			return nil, err
		}
		shipment = updatedShipment
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("Failed to commit transaction", "error", err)
		return nil, err
	}

	s.logger.Info("Processed tracking event",
		"shipment_id", req.ShipmentID,
		"status_code", req.StatusCode,
		"status_changed", statusChanged,
		"source", req.Source,
	)

	return &ProcessEventResult{
		Event:          event,
		Shipment:       shipment,
		StatusChanged:  statusChanged,
		PreviousStatus: previousStatus,
		IsNewEvent:     true,
	}, nil
}

// updateShipmentStatus updates the shipment status based on the tracking event.
func (s *Service) updateShipmentStatus(
	ctx context.Context,
	qtx *db.Queries,
	shipment *db.Shipment,
	newStatus domain.ShipmentStatus,
	occurredAt time.Time,
) (*db.Shipment, error) {
	ts := pgtype.Timestamptz{Time: occurredAt, Valid: true}

	// Use specific update functions for statuses that have timestamp fields
	switch newStatus {
	case domain.ShipmentStatusPickedUp:
		return qtx.UpdateShipmentPickedUp(ctx, db.UpdateShipmentPickedUpParams{
			ID:         shipment.ID,
			PickedUpAt: ts,
		})
	case domain.ShipmentStatusDelivered:
		return qtx.UpdateShipmentDelivered(ctx, db.UpdateShipmentDeliveredParams{
			ID:          shipment.ID,
			DeliveredAt: ts,
		})
	default:
		// For other statuses, use the generic status update
		return qtx.UpdateShipmentStatus(ctx, db.UpdateShipmentStatusParams{
			ID:     shipment.ID,
			Status: string(newStatus),
		})
	}
}

// shouldUpdateShipmentStatus determines if the shipment status should be updated.
// It implements status progression rules to prevent backward transitions.
func shouldUpdateShipmentStatus(currentStatus, newStatus string) bool {
	// Define status order for progression
	statusOrder := map[string]int{
		string(domain.ShipmentStatusDraft):            0,
		string(domain.ShipmentStatusLabelPurchased):   1,
		string(domain.ShipmentStatusPickupScheduled):  2,
		string(domain.ShipmentStatusPickedUp):         3,
		string(domain.ShipmentStatusInTransit):        4,
		string(domain.ShipmentStatusAtHub):            5,
		string(domain.ShipmentStatusOutForDelivery):   6,
		string(domain.ShipmentStatusDelivered):        7,
		string(domain.ShipmentStatusDeliveryFailed):   6, // Same level as out_for_delivery
		string(domain.ShipmentStatusReturning):        7,
		string(domain.ShipmentStatusReturnedToSender): 8,
		string(domain.ShipmentStatusCancelled):        9,
		string(domain.ShipmentStatusException):        5, // Can happen at various stages
	}

	currentOrder, currentExists := statusOrder[currentStatus]
	newOrder, newExists := statusOrder[newStatus]

	if !currentExists || !newExists {
		return true // Unknown status, allow update
	}

	// Allow update if new status is same or higher order
	// Special case: allow delivery_failed after out_for_delivery
	if currentStatus == string(domain.ShipmentStatusOutForDelivery) &&
		newStatus == string(domain.ShipmentStatusDeliveryFailed) {
		return true
	}

	// Special case: allow returning after delivery_failed
	if currentStatus == string(domain.ShipmentStatusDeliveryFailed) &&
		newStatus == string(domain.ShipmentStatusReturning) {
		return true
	}

	return newOrder >= currentOrder
}

// GetTrackingHistory retrieves the complete tracking history for a shipment.
func (s *Service) GetTrackingHistory(ctx context.Context, shipmentID uuid.UUID) (*domain.TrackingHistory, error) {
	// Get shipment
	shipment, err := s.queries.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}

	// Get tracking events
	events, err := s.queries.ListTrackingEventsByShipment(ctx, shipmentID)
	if err != nil {
		return nil, err
	}

	// Build tracking history
	trackingNumber := ""
	if shipment.ProviderTrackingNumber != nil {
		trackingNumber = *shipment.ProviderTrackingNumber
	}

	history := domain.NewTrackingHistory(
		shipmentID,
		trackingNumber,
		domain.ProviderCode(shipment.ProviderCode),
	)

	// Convert and add events
	for _, e := range events {
		domainEvent := dbEventToDomain(e)
		history.AddEvent(*domainEvent)
	}

	// Set current status from shipment
	history.CurrentStatus = domain.TrackingStatus(shipment.Status)

	// Set delivered timestamp if available
	if shipment.DeliveredAt.Valid {
		t := shipment.DeliveredAt.Time
		history.DeliveredAt = &t
	}

	return history, nil
}

// GetLatestEvent retrieves the most recent tracking event for a shipment.
func (s *Service) GetLatestEvent(ctx context.Context, shipmentID uuid.UUID) (*domain.TrackingEvent, error) {
	event, err := s.queries.GetLatestTrackingEvent(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No events yet
		}
		return nil, err
	}

	return dbEventToDomain(event), nil
}

// ListEventsByShipment retrieves all tracking events for a shipment.
func (s *Service) ListEventsByShipment(ctx context.Context, shipmentID uuid.UUID) ([]*domain.TrackingEvent, error) {
	events, err := s.queries.ListTrackingEventsByShipment(ctx, shipmentID)
	if err != nil {
		return nil, err
	}

	result := make([]*domain.TrackingEvent, len(events))
	for i, e := range events {
		result[i] = dbEventToDomain(e)
	}

	return result, nil
}

// NormalizeProviderStatus normalizes a provider-specific status code to a standard tracking status.
func (s *Service) NormalizeProviderStatus(
	providerCode domain.ProviderCode,
	rawStatusCode string,
	mapper domain.StatusMapper,
) (domain.TrackingStatus, string) {
	if mapper == nil {
		return domain.TrackingStatusUnknown, "Unknown status: " + rawStatusCode
	}
	return mapper.MapStatus(rawStatusCode)
}

// ProcessBatchEvents processes multiple tracking events in a single transaction.
func (s *Service) ProcessBatchEvents(ctx context.Context, events []*ProcessEventRequest) ([]*ProcessEventResult, error) {
	if len(events) == 0 {
		return nil, nil
	}

	// Start transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)
	results := make([]*ProcessEventResult, 0, len(events))

	for _, req := range events {
		result, err := s.processEventInTx(ctx, qtx, req)
		if err != nil {
			s.logger.Warn("Failed to process event in batch",
				"error", err,
				"shipment_id", req.ShipmentID,
			)
			continue // Skip failed events, continue with others
		}
		results = append(results, result)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return results, nil
}

// processEventInTx processes a single event within an existing transaction.
func (s *Service) processEventInTx(ctx context.Context, qtx *db.Queries, req *ProcessEventRequest) (*ProcessEventResult, error) {
	// Validate request
	if req.ShipmentID == uuid.Nil || !req.StatusCode.IsValid() || !req.Source.IsValid() {
		return nil, ErrInvalidTrackingEvent
	}

	// Get shipment
	shipment, err := qtx.GetShipmentByID(ctx, req.ShipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}

	// Check for duplicate
	occurredAt := pgtype.Timestamptz{Time: req.OccurredAt, Valid: true}
	var rawStatusCode *string
	if req.RawStatusCode != "" {
		rawStatusCode = &req.RawStatusCode
	}

	exists, err := qtx.CheckTrackingEventExists(ctx, db.CheckTrackingEventExistsParams{
		ShipmentID:    req.ShipmentID,
		ProviderCode:  string(req.ProviderCode),
		RawStatusCode: rawStatusCode,
		OccurredAt:    occurredAt,
	})
	if err != nil {
		return nil, err
	}

	if exists {
		return &ProcessEventResult{
			Shipment:      shipment,
			StatusChanged: false,
			IsNewEvent:    false,
		}, nil
	}

	// Prepare location fields
	var locationCity, locationDetail *string
	if req.LocationCity != "" {
		locationCity = &req.LocationCity
	}
	if req.LocationDetail != "" {
		locationDetail = &req.LocationDetail
	}

	// Create event
	event, err := qtx.CreateTrackingEvent(ctx, db.CreateTrackingEventParams{
		ShipmentID:        req.ShipmentID,
		ProviderCode:      string(req.ProviderCode),
		StatusCode:        string(req.StatusCode),
		StatusDescription: req.StatusDescription,
		LocationCity:      locationCity,
		LocationDetail:    locationDetail,
		OccurredAt:        occurredAt,
		RawStatusCode:     rawStatusCode,
		RawData:           req.RawData,
		Source:            string(req.Source),
	})
	if err != nil {
		return nil, err
	}

	// Check status change
	previousStatus := shipment.Status
	newShipmentStatus := req.StatusCode.ToShipmentStatus()
	statusChanged := shouldUpdateShipmentStatus(previousStatus, string(newShipmentStatus))

	if statusChanged {
		shipment, err = s.updateShipmentStatus(ctx, qtx, shipment, newShipmentStatus, req.OccurredAt)
		if err != nil {
			return nil, err
		}
	}

	return &ProcessEventResult{
		Event:          event,
		Shipment:       shipment,
		StatusChanged:  statusChanged,
		PreviousStatus: previousStatus,
		IsNewEvent:     true,
	}, nil
}

// RebuildTrackingHistory rebuilds the tracking history for a shipment from provider data.
// This is used for admin recovery operations.
func (s *Service) RebuildTrackingHistory(ctx context.Context, shipmentID uuid.UUID) error {
	// Delete existing events
	if err := s.queries.DeleteTrackingEventsByShipment(ctx, shipmentID); err != nil {
		return err
	}

	s.logger.Info("Deleted tracking events for rebuild", "shipment_id", shipmentID)

	// Note: The actual rebuild would require calling the provider's tracking API
	// This is typically done by the tracking poll task or manually triggered
	return nil
}

// dbEventToDomain converts a database tracking event to a domain tracking event.
func dbEventToDomain(e *db.TrackingEvent) *domain.TrackingEvent {
	event := &domain.TrackingEvent{
		ID:                e.ID,
		ShipmentID:        e.ShipmentID,
		ProviderCode:      domain.ProviderCode(e.ProviderCode),
		StatusCode:        domain.TrackingStatus(e.StatusCode),
		StatusDescription: e.StatusDescription,
		LocationCity:      e.LocationCity,
		LocationDetail:    e.LocationDetail,
		OccurredAt:        e.OccurredAt.Time,
		RawStatusCode:     e.RawStatusCode,
		RawData:           e.RawData,
		Source:            domain.TrackingEventSource(e.Source),
		CreatedAt:         e.CreatedAt.Time,
	}

	return event
}
