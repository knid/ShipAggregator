package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WebhookEventType represents the type of a webhook event.
type WebhookEventType string

const (
	// Shipment events
	WebhookEventShipmentCreated        WebhookEventType = "shipment.created"
	WebhookEventShipmentLabelPurchased WebhookEventType = "shipment.label_purchased"
	WebhookEventShipmentPickedUp       WebhookEventType = "shipment.picked_up"
	WebhookEventShipmentInTransit      WebhookEventType = "shipment.in_transit"
	WebhookEventShipmentOutForDelivery WebhookEventType = "shipment.out_for_delivery"
	WebhookEventShipmentDelivered      WebhookEventType = "shipment.delivered"
	WebhookEventShipmentDeliveryFailed WebhookEventType = "shipment.delivery_failed"
	WebhookEventShipmentReturning      WebhookEventType = "shipment.returning"
	WebhookEventShipmentReturned       WebhookEventType = "shipment.returned"
	WebhookEventShipmentCancelled      WebhookEventType = "shipment.cancelled"
	WebhookEventShipmentException      WebhookEventType = "shipment.exception"

	// Tracking events
	WebhookEventTrackingUpdated WebhookEventType = "tracking.updated"

	// Billing events
	WebhookEventBillingTopupCompleted WebhookEventType = "billing.topup.completed"
	WebhookEventBillingTopupFailed    WebhookEventType = "billing.topup.failed"
	WebhookEventBillingRefundCreated  WebhookEventType = "billing.refund.created"
	WebhookEventBillingCODSettled     WebhookEventType = "billing.cod.settled"

	// Provider account events
	WebhookEventProviderAccountVerified WebhookEventType = "provider_account.verified"
	WebhookEventProviderAccountFailed   WebhookEventType = "provider_account.verification_failed"
)

