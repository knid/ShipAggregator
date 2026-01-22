// Package handler provides HTTP handlers for the API.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/shipment"
)

// ShipmentHandler handles shipment-related HTTP requests.
type ShipmentHandler struct {
	shipmentService *shipment.Service
	queries         *db.Queries
	logger          *observability.Logger
}

// NewShipmentHandler creates a new shipment handler.
func NewShipmentHandler(shipmentService *shipment.Service, queries *db.Queries, logger *observability.Logger) *ShipmentHandler {
	return &ShipmentHandler{
		shipmentService: shipmentService,
		queries:         queries,
		logger:          logger,
	}
}

// CreateShipmentRequest represents the request body for creating a shipment.
type CreateShipmentRequest struct {
	IdempotencyKey string                 `json:"idempotency_key"`
	OfferID        uuid.UUID              `json:"offer_id"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// CreateShipment handles POST /shipments
// Creates a new shipment from an offer.
func (h *ShipmentHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req CreateShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.IdempotencyKey == "" {
		fieldErrors = append(fieldErrors, NewFieldError("idempotency_key", "REQUIRED", "Idempotency key is required"))
	}
	if req.OfferID == uuid.Nil {
		fieldErrors = append(fieldErrors, NewFieldError("offer_id", "REQUIRED", "Offer ID is required"))
	}

	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	// Get actor info from context
	actorID, actorType := getActorFromContext(ctx)

	// Create shipment
	result, err := h.shipmentService.CreateFromOffer(ctx, &shipment.CreateFromOfferRequest{
		OrganizationID: orgID,
		OfferID:        req.OfferID,
		IdempotencyKey: req.IdempotencyKey,
		ActorID:        actorID,
		ActorType:      actorType,
		Metadata:       req.Metadata,
	})
	if err != nil {
		h.handleShipmentError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, shipmentToResponse(result.Shipment))
}

// GetShipment handles GET /shipments/{id}
// Returns a specific shipment.
func (h *ShipmentHandler) GetShipment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get shipment ID from URL
	shipmentIDStr := chi.URLParam(r, "id")
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid shipment ID"))
		return
	}

	s, err := h.shipmentService.Get(ctx, shipmentID, orgID)
	if err != nil {
		h.handleShipmentError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, shipmentToResponse(s))
}

// ListShipments handles GET /shipments
// Returns shipments for the current organization.
func (h *ShipmentHandler) ListShipments(w http.ResponseWriter, r *http.Request) {
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

	var providerCode *string
	if p := r.URL.Query().Get("provider_code"); p != "" {
		providerCode = &p
	}

	var createdAfter *time.Time
	if ca := r.URL.Query().Get("created_after"); ca != "" {
		if t, err := time.Parse(time.RFC3339, ca); err == nil {
			createdAfter = &t
		}
	}

	var createdBefore *time.Time
	if cb := r.URL.Query().Get("created_before"); cb != "" {
		if t, err := time.Parse(time.RFC3339, cb); err == nil {
			createdBefore = &t
		}
	}

	result, err := h.shipmentService.List(ctx, &shipment.ListRequest{
		OrganizationID: orgID,
		Status:         status,
		ProviderCode:   providerCode,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list shipments", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list shipments"))
		return
	}

	response := make([]ShipmentResponse, len(result.Shipments))
	for i, s := range result.Shipments {
		response[i] = shipmentToResponse(s)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  result.Total,
		Offset: offset,
		Limit:  limit,
	})
}

// PurchaseLabelRequest represents the request body for purchasing a label.
type PurchaseLabelRequest struct {
	LabelFormat string `json:"label_format,omitempty"`
}

// PurchaseLabel handles POST /shipments/{id}/purchase-label
// Purchases a shipping label for a shipment.
func (h *ShipmentHandler) PurchaseLabel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get shipment ID from URL
	shipmentIDStr := chi.URLParam(r, "id")
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid shipment ID"))
		return
	}

	var req PurchaseLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body, use default format
		req.LabelFormat = "pdf"
	}

	// Parse label format
	labelFormat := domain.LabelFormatPDF
	if req.LabelFormat != "" {
		switch req.LabelFormat {
		case "pdf":
			labelFormat = domain.LabelFormatPDF
		case "zpl":
			labelFormat = domain.LabelFormatZPL
		case "png":
			labelFormat = domain.LabelFormatPNG
		default:
			WriteError(w, r, NewValidationError("Invalid label format", []ErrorDetail{
				NewFieldError("label_format", "INVALID", "Label format must be one of: pdf, zpl, png"),
			}))
			return
		}
	}

	// Get actor info from context
	actorID, actorType := getActorFromContext(ctx)

	// Purchase label
	result, err := h.shipmentService.PurchaseLabel(ctx, &shipment.PurchaseLabelRequest{
		ShipmentID:     shipmentID,
		OrganizationID: orgID,
		LabelFormat:    labelFormat,
		ActorID:        actorID,
		ActorType:      actorType,
	})
	if err != nil {
		h.handleShipmentError(w, r, err)
		return
	}

	response := PurchaseLabelResponse{
		Shipment:       shipmentToResponse(result.Shipment),
		TrackingNumber: result.TrackingNumber,
	}
	if result.LabelDocument != nil {
		response.LabelDocumentID = &result.LabelDocument.ID
	}

	writeJSON(w, http.StatusOK, response)
}

// CancelShipmentRequest represents the request body for cancelling a shipment.
type CancelShipmentRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CancelShipment handles POST /shipments/{id}/cancel
// Cancels a shipment.
func (h *ShipmentHandler) CancelShipment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get shipment ID from URL
	shipmentIDStr := chi.URLParam(r, "id")
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid shipment ID"))
		return
	}

	var req CancelShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req.Reason = "Cancelled by user"
	}

	// Get actor info from context
	actorID, actorType := getActorFromContext(ctx)

	// Cancel shipment
	s, err := h.shipmentService.Cancel(ctx, &shipment.CancelRequest{
		ShipmentID:     shipmentID,
		OrganizationID: orgID,
		Reason:         req.Reason,
		ActorID:        actorID,
		ActorType:      actorType,
	})
	if err != nil {
		h.handleShipmentError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, shipmentToResponse(s))
}

// Response types

// ShipmentResponse represents a shipment in API responses.
type ShipmentResponse struct {
	ID                     uuid.UUID       `json:"id"`
	OrganizationID         uuid.UUID       `json:"organization_id"`
	IdempotencyKey         string          `json:"idempotency_key"`
	OfferID                *uuid.UUID      `json:"offer_id,omitempty"`
	ProviderCode           string          `json:"provider_code"`
	ServiceCode            string          `json:"service_code"`
	ProviderAccountID      *uuid.UUID      `json:"provider_account_id,omitempty"`
	IsMarketplace          bool            `json:"is_marketplace"`
	ProviderShipmentID     *string         `json:"provider_shipment_id,omitempty"`
	ProviderTrackingNumber *string         `json:"provider_tracking_number,omitempty"`
	ProviderBarcode        *string         `json:"provider_barcode,omitempty"`
	OriginAddress          json.RawMessage `json:"origin_address"`
	DestinationAddress     json.RawMessage `json:"destination_address"`
	Parcel                 json.RawMessage `json:"parcel"`
	CODAmountKurus         *int32          `json:"cod_amount_kurus,omitempty"`
	DeclaredValueKurus     *int32          `json:"declared_value_kurus,omitempty"`
	Status                 string          `json:"status"`
	LabelDocumentID        *uuid.UUID      `json:"label_document_id,omitempty"`
	LabelFormat            *string         `json:"label_format,omitempty"`
	PriceChargedKurus      *int32          `json:"price_charged_kurus,omitempty"`
	CostKurus              *int32          `json:"cost_kurus,omitempty"`
	PickupScheduledAt      *string         `json:"pickup_scheduled_at,omitempty"`
	PickedUpAt             *string         `json:"picked_up_at,omitempty"`
	DeliveredAt            *string         `json:"delivered_at,omitempty"`
	CancelledAt            *string         `json:"cancelled_at,omitempty"`
	CancellationReason     *string         `json:"cancellation_reason,omitempty"`
	ReturnShipmentID       *uuid.UUID      `json:"return_shipment_id,omitempty"`
	OriginalShipmentID     *uuid.UUID      `json:"original_shipment_id,omitempty"`
	Metadata               json.RawMessage `json:"metadata,omitempty"`
	CreatedAt              string          `json:"created_at"`
	UpdatedAt              string          `json:"updated_at"`
}

// PurchaseLabelResponse represents the response for purchasing a label.
type PurchaseLabelResponse struct {
	Shipment        ShipmentResponse `json:"shipment"`
	TrackingNumber  string           `json:"tracking_number"`
	LabelDocumentID *uuid.UUID       `json:"label_document_id,omitempty"`
}

// Conversion helpers

func shipmentToResponse(s *db.Shipment) ShipmentResponse {
	resp := ShipmentResponse{
		ID:                     s.ID,
		OrganizationID:         s.OrganizationID,
		IdempotencyKey:         s.IdempotencyKey,
		ProviderCode:           s.ProviderCode,
		ServiceCode:            s.ServiceCode,
		IsMarketplace:          s.IsMarketplace,
		ProviderShipmentID:     s.ProviderShipmentID,
		ProviderTrackingNumber: s.ProviderTrackingNumber,
		ProviderBarcode:        s.ProviderBarcode,
		OriginAddress:          s.OriginAddress,
		DestinationAddress:     s.DestinationAddress,
		Parcel:                 s.Parcel,
		CODAmountKurus:         s.CodAmountKurus,
		DeclaredValueKurus:     s.DeclaredValueKurus,
		Status:                 s.Status,
		LabelFormat:            s.LabelFormat,
		PriceChargedKurus:      s.PriceChargedKurus,
		CostKurus:              s.CostKurus,
		CancellationReason:     s.CancellationReason,
		Metadata:               s.Metadata,
		CreatedAt:              s.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:              s.UpdatedAt.Time.Format(time.RFC3339),
	}

	// Handle optional UUID fields
	if s.OfferID.Valid {
		id := uuid.UUID(s.OfferID.Bytes)
		resp.OfferID = &id
	}
	if s.ProviderAccountID.Valid {
		id := uuid.UUID(s.ProviderAccountID.Bytes)
		resp.ProviderAccountID = &id
	}
	if s.LabelDocumentID.Valid {
		id := uuid.UUID(s.LabelDocumentID.Bytes)
		resp.LabelDocumentID = &id
	}
	if s.ReturnShipmentID.Valid {
		id := uuid.UUID(s.ReturnShipmentID.Bytes)
		resp.ReturnShipmentID = &id
	}
	if s.OriginalShipmentID.Valid {
		id := uuid.UUID(s.OriginalShipmentID.Bytes)
		resp.OriginalShipmentID = &id
	}

	// Handle optional timestamp fields
	if s.PickupScheduledAt.Valid {
		t := s.PickupScheduledAt.Time.Format(time.RFC3339)
		resp.PickupScheduledAt = &t
	}
	if s.PickedUpAt.Valid {
		t := s.PickedUpAt.Time.Format(time.RFC3339)
		resp.PickedUpAt = &t
	}
	if s.DeliveredAt.Valid {
		t := s.DeliveredAt.Time.Format(time.RFC3339)
		resp.DeliveredAt = &t
	}
	if s.CancelledAt.Valid {
		t := s.CancelledAt.Time.Format(time.RFC3339)
		resp.CancelledAt = &t
	}

	return resp
}

// Error handling

func (h *ShipmentHandler) handleShipmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, shipment.ErrShipmentNotFound):
		WriteError(w, r, NewNotFoundError("Shipment"))
	case errors.Is(err, shipment.ErrOfferNotFound):
		WriteError(w, r, NewNotFoundError("Offer"))
	case errors.Is(err, shipment.ErrOfferExpired):
		WriteError(w, r, &ProblemDetails{
			Type:   ErrorTypeQuoteExpired,
			Title:  "Offer Expired",
			Status: http.StatusGone,
			Detail: "The offer has expired. Please request a new quote.",
			Code:   CodeQuoteExpired,
		})
	case errors.Is(err, shipment.ErrLabelAlreadyPurchased):
		WriteError(w, r, NewConflictError("Label has already been purchased for this shipment"))
	case errors.Is(err, shipment.ErrInsufficientBalance):
		WriteError(w, r, &ProblemDetails{
			Type:   ErrorTypeInsufficientBalance,
			Title:  "Insufficient Balance",
			Status: http.StatusPaymentRequired,
			Detail: "Your account balance is insufficient for this operation",
			Code:   CodeInsufficientBalance,
		})
	case errors.Is(err, shipment.ErrInvalidShipmentStatus):
		WriteError(w, r, NewConflictError("Shipment cannot be modified in its current status"))
	case errors.Is(err, shipment.ErrProviderError):
		WriteError(w, r, &ProblemDetails{
			Type:   ErrorTypeProviderError,
			Title:  "Provider Error",
			Status: http.StatusBadGateway,
			Detail: "An error occurred while communicating with the shipping provider",
			Code:   CodeProviderError,
		})
	default:
		h.logger.Error("Shipment operation failed", "error", err)
		WriteError(w, r, NewInternalError("Failed to process shipment operation"))
	}
}

// Helper to get actor info from context
func getActorFromContext(ctx context.Context) (*uuid.UUID, domain.ActorType) {
	authInfo := middleware.GetAuthInfo(ctx)
	if authInfo == nil {
		return nil, domain.ActorTypeSystem
	}

	// Check for member auth
	if authInfo.MemberID != uuid.Nil {
		return &authInfo.MemberID, domain.ActorTypeMember
	}

	// Check for API key auth
	if authInfo.APIKeyID != uuid.Nil {
		return &authInfo.APIKeyID, domain.ActorTypeAPIKey
	}

	// Default to system
	return nil, domain.ActorTypeSystem
}
