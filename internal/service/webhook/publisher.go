// Package webhook provides webhook publishing and delivery services.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// Publisher errors
var (
	ErrNoSubscribers      = errors.New("webhook: no subscribers for event type")
	ErrInvalidEventType   = errors.New("webhook: invalid event type")
	ErrPublishFailed      = errors.New("webhook: failed to publish event")
	ErrEndpointNotFound   = errors.New("webhook: endpoint not found")
	ErrSubscriptionExists = errors.New("webhook: subscription already exists")
)

// Publisher handles webhook event publishing via the outbox pattern.
type Publisher struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewPublisher creates a new webhook publisher.
func NewPublisher(pool *pgxpool.Pool, queries *db.Queries, logger *observability.Logger) *Publisher {
	return &Publisher{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// PublishEventRequest represents a request to publish a webhook event.
type PublishEventRequest struct {
	// OrganizationID is the organization that owns the resource.
	OrganizationID uuid.UUID
	// EventType is the type of event to publish.
	EventType domain.WebhookEventType
	// ResourceID is the ID of the resource that triggered the event.
	ResourceID uuid.UUID
	// Payload is the event payload data.
	Payload interface{}
}

// PublishEventResult represents the result of publishing a webhook event.
type PublishEventResult struct {
	// Event is the created webhook event.
	Event *db.WebhookEvent
	// DeliveriesCreated is the number of delivery records created.
	DeliveriesCreated int
	// OutboxMessage is the outbox message created for async processing.
	OutboxMessage *db.Outbox
}

// PublishEvent publishes a webhook event to all subscribed endpoints.
// It uses the outbox pattern to ensure reliable delivery.
func (p *Publisher) PublishEvent(ctx context.Context, req *PublishEventRequest) (*PublishEventResult, error) {
	// Validate request
	if req.OrganizationID == uuid.Nil {
		return nil, ErrInvalidEventType
	}
	if !req.EventType.IsValid() {
		return nil, ErrInvalidEventType
	}
	if req.ResourceID == uuid.Nil {
		return nil, ErrInvalidEventType
	}

	// Marshal payload
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		p.logger.Error("Failed to marshal webhook payload", "error", err)
		return nil, fmt.Errorf("webhook: failed to marshal payload: %w", err)
	}

	// Start transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		p.logger.Error("Failed to begin transaction", "error", err)
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := p.queries.WithTx(tx)

	// Create webhook event
	event, err := qtx.CreateWebhookEvent(ctx, db.CreateWebhookEventParams{
		OrganizationID: req.OrganizationID,
		EventType:      string(req.EventType),
		ResourceType:   req.EventType.ResourceType(),
		ResourceID:     req.ResourceID,
		Payload:        payloadJSON,
	})
	if err != nil {
		p.logger.Error("Failed to create webhook event", "error", err)
		return nil, err
	}

	// Find all active subscriptions for this event type and organization
	subscriptions, err := qtx.ListWebhookSubscriptionsByOrganizationAndEventType(ctx, db.ListWebhookSubscriptionsByOrganizationAndEventTypeParams{
		OrganizationID: req.OrganizationID,
		EventType:      string(req.EventType),
	})
	if err != nil {
		p.logger.Error("Failed to list subscriptions", "error", err)
		return nil, err
	}

	// Create delivery records for each subscription
	deliveriesCreated := 0
	for _, sub := range subscriptions {
		_, err := qtx.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			EndpointUrl:    sub.Url,
			Status:         string(domain.WebhookDeliveryStatusPending),
		})
		if err != nil {
			p.logger.Warn("Failed to create webhook delivery",
				"error", err,
				"event_id", event.ID,
				"subscription_id", sub.ID,
			)
			continue
		}
		deliveriesCreated++
	}

	// Create outbox message for async processing
	outboxPayload := OutboxEventPayload{
		EventID:        event.ID,
		OrganizationID: req.OrganizationID,
		EventType:      string(req.EventType),
		ResourceID:     req.ResourceID,
	}
	outboxPayloadJSON, err := json.Marshal(outboxPayload)
	if err != nil {
		p.logger.Error("Failed to marshal outbox payload", "error", err)
		return nil, err
	}

	outboxMsg, err := qtx.CreateOutboxMessage(ctx, db.CreateOutboxMessageParams{
		AggregateType: "webhook_event",
		AggregateID:   event.ID,
		EventType:     string(req.EventType),
		Payload:       outboxPayloadJSON,
	})
	if err != nil {
		p.logger.Error("Failed to create outbox message", "error", err)
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		p.logger.Error("Failed to commit transaction", "error", err)
		return nil, err
	}

	p.logger.Info("Published webhook event",
		"event_id", event.ID,
		"event_type", req.EventType,
		"organization_id", req.OrganizationID,
		"deliveries_created", deliveriesCreated,
	)

	return &PublishEventResult{
		Event:             event,
		DeliveriesCreated: deliveriesCreated,
		OutboxMessage:     outboxMsg,
	}, nil
}