// IsValid checks if the webhook event type is valid.
func (t WebhookEventType) IsValid() bool {
	switch t {
	case WebhookEventShipmentCreated, WebhookEventShipmentLabelPurchased,
		WebhookEventShipmentPickedUp, WebhookEventShipmentInTransit,
		WebhookEventShipmentOutForDelivery, WebhookEventShipmentDelivered,
		WebhookEventShipmentDeliveryFailed, WebhookEventShipmentReturning,
		WebhookEventShipmentReturned, WebhookEventShipmentCancelled,
		WebhookEventShipmentException, WebhookEventTrackingUpdated,
		WebhookEventBillingTopupCompleted, WebhookEventBillingTopupFailed,
		WebhookEventBillingRefundCreated, WebhookEventBillingCODSettled,
		WebhookEventProviderAccountVerified, WebhookEventProviderAccountFailed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the webhook event type.
func (t WebhookEventType) String() string {
	return string(t)
}

// ResourceType returns the resource type for the event.
func (t WebhookEventType) ResourceType() string {
	switch t {
	case WebhookEventShipmentCreated, WebhookEventShipmentLabelPurchased,
		WebhookEventShipmentPickedUp, WebhookEventShipmentInTransit,
		WebhookEventShipmentOutForDelivery, WebhookEventShipmentDelivered,
		WebhookEventShipmentDeliveryFailed, WebhookEventShipmentReturning,
		WebhookEventShipmentReturned, WebhookEventShipmentCancelled,
		WebhookEventShipmentException:
		return "shipment"
	case WebhookEventTrackingUpdated:
		return "tracking"
	case WebhookEventBillingTopupCompleted, WebhookEventBillingTopupFailed,
		WebhookEventBillingRefundCreated, WebhookEventBillingCODSettled:
		return "billing"
	case WebhookEventProviderAccountVerified, WebhookEventProviderAccountFailed:
		return "provider_account"
	default:
		return "unknown"
	}
}

// AllWebhookEventTypes returns all valid webhook event types.
func AllWebhookEventTypes() []WebhookEventType {
	return []WebhookEventType{
		WebhookEventShipmentCreated, WebhookEventShipmentLabelPurchased,
		WebhookEventShipmentPickedUp, WebhookEventShipmentInTransit,
		WebhookEventShipmentOutForDelivery, WebhookEventShipmentDelivered,
		WebhookEventShipmentDeliveryFailed, WebhookEventShipmentReturning,
		WebhookEventShipmentReturned, WebhookEventShipmentCancelled,
		WebhookEventShipmentException, WebhookEventTrackingUpdated,
		WebhookEventBillingTopupCompleted, WebhookEventBillingTopupFailed,
		WebhookEventBillingRefundCreated, WebhookEventBillingCODSettled,
		WebhookEventProviderAccountVerified, WebhookEventProviderAccountFailed,
	}
}

// WebhookEndpoint represents a customer webhook endpoint registration.
type WebhookEndpoint struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// URL is the webhook URL.
	URL string `json:"url"`
	// SecretHash is the HMAC secret hash for signature verification.
	SecretHash string `json:"-"`
	// Description is the endpoint description.
	Description *string `json:"description,omitempty"`
	// IsActive indicates if the endpoint is active.
	IsActive bool `json:"is_active"`
	// CreatedAt is the timestamp when the endpoint was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the endpoint was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// Subscriptions contains the event subscriptions (populated on demand).
	Subscriptions []WebhookSubscription `json:"subscriptions,omitempty"`
}

// NewWebhookEndpoint creates a new WebhookEndpoint.
func NewWebhookEndpoint(organizationID uuid.UUID, url, secretHash string) *WebhookEndpoint {
	now := time.Now().UTC()
	return &WebhookEndpoint{
		ID:             uuid.New(),
		OrganizationID: organizationID,
		URL:            url,
		SecretHash:     secretHash,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// SetDescription sets the endpoint description.
func (e *WebhookEndpoint) SetDescription(description string) {
	e.Description = &description
	e.UpdatedAt = time.Now().UTC()
}

// ClearDescription clears the endpoint description.
func (e *WebhookEndpoint) ClearDescription() {
	e.Description = nil
	e.UpdatedAt = time.Now().UTC()
}

// Activate activates the endpoint.
func (e *WebhookEndpoint) Activate() {
	e.IsActive = true
	e.UpdatedAt = time.Now().UTC()
}

// Deactivate deactivates the endpoint.
func (e *WebhookEndpoint) Deactivate() {
	e.IsActive = false
	e.UpdatedAt = time.Now().UTC()
}

// UpdateURL updates the endpoint URL.
func (e *WebhookEndpoint) UpdateURL(url string) {
	e.URL = url
	e.UpdatedAt = time.Now().UTC()
}

// UpdateSecret updates the endpoint secret.
func (e *WebhookEndpoint) UpdateSecret(secretHash string) {
	e.SecretHash = secretHash
	e.UpdatedAt = time.Now().UTC()
}

// HasSubscription checks if the endpoint has a subscription for the given event type.
func (e *WebhookEndpoint) HasSubscription(eventType WebhookEventType) bool {
	for _, sub := range e.Subscriptions {
		if sub.EventType == eventType && sub.IsActive {
			return true
		}
	}
	return false
}

// WebhookSubscription represents an event type subscription for an endpoint.
type WebhookSubscription struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// EndpointID is the parent endpoint ID.
	EndpointID uuid.UUID `json:"endpoint_id"`
	// EventType is the subscribed event type.
	EventType WebhookEventType `json:"event_type"`
	// IsActive indicates if the subscription is active.
	IsActive bool `json:"is_active"`
	// CreatedAt is the timestamp when the subscription was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewWebhookSubscription creates a new WebhookSubscription.
func NewWebhookSubscription(endpointID uuid.UUID, eventType WebhookEventType) *WebhookSubscription {
	return &WebhookSubscription{
		ID:         uuid.New(),
		EndpointID: endpointID,
		EventType:  eventType,
		IsActive:   true,
		CreatedAt:  time.Now().UTC(),
	}
}

// Activate activates the subscription.
func (s *WebhookSubscription) Activate() {
	s.IsActive = true
}

// Deactivate deactivates the subscription.
func (s *WebhookSubscription) Deactivate() {
	s.IsActive = false
}

// WebhookEvent represents a generated webhook event.
type WebhookEvent struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// EventType is the event type.
	EventType WebhookEventType `json:"event_type"`
	// ResourceType is the resource type.
	ResourceType string `json:"resource_type"`
	// ResourceID is the resource identifier.
	ResourceID uuid.UUID `json:"resource_id"`
	// Payload is the event payload.
	Payload json.RawMessage `json:"payload"`
	// CreatedAt is the timestamp when the event was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewWebhookEvent creates a new WebhookEvent.
func NewWebhookEvent(organizationID uuid.UUID, eventType WebhookEventType, resourceID uuid.UUID, payload interface{}) (*WebhookEvent, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &WebhookEvent{
		ID:             uuid.New(),
		OrganizationID: organizationID,
		EventType:      eventType,
		ResourceType:   eventType.ResourceType(),
		ResourceID:     resourceID,
		Payload:        payloadJSON,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// WebhookDeliveryStatus represents the status of a webhook delivery.
type WebhookDeliveryStatus string

const (
	// WebhookDeliveryStatusPending indicates the delivery is pending.
	WebhookDeliveryStatusPending WebhookDeliveryStatus = "pending"
	// WebhookDeliveryStatusDelivered indicates the delivery was successful.
	WebhookDeliveryStatusDelivered WebhookDeliveryStatus = "delivered"
	// WebhookDeliveryStatusFailed indicates the delivery failed.
	WebhookDeliveryStatusFailed WebhookDeliveryStatus = "failed"
	// WebhookDeliveryStatusDLQ indicates the delivery is in the dead letter queue.
	WebhookDeliveryStatusDLQ WebhookDeliveryStatus = "dlq"
)

// IsValid checks if the webhook delivery status is valid.
func (s WebhookDeliveryStatus) IsValid() bool {
	switch s {
	case WebhookDeliveryStatusPending, WebhookDeliveryStatusDelivered,
		WebhookDeliveryStatusFailed, WebhookDeliveryStatusDLQ:
		return true
	default:
		return false
	}
}

// String returns the string representation of the webhook delivery status.
func (s WebhookDeliveryStatus) String() string {
	return string(s)
}

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// EventID is the source event ID.
	EventID uuid.UUID `json:"event_id"`
	// SubscriptionID is the target subscription ID.
	SubscriptionID uuid.UUID `json:"subscription_id"`
	// EndpointURL is the delivery URL (snapshot).
	EndpointURL string `json:"endpoint_url"`
	// Status is the delivery status.
	Status WebhookDeliveryStatus `json:"status"`
	// Attempts is the number of delivery attempts.
	Attempts int `json:"attempts"`
	// LastAttemptAt is the timestamp of the last attempt.
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	// NextRetryAt is the timestamp of the next retry.
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	// ResponseStatus is the HTTP response status code.
	ResponseStatus *int `json:"response_status,omitempty"`
	// ResponseBodyPreview is the response body (truncated).
	ResponseBodyPreview *string `json:"response_body_preview,omitempty"`
	// ResponseTimeMs is the response time in milliseconds.
	ResponseTimeMs *int `json:"response_time_ms,omitempty"`
	// ErrorMessage is the error message.
	ErrorMessage *string `json:"error_message,omitempty"`
	// CreatedAt is the timestamp when the delivery was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the delivery was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewWebhookDelivery creates a new WebhookDelivery.
func NewWebhookDelivery(eventID, subscriptionID uuid.UUID, endpointURL string) *WebhookDelivery {
	now := time.Now().UTC()
	return &WebhookDelivery{
		ID:             uuid.New(),
		EventID:        eventID,
		SubscriptionID: subscriptionID,
		EndpointURL:    endpointURL,
		Status:         WebhookDeliveryStatusPending,
		Attempts:       0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// MaxRetryAttempts is the maximum number of retry attempts.
const MaxRetryAttempts = 7

// RetryDelays defines the delay between retry attempts.
var RetryDelays = []time.Duration{
	0,                // Attempt 1: immediate
	1 * time.Minute,  // Attempt 2: 1 minute
	5 * time.Minute,  // Attempt 3: 5 minutes
	30 * time.Minute, // Attempt 4: 30 minutes
	2 * time.Hour,    // Attempt 5: 2 hours
	8 * time.Hour,    // Attempt 6: 8 hours
	24 * time.Hour,   // Attempt 7: 24 hours
}

// IsPending returns true if the delivery is pending.
func (d *WebhookDelivery) IsPending() bool {
	return d.Status == WebhookDeliveryStatusPending
}

// IsDelivered returns true if the delivery was successful.
func (d *WebhookDelivery) IsDelivered() bool {
	return d.Status == WebhookDeliveryStatusDelivered
}

// IsFailed returns true if the delivery failed.
func (d *WebhookDelivery) IsFailed() bool {
	return d.Status == WebhookDeliveryStatusFailed
}

// IsInDLQ returns true if the delivery is in the dead letter queue.
func (d *WebhookDelivery) IsInDLQ() bool {
	return d.Status == WebhookDeliveryStatusDLQ
}

// CanRetry returns true if the delivery can be retried.
func (d *WebhookDelivery) CanRetry() bool {
	return d.Attempts < MaxRetryAttempts && !d.IsDelivered() && !d.IsInDLQ()
}

// RecordSuccess records a successful delivery.
func (d *WebhookDelivery) RecordSuccess(responseStatus int, responseBody string, responseTimeMs int) {
	now := time.Now().UTC()
	d.Attempts++
	d.LastAttemptAt = &now
	d.Status = WebhookDeliveryStatusDelivered
	d.ResponseStatus = &responseStatus
	d.ResponseTimeMs = &responseTimeMs
	if len(responseBody) > 1000 {
		responseBody = responseBody[:1000]
	}
	d.ResponseBodyPreview = &responseBody
	d.NextRetryAt = nil
	d.ErrorMessage = nil
	d.UpdatedAt = now
}

// RecordFailure records a failed delivery attempt.
func (d *WebhookDelivery) RecordFailure(responseStatus *int, responseBody, errorMessage string, responseTimeMs *int) {
	now := time.Now().UTC()
	d.Attempts++
	d.LastAttemptAt = &now
	d.ResponseStatus = responseStatus
	d.ResponseTimeMs = responseTimeMs

	if len(responseBody) > 1000 {
		responseBody = responseBody[:1000]
	}
	if responseBody != "" {
		d.ResponseBodyPreview = &responseBody
	}

	if errorMessage != "" {
		d.ErrorMessage = &errorMessage
	}

	if d.CanRetry() {
		d.Status = WebhookDeliveryStatusFailed
		nextRetry := now.Add(d.GetNextRetryDelay())
		d.NextRetryAt = &nextRetry
	} else {
		d.Status = WebhookDeliveryStatusDLQ
		d.NextRetryAt = nil
	}

	d.UpdatedAt = now
}

// GetNextRetryDelay returns the delay until the next retry.
func (d *WebhookDelivery) GetNextRetryDelay() time.Duration {
	if d.Attempts >= len(RetryDelays) {
		return RetryDelays[len(RetryDelays)-1]
	}
	return RetryDelays[d.Attempts]
}

// MoveToDLQ moves the delivery to the dead letter queue.
func (d *WebhookDelivery) MoveToDLQ() {
	d.Status = WebhookDeliveryStatusDLQ
	d.NextRetryAt = nil
	d.UpdatedAt = time.Now().UTC()
}

// Replay resets the delivery for replay.
func (d *WebhookDelivery) Replay() {
	now := time.Now().UTC()
	d.Status = WebhookDeliveryStatusPending
	d.Attempts = 0
	d.NextRetryAt = &now
	d.ResponseStatus = nil
	d.ResponseBodyPreview = nil
	d.ResponseTimeMs = nil
	d.ErrorMessage = nil
	d.UpdatedAt = now
}

// Outbox represents a transactional outbox entry for event publishing.
type Outbox struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// AggregateType is the aggregate type (e.g., "shipment", "billing").
	AggregateType string `json:"aggregate_type"`
	// AggregateID is the aggregate identifier.
	AggregateID uuid.UUID `json:"aggregate_id"`
	// EventType is the event type.
	EventType string `json:"event_type"`
	// Payload is the event payload.
	Payload json.RawMessage `json:"payload"`
	// PublishedAt is the publication timestamp (nil = unpublished).
	PublishedAt *time.Time `json:"published_at,omitempty"`
	// RetryCount is the number of publish attempts.
	RetryCount int `json:"retry_count"`
	// CreatedAt is the timestamp when the entry was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewOutbox creates a new Outbox entry.
func NewOutbox(aggregateType string, aggregateID uuid.UUID, eventType string, payload interface{}) (*Outbox, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Outbox{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadJSON,
		RetryCount:    0,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// IsPublished returns true if the entry has been published.
func (o *Outbox) IsPublished() bool {
	return o.PublishedAt != nil
}

// MarkPublished marks the entry as published.
func (o *Outbox) MarkPublished() {
	now := time.Now().UTC()
	o.PublishedAt = &now
}

// IncrementRetry increments the retry count.
func (o *Outbox) IncrementRetry() {
	o.RetryCount++
}
