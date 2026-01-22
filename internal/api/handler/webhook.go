// Package handler provides HTTP handlers for the API.
package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/crypto"
	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
)

// WebhookHandler handles webhook-related HTTP requests.
type WebhookHandler struct {
	publisher       *webhook.Publisher
	deliveryService *webhook.DeliveryService
	queries         *db.Queries
	logger          *observability.Logger
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(
	publisher *webhook.Publisher,
	deliveryService *webhook.DeliveryService,
	queries *db.Queries,
	logger *observability.Logger,
) *WebhookHandler {
	return &WebhookHandler{
		publisher:       publisher,
		deliveryService: deliveryService,
		queries:         queries,
		logger:          logger,
	}
}

// ============================================================================
// ENDPOINT HANDLERS (T154)
// ============================================================================

// CreateEndpointRequest represents the request body for creating a webhook endpoint.
type CreateEndpointRequest struct {
	URL         string   `json:"url"`
	Description *string  `json:"description,omitempty"`
	EventTypes  []string `json:"event_types,omitempty"`
}

// CreateEndpoint handles POST /webhooks/endpoints
// Creates a new webhook endpoint.
func (h *WebhookHandler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req CreateEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.URL == "" {
		fieldErrors = append(fieldErrors, NewFieldError("url", "REQUIRED", "URL is required"))
	}

	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	// Generate secret
	secret, secretHash, err := generateWebhookSecret()
	if err != nil {
		h.logger.Error("Failed to generate webhook secret", "error", err)
		WriteError(w, r, NewInternalError("Failed to create endpoint"))
		return
	}

	// Create endpoint
	endpoint, err := h.publisher.CreateEndpoint(ctx, &webhook.CreateEndpointRequest{
		OrganizationID: orgID,
		URL:            req.URL,
		SecretHash:     secretHash,
		Description:    req.Description,
		IsActive:       true,
	})
	if err != nil {
		h.logger.Error("Failed to create endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to create endpoint"))
		return
	}

	// Create subscriptions for requested event types
	var subscriptions []*db.WebhookSubscription
	for _, eventTypeStr := range req.EventTypes {
		eventType := domain.WebhookEventType(eventTypeStr)
		if !eventType.IsValid() {
			continue // Skip invalid event types
		}
		sub, err := h.publisher.CreateSubscription(ctx, endpoint.ID, eventType)
		if err != nil {
			h.logger.Warn("Failed to create subscription", "error", err, "event_type", eventTypeStr)
			continue
		}
		subscriptions = append(subscriptions, sub)
	}

	// Return response with secret (only shown once)
	response := EndpointResponse{
		ID:             endpoint.ID,
		OrganizationID: endpoint.OrganizationID,
		URL:            endpoint.Url,
		Description:    endpoint.Description,
		IsActive:       endpoint.IsActive,
		Secret:         &secret, // Only returned on creation
		CreatedAt:      endpoint.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:      endpoint.UpdatedAt.Time.Format(time.RFC3339),
	}

	// Add subscriptions to response
	for _, sub := range subscriptions {
		response.Subscriptions = append(response.Subscriptions, SubscriptionResponse{
			ID:        sub.ID,
			EventType: sub.EventType,
			IsActive:  sub.IsActive,
			CreatedAt: sub.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusCreated, response)
}

// GetEndpoint handles GET /webhooks/endpoints/{id}
// Returns a specific webhook endpoint.
func (h *WebhookHandler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Get endpoint
	endpoint, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to get endpoint"))
		return
	}

	// Verify ownership
	if endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	// Get subscriptions
	subscriptions, err := h.publisher.ListSubscriptionsByEndpoint(ctx, endpointID)
	if err != nil {
		h.logger.Warn("Failed to get subscriptions", "error", err)
	}

	response := endpointToResponse(endpoint, subscriptions)
	writeJSON(w, http.StatusOK, response)
}

// ListEndpoints handles GET /webhooks/endpoints
// Returns webhook endpoints for the current organization.
func (h *WebhookHandler) ListEndpoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse query parameters
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Get endpoints
	endpoints, err := h.publisher.ListEndpoints(ctx, orgID, int32(offset), int32(limit))
	if err != nil {
		h.logger.Error("Failed to list endpoints", "error", err)
		WriteError(w, r, NewInternalError("Failed to list endpoints"))
		return
	}

	// Get total count
	total, err := h.publisher.CountEndpoints(ctx, orgID)
	if err != nil {
		h.logger.Warn("Failed to count endpoints", "error", err)
		total = int64(len(endpoints))
	}

	// Build response
	response := make([]EndpointResponse, len(endpoints))
	for i, endpoint := range endpoints {
		// Get subscriptions for each endpoint
		subscriptions, _ := h.publisher.ListSubscriptionsByEndpoint(ctx, endpoint.ID)
		response[i] = endpointToResponse(endpoint, subscriptions)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// UpdateEndpointRequest represents the request body for updating a webhook endpoint.
type UpdateEndpointRequest struct {
	URL         *string `json:"url,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// UpdateEndpoint handles PATCH /webhooks/endpoints/{id}
// Updates a webhook endpoint.
func (h *WebhookHandler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Get existing endpoint to verify ownership
	existing, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to update endpoint"))
		return
	}

	if existing.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	var req UpdateEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Update endpoint
	endpoint, err := h.publisher.UpdateEndpoint(ctx, &webhook.UpdateEndpointRequest{
		ID:          endpointID,
		URL:         req.URL,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		h.logger.Error("Failed to update endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to update endpoint"))
		return
	}

	// Get subscriptions
	subscriptions, _ := h.publisher.ListSubscriptionsByEndpoint(ctx, endpointID)

	response := endpointToResponse(endpoint, subscriptions)
	writeJSON(w, http.StatusOK, response)
}

// DeleteEndpoint handles DELETE /webhooks/endpoints/{id}
// Deletes a webhook endpoint.
func (h *WebhookHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Get existing endpoint to verify ownership
	existing, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to delete endpoint"))
		return
	}

	if existing.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	// Delete endpoint
	if err := h.publisher.DeleteEndpoint(ctx, endpointID); err != nil {
		h.logger.Error("Failed to delete endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to delete endpoint"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RotateEndpointSecret handles POST /webhooks/endpoints/{id}/rotate-secret
// Rotates the webhook endpoint secret.
func (h *WebhookHandler) RotateEndpointSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Get existing endpoint to verify ownership
	existing, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to rotate secret"))
		return
	}

	if existing.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	// Generate new secret
	secret, secretHash, err := generateWebhookSecret()
	if err != nil {
		h.logger.Error("Failed to generate webhook secret", "error", err)
		WriteError(w, r, NewInternalError("Failed to rotate secret"))
		return
	}

	// Update endpoint with new secret
	endpoint, err := h.publisher.UpdateEndpoint(ctx, &webhook.UpdateEndpointRequest{
		ID:         endpointID,
		SecretHash: &secretHash,
	})
	if err != nil {
		h.logger.Error("Failed to update endpoint secret", "error", err)
		WriteError(w, r, NewInternalError("Failed to rotate secret"))
		return
	}

	// Get subscriptions
	subscriptions, _ := h.publisher.ListSubscriptionsByEndpoint(ctx, endpointID)

	response := endpointToResponse(endpoint, subscriptions)
	response.Secret = &secret // Return new secret

	writeJSON(w, http.StatusOK, response)
}

// ============================================================================
// SUBSCRIPTION HANDLERS (T155)
// ============================================================================

// CreateSubscriptionRequest represents the request body for creating a subscription.
type CreateSubscriptionRequest struct {
	EventType string `json:"event_type"`
}

// CreateSubscription handles POST /webhooks/endpoints/{id}/subscriptions
// Creates a new subscription for an endpoint.
func (h *WebhookHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Verify endpoint ownership
	endpoint, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to create subscription"))
		return
	}

	if endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate event type
	eventType := domain.WebhookEventType(req.EventType)
	if !eventType.IsValid() {
		WriteError(w, r, NewValidationError("Invalid event type", []ErrorDetail{
			NewFieldError("event_type", "INVALID", "Event type is not valid"),
		}))
		return
	}

	// Create subscription
	subscription, err := h.publisher.CreateSubscription(ctx, endpointID, eventType)
	if err != nil {
		if errors.Is(err, webhook.ErrSubscriptionExists) {
			WriteError(w, r, NewConflictError("Subscription already exists for this event type"))
			return
		}
		h.logger.Error("Failed to create subscription", "error", err)
		WriteError(w, r, NewInternalError("Failed to create subscription"))
		return
	}

	response := SubscriptionResponse{
		ID:         subscription.ID,
		EndpointID: subscription.EndpointID,
		EventType:  subscription.EventType,
		IsActive:   subscription.IsActive,
		CreatedAt:  subscription.CreatedAt.Time.Format(time.RFC3339),
	}

	writeJSON(w, http.StatusCreated, response)
}

// ListSubscriptions handles GET /webhooks/endpoints/{id}/subscriptions
// Returns subscriptions for an endpoint.
func (h *WebhookHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Verify endpoint ownership
	endpoint, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to list subscriptions"))
		return
	}

	if endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	// Get subscriptions
	subscriptions, err := h.publisher.ListSubscriptionsByEndpoint(ctx, endpointID)
	if err != nil {
		h.logger.Error("Failed to list subscriptions", "error", err)
		WriteError(w, r, NewInternalError("Failed to list subscriptions"))
		return
	}

	response := make([]SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		response[i] = SubscriptionResponse{
			ID:         sub.ID,
			EndpointID: sub.EndpointID,
			EventType:  sub.EventType,
			IsActive:   sub.IsActive,
			CreatedAt:  sub.CreatedAt.Time.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateSubscriptionRequest represents the request body for updating a subscription.
type UpdateSubscriptionRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateSubscription handles PATCH /webhooks/subscriptions/{id}
// Updates a subscription.
func (h *WebhookHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get subscription ID from URL
	subscriptionIDStr := chi.URLParam(r, "id")
	subscriptionID, err := uuid.Parse(subscriptionIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid subscription ID"))
		return
	}

	// Get subscription
	subscription, err := h.publisher.GetSubscription(ctx, subscriptionID)
	if err != nil {
		WriteError(w, r, NewNotFoundError("Webhook subscription"))
		return
	}

	// Verify ownership via endpoint
	endpoint, err := h.publisher.GetEndpoint(ctx, subscription.EndpointID)
	if err != nil || endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook subscription"))
		return
	}

	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Update subscription
	updated, err := h.publisher.UpdateSubscription(ctx, subscriptionID, req.IsActive)
	if err != nil {
		h.logger.Error("Failed to update subscription", "error", err)
		WriteError(w, r, NewInternalError("Failed to update subscription"))
		return
	}

	response := SubscriptionResponse{
		ID:         updated.ID,
		EndpointID: updated.EndpointID,
		EventType:  updated.EventType,
		IsActive:   updated.IsActive,
		CreatedAt:  updated.CreatedAt.Time.Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}

// DeleteSubscription handles DELETE /webhooks/subscriptions/{id}
// Deletes a subscription.
func (h *WebhookHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get subscription ID from URL
	subscriptionIDStr := chi.URLParam(r, "id")
	subscriptionID, err := uuid.Parse(subscriptionIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid subscription ID"))
		return
	}

	// Get subscription
	subscription, err := h.publisher.GetSubscription(ctx, subscriptionID)
	if err != nil {
		WriteError(w, r, NewNotFoundError("Webhook subscription"))
		return
	}

	// Verify ownership via endpoint
	endpoint, err := h.publisher.GetEndpoint(ctx, subscription.EndpointID)
	if err != nil || endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook subscription"))
		return
	}

	// Delete subscription
	if err := h.publisher.DeleteSubscription(ctx, subscriptionID); err != nil {
		h.logger.Error("Failed to delete subscription", "error", err)
		WriteError(w, r, NewInternalError("Failed to delete subscription"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================================
// DELIVERY HANDLERS (T156)
// ============================================================================

// ListDeliveries handles GET /webhooks/deliveries
// Returns webhook deliveries for the current organization.
func (h *WebhookHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse query parameters
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	// Get deliveries
	deliveries, total, err := h.deliveryService.ListDeliveriesByOrganization(ctx, orgID, status, int32(offset), int32(limit))
	if err != nil {
		h.logger.Error("Failed to list deliveries", "error", err)
		WriteError(w, r, NewInternalError("Failed to list deliveries"))
		return
	}

	// Build response
	response := make([]DeliveryResponse, len(deliveries))
	for i, d := range deliveries {
		response[i] = deliveryRowToResponse(d)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// GetDelivery handles GET /webhooks/deliveries/{id}
// Returns a specific webhook delivery.
func (h *WebhookHandler) GetDelivery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get delivery ID from URL
	deliveryIDStr := chi.URLParam(r, "id")
	deliveryID, err := uuid.Parse(deliveryIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid delivery ID"))
		return
	}

	// Get delivery
	delivery, err := h.deliveryService.GetDelivery(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, webhook.ErrDeliveryNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook delivery"))
			return
		}
		h.logger.Error("Failed to get delivery", "error", err)
		WriteError(w, r, NewInternalError("Failed to get delivery"))
		return
	}

	// Verify ownership via event
	event, err := h.deliveryService.GetEvent(ctx, delivery.EventID)
	if err != nil || event.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook delivery"))
		return
	}

	response := deliveryToResponse(delivery, event)
	writeJSON(w, http.StatusOK, response)
}

// ReplayDelivery handles POST /webhooks/deliveries/{id}/replay
// Replays a failed webhook delivery.
func (h *WebhookHandler) ReplayDelivery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get delivery ID from URL
	deliveryIDStr := chi.URLParam(r, "id")
	deliveryID, err := uuid.Parse(deliveryIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid delivery ID"))
		return
	}

	// Get delivery to verify ownership
	delivery, err := h.deliveryService.GetDelivery(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, webhook.ErrDeliveryNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook delivery"))
			return
		}
		h.logger.Error("Failed to get delivery", "error", err)
		WriteError(w, r, NewInternalError("Failed to replay delivery"))
		return
	}

	// Verify ownership via event
	event, err := h.deliveryService.GetEvent(ctx, delivery.EventID)
	if err != nil || event.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook delivery"))
		return
	}

	// Replay delivery
	replayed, err := h.deliveryService.ReplayDelivery(ctx, deliveryID)
	if err != nil {
		if errors.Is(err, webhook.ErrNotInDLQ) {
			WriteError(w, r, NewConflictError("Delivery is not in DLQ and cannot be replayed"))
			return
		}
		h.logger.Error("Failed to replay delivery", "error", err)
		WriteError(w, r, NewInternalError("Failed to replay delivery"))
		return
	}

	response := deliveryToResponse(replayed, event)
	writeJSON(w, http.StatusOK, response)
}

// ============================================================================
// TEST ENDPOINT HANDLER (T157)
// ============================================================================

// TestEndpoint handles POST /webhooks/endpoints/{id}/test
// Sends a test webhook to an endpoint.
func (h *WebhookHandler) TestEndpoint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get endpoint ID from URL
	endpointIDStr := chi.URLParam(r, "id")
	endpointID, err := uuid.Parse(endpointIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid endpoint ID"))
		return
	}

	// Verify endpoint ownership
	endpoint, err := h.publisher.GetEndpoint(ctx, endpointID)
	if err != nil {
		if errors.Is(err, webhook.ErrEndpointNotFound) {
			WriteError(w, r, NewNotFoundError("Webhook endpoint"))
			return
		}
		h.logger.Error("Failed to get endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to test endpoint"))
		return
	}

	if endpoint.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Webhook endpoint"))
		return
	}

	// Send test webhook
	result, err := h.deliveryService.TestEndpoint(ctx, endpointID)
	if err != nil {
		h.logger.Error("Failed to test endpoint", "error", err)
		WriteError(w, r, NewInternalError("Failed to test endpoint"))
		return
	}

	response := TestEndpointResponse{
		Success:      result.Success,
		StatusCode:   result.StatusCode,
		ResponseTime: result.ResponseTime,
		Error:        result.Error,
	}

	writeJSON(w, http.StatusOK, response)
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

// EndpointResponse represents a webhook endpoint in API responses.
type EndpointResponse struct {
	ID             uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID              `json:"organization_id"`
	URL            string                 `json:"url"`
	Description    *string                `json:"description,omitempty"`
	IsActive       bool                   `json:"is_active"`
	Secret         *string                `json:"secret,omitempty"` // Only returned on creation/rotation
	Subscriptions  []SubscriptionResponse `json:"subscriptions,omitempty"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

// SubscriptionResponse represents a webhook subscription in API responses.
type SubscriptionResponse struct {
	ID         uuid.UUID `json:"id"`
	EndpointID uuid.UUID `json:"endpoint_id,omitempty"`
	EventType  string    `json:"event_type"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  string    `json:"created_at"`
}

// DeliveryResponse represents a webhook delivery in API responses.
type DeliveryResponse struct {
	ID                  uuid.UUID `json:"id"`
	EventID             uuid.UUID `json:"event_id"`
	SubscriptionID      uuid.UUID `json:"subscription_id"`
	EndpointURL         string    `json:"endpoint_url"`
	Status              string    `json:"status"`
	Attempts            int       `json:"attempts"`
	LastAttemptAt       *string   `json:"last_attempt_at,omitempty"`
	NextRetryAt         *string   `json:"next_retry_at,omitempty"`
	ResponseStatus      *int      `json:"response_status,omitempty"`
	ResponseBodyPreview *string   `json:"response_body_preview,omitempty"`
	ResponseTimeMs      *int      `json:"response_time_ms,omitempty"`
	ErrorMessage        *string   `json:"error_message,omitempty"`
	EventType           string    `json:"event_type,omitempty"`
	ResourceType        string    `json:"resource_type,omitempty"`
	ResourceID          uuid.UUID `json:"resource_id,omitempty"`
	CreatedAt           string    `json:"created_at"`
	UpdatedAt           string    `json:"updated_at"`
}

// TestEndpointResponse represents the response for testing an endpoint.
type TestEndpointResponse struct {
	Success      bool   `json:"success"`
	StatusCode   int    `json:"status_code,omitempty"`
	ResponseTime int    `json:"response_time_ms,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// generateWebhookSecret generates a new webhook secret and its hash.
func generateWebhookSecret() (secret, hash string, err error) {
	// Generate 32 bytes of random data
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}

	// Encode as hex with prefix
	secret = "whsec_" + hex.EncodeToString(secretBytes)

	// Hash the secret for storage
	hash = crypto.HashAPIKey(secret)

	return secret, hash, nil
}

// endpointToResponse converts a database endpoint to an API response.
func endpointToResponse(endpoint *db.WebhookEndpoint, subscriptions []*db.WebhookSubscription) EndpointResponse {
	resp := EndpointResponse{
		ID:             endpoint.ID,
		OrganizationID: endpoint.OrganizationID,
		URL:            endpoint.Url,
		Description:    endpoint.Description,
		IsActive:       endpoint.IsActive,
		CreatedAt:      endpoint.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:      endpoint.UpdatedAt.Time.Format(time.RFC3339),
	}

	for _, sub := range subscriptions {
		resp.Subscriptions = append(resp.Subscriptions, SubscriptionResponse{
			ID:        sub.ID,
			EventType: sub.EventType,
			IsActive:  sub.IsActive,
			CreatedAt: sub.CreatedAt.Time.Format(time.RFC3339),
		})
	}

	return resp
}

// deliveryToResponse converts a database delivery to an API response.
func deliveryToResponse(d *db.WebhookDelivery, event *db.WebhookEvent) DeliveryResponse {
	resp := DeliveryResponse{
		ID:             d.ID,
		EventID:        d.EventID,
		SubscriptionID: d.SubscriptionID,
		EndpointURL:    d.EndpointUrl,
		Status:         d.Status,
		Attempts:       int(d.Attempts),
		CreatedAt:      d.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:      d.UpdatedAt.Time.Format(time.RFC3339),
	}

	if d.LastAttemptAt.Valid {
		t := d.LastAttemptAt.Time.Format(time.RFC3339)
		resp.LastAttemptAt = &t
	}
	if d.NextRetryAt.Valid {
		t := d.NextRetryAt.Time.Format(time.RFC3339)
		resp.NextRetryAt = &t
	}
	if d.ResponseStatus != nil {
		status := int(*d.ResponseStatus)
		resp.ResponseStatus = &status
	}
	if d.ResponseBodyPreview != nil {
		resp.ResponseBodyPreview = d.ResponseBodyPreview
	}
	if d.ResponseTimeMs != nil {
		ms := int(*d.ResponseTimeMs)
		resp.ResponseTimeMs = &ms
	}
	if d.ErrorMessage != nil {
		resp.ErrorMessage = d.ErrorMessage
	}

	if event != nil {
		resp.EventType = event.EventType
		resp.ResourceType = event.ResourceType
		resp.ResourceID = event.ResourceID
	}

	return resp
}

// deliveryRowToResponse converts a delivery row to an API response.
func deliveryRowToResponse(d *db.ListWebhookDeliveriesByOrganizationRow) DeliveryResponse {
	resp := DeliveryResponse{
		ID:             d.ID,
		EventID:        d.EventID,
		SubscriptionID: d.SubscriptionID,
		EndpointURL:    d.EndpointUrl,
		Status:         d.Status,
		Attempts:       int(d.Attempts),
		EventType:      d.EventType,
		ResourceType:   d.ResourceType,
		ResourceID:     d.ResourceID,
		CreatedAt:      d.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:      d.UpdatedAt.Time.Format(time.RFC3339),
	}

	if d.LastAttemptAt.Valid {
		t := d.LastAttemptAt.Time.Format(time.RFC3339)
		resp.LastAttemptAt = &t
	}
	if d.NextRetryAt.Valid {
		t := d.NextRetryAt.Time.Format(time.RFC3339)
		resp.NextRetryAt = &t
	}
	if d.ResponseStatus != nil {
		status := int(*d.ResponseStatus)
		resp.ResponseStatus = &status
	}
	if d.ResponseBodyPreview != nil {
		resp.ResponseBodyPreview = d.ResponseBodyPreview
	}
	if d.ResponseTimeMs != nil {
		ms := int(*d.ResponseTimeMs)
		resp.ResponseTimeMs = &ms
	}
	if d.ErrorMessage != nil {
		resp.ErrorMessage = d.ErrorMessage
	}

	return resp
}

// ListEventTypes handles GET /webhooks/event-types
// Returns all available webhook event types.
func (h *WebhookHandler) ListEventTypes(w http.ResponseWriter, r *http.Request) {
	eventTypes := domain.AllWebhookEventTypes()

	response := make([]EventTypeResponse, len(eventTypes))
	for i, et := range eventTypes {
		response[i] = EventTypeResponse{
			Type:         string(et),
			ResourceType: et.ResourceType(),
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// EventTypeResponse represents a webhook event type in API responses.
type EventTypeResponse struct {
	Type         string `json:"type"`
	ResourceType string `json:"resource_type"`
}