// OutboxEventPayload represents the payload stored in the outbox for webhook events.
type OutboxEventPayload struct {
	EventID        uuid.UUID `json:"event_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	EventType      string    `json:"event_type"`
	ResourceID     uuid.UUID `json:"resource_id"`
}

// PublishToOutbox writes an event to the outbox without creating deliveries.
// This is used when the event should be processed asynchronously by a worker.
func (p *Publisher) PublishToOutbox(ctx context.Context, req *PublishEventRequest) (*db.Outbox, error) {
	// Validate request
	if req.OrganizationID == uuid.Nil || !req.EventType.IsValid() || req.ResourceID == uuid.Nil {
		return nil, ErrInvalidEventType
	}

	// Marshal payload
	payloadJSON, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("webhook: failed to marshal payload: %w", err)
	}

	// Create outbox message
	outboxMsg, err := p.queries.CreateOutboxMessage(ctx, db.CreateOutboxMessageParams{
		AggregateType: req.EventType.ResourceType(),
		AggregateID:   req.ResourceID,
		EventType:     string(req.EventType),
		Payload:       payloadJSON,
	})
	if err != nil {
		p.logger.Error("Failed to create outbox message", "error", err)
		return nil, err
	}

	return outboxMsg, nil
}

// ProcessOutboxMessage processes a pending outbox message and creates webhook deliveries.
func (p *Publisher) ProcessOutboxMessage(ctx context.Context, outboxID uuid.UUID) error {
	// Get outbox message
	outbox, err := p.queries.GetOutboxMessageByID(ctx, outboxID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("webhook: outbox message not found: %s", outboxID)
		}
		return err
	}

	// Check if already published
	if outbox.PublishedAt.Valid {
		return nil // Already processed
	}

	// Parse payload
	var payload OutboxEventPayload
	if err := json.Unmarshal(outbox.Payload, &payload); err != nil {
		p.logger.Error("Failed to unmarshal outbox payload", "error", err, "outbox_id", outboxID)
		return err
	}

	// Start transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := p.queries.WithTx(tx)

	// Create webhook event if not exists
	event, err := qtx.GetWebhookEventByID(ctx, payload.EventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Create the event
			eventPayload, _ := json.Marshal(payload)
			event, err = qtx.CreateWebhookEvent(ctx, db.CreateWebhookEventParams{
				OrganizationID: payload.OrganizationID,
				EventType:      payload.EventType,
				ResourceType:   domain.WebhookEventType(payload.EventType).ResourceType(),
				ResourceID:     payload.ResourceID,
				Payload:        eventPayload,
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Find subscriptions
	subscriptions, err := qtx.ListWebhookSubscriptionsByOrganizationAndEventType(ctx, db.ListWebhookSubscriptionsByOrganizationAndEventTypeParams{
		OrganizationID: payload.OrganizationID,
		EventType:      payload.EventType,
	})
	if err != nil {
		return err
	}

	// Create deliveries
	for _, sub := range subscriptions {
		_, err := qtx.CreateWebhookDelivery(ctx, db.CreateWebhookDeliveryParams{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			EndpointUrl:    sub.Url,
			Status:         string(domain.WebhookDeliveryStatusPending),
		})
		if err != nil {
			p.logger.Warn("Failed to create delivery", "error", err, "subscription_id", sub.ID)
		}
	}

	// Mark outbox as published
	_, err = qtx.UpdateOutboxMessagePublished(ctx, outboxID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetPendingOutboxMessages retrieves pending outbox messages for processing.
func (p *Publisher) GetPendingOutboxMessages(ctx context.Context, limit int32) ([]*db.Outbox, error) {
	return p.queries.ListPendingOutboxMessages(ctx, limit)
}

// EventEnvelope represents the standard webhook event envelope.
type EventEnvelope struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	APIVersion     string          `json:"api_version"`
	CreatedAt      string          `json:"created_at"`
	OrganizationID string          `json:"organization_id"`
	Data           json.RawMessage `json:"data"`
}

// BuildEventEnvelope creates a standard event envelope for webhook delivery.
func BuildEventEnvelope(event *db.WebhookEvent) *EventEnvelope {
	return &EventEnvelope{
		ID:             fmt.Sprintf("evt_%s", event.ID.String()),
		Type:           event.EventType,
		APIVersion:     "2026-01-01",
		CreatedAt:      event.CreatedAt.Time.Format(time.RFC3339),
		OrganizationID: fmt.Sprintf("org_%s", event.OrganizationID.String()),
		Data:           event.Payload,
	}
}

// ShipmentEventPayload represents the payload for shipment-related events.
type ShipmentEventPayload struct {
	Shipment      interface{} `json:"shipment"`
	TrackingEvent interface{} `json:"tracking_event,omitempty"`
	Charge        interface{} `json:"charge,omitempty"`
	Cancellation  interface{} `json:"cancellation,omitempty"`
	Delivery      interface{} `json:"delivery,omitempty"`
	Failure       interface{} `json:"failure,omitempty"`
}

// BillingEventPayload represents the payload for billing-related events.
type BillingEventPayload struct {
	Topup       interface{} `json:"topup,omitempty"`
	Wallet      interface{} `json:"wallet,omitempty"`
	LedgerEntry interface{} `json:"ledger_entry,omitempty"`
	Refund      interface{} `json:"refund,omitempty"`
	Settlement  interface{} `json:"settlement,omitempty"`
	Failure     interface{} `json:"failure,omitempty"`
}

// ProviderAccountEventPayload represents the payload for provider account events.
type ProviderAccountEventPayload struct {
	ProviderAccount interface{} `json:"provider_account"`
	Failure         interface{} `json:"failure,omitempty"`
}

// CreateEndpointRequest represents a request to create a webhook endpoint.
type CreateEndpointRequest struct {
	OrganizationID uuid.UUID
	URL            string
	SecretHash     string
	Description    *string
	IsActive       bool
}

// CreateEndpoint creates a new webhook endpoint.
func (p *Publisher) CreateEndpoint(ctx context.Context, req *CreateEndpointRequest) (*db.WebhookEndpoint, error) {
	return p.queries.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
		OrganizationID: req.OrganizationID,
		Url:            req.URL,
		SecretHash:     req.SecretHash,
		Description:    req.Description,
		IsActive:       req.IsActive,
	})
}

// GetEndpoint retrieves a webhook endpoint by ID.
func (p *Publisher) GetEndpoint(ctx context.Context, endpointID uuid.UUID) (*db.WebhookEndpoint, error) {
	endpoint, err := p.queries.GetWebhookEndpointByID(ctx, endpointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEndpointNotFound
		}
		return nil, err
	}
	return endpoint, nil
}

// ListEndpoints retrieves webhook endpoints for an organization.
func (p *Publisher) ListEndpoints(ctx context.Context, orgID uuid.UUID, offset, limit int32) ([]*db.WebhookEndpoint, error) {
	return p.queries.ListWebhookEndpointsByOrganization(ctx, db.ListWebhookEndpointsByOrganizationParams{
		OrganizationID: orgID,
		OffsetCount:    offset,
		LimitCount:     limit,
	})
}

// UpdateEndpointRequest represents a request to update a webhook endpoint.
type UpdateEndpointRequest struct {
	ID          uuid.UUID
	URL         *string
	SecretHash  *string
	Description *string
	IsActive    *bool
}

// UpdateEndpoint updates a webhook endpoint.
func (p *Publisher) UpdateEndpoint(ctx context.Context, req *UpdateEndpointRequest) (*db.WebhookEndpoint, error) {
	return p.queries.UpdateWebhookEndpoint(ctx, db.UpdateWebhookEndpointParams{
		ID:          req.ID,
		Url:         req.URL,
		SecretHash:  req.SecretHash,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
}

// DeleteEndpoint deletes a webhook endpoint and its subscriptions.
func (p *Publisher) DeleteEndpoint(ctx context.Context, endpointID uuid.UUID) error {
	// Start transaction
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := p.queries.WithTx(tx)

	// Delete subscriptions first
	if err := qtx.DeleteWebhookSubscriptionsByEndpoint(ctx, endpointID); err != nil {
		return err
	}

	// Delete endpoint
	if err := qtx.DeleteWebhookEndpoint(ctx, endpointID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// CreateSubscription creates a new webhook subscription.
func (p *Publisher) CreateSubscription(ctx context.Context, endpointID uuid.UUID, eventType domain.WebhookEventType) (*db.WebhookSubscription, error) {
	// Check if subscription already exists
	exists, err := p.queries.CheckWebhookSubscriptionExists(ctx, db.CheckWebhookSubscriptionExistsParams{
		EndpointID: endpointID,
		EventType:  string(eventType),
	})
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSubscriptionExists
	}

	return p.queries.CreateWebhookSubscription(ctx, db.CreateWebhookSubscriptionParams{
		EndpointID: endpointID,
		EventType:  string(eventType),
		IsActive:   true,
	})
}

// GetSubscription retrieves a webhook subscription by ID.
func (p *Publisher) GetSubscription(ctx context.Context, subscriptionID uuid.UUID) (*db.WebhookSubscription, error) {
	return p.queries.GetWebhookSubscriptionByID(ctx, subscriptionID)
}

// ListSubscriptionsByEndpoint retrieves subscriptions for an endpoint.
func (p *Publisher) ListSubscriptionsByEndpoint(ctx context.Context, endpointID uuid.UUID) ([]*db.WebhookSubscription, error) {
	return p.queries.ListWebhookSubscriptionsByEndpoint(ctx, endpointID)
}

// UpdateSubscription updates a webhook subscription.
func (p *Publisher) UpdateSubscription(ctx context.Context, subscriptionID uuid.UUID, isActive bool) (*db.WebhookSubscription, error) {
	return p.queries.UpdateWebhookSubscription(ctx, db.UpdateWebhookSubscriptionParams{
		ID:       subscriptionID,
		IsActive: isActive,
	})
}

// DeleteSubscription deletes a webhook subscription.
func (p *Publisher) DeleteSubscription(ctx context.Context, subscriptionID uuid.UUID) error {
	return p.queries.DeleteWebhookSubscription(ctx, subscriptionID)
}

// CountEndpoints returns the count of endpoints for an organization.
func (p *Publisher) CountEndpoints(ctx context.Context, orgID uuid.UUID) (int64, error) {
	return p.queries.CountWebhookEndpointsByOrganization(ctx, orgID)
}
