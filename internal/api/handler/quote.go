// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	"github.com/knid/kargonet/internal/service/quote"
)

// QuoteHandler handles quote-related HTTP requests.
type QuoteHandler struct {
	quoteService *quote.Service
	queries      *db.Queries
	logger       *observability.Logger
}

// NewQuoteHandler creates a new quote handler.
func NewQuoteHandler(quoteService *quote.Service, queries *db.Queries, logger *observability.Logger) *QuoteHandler {
	return &QuoteHandler{
		quoteService: quoteService,
		queries:      queries,
		logger:       logger,
	}
}

// CreateQuoteRequest represents the request body for creating a quote.
type CreateQuoteRequest struct {
	IdempotencyKey     string               `json:"idempotency_key"`
	OriginAddress      *QuoteAddressRequest `json:"origin_address"`
	DestinationAddress *QuoteAddressRequest `json:"destination_address"`
	Parcel             *QuoteParcelRequest  `json:"parcel"`
	CODAmountKurus     *int32               `json:"cod_amount_kurus,omitempty"`
	DeclaredValueKurus *int32               `json:"declared_value_kurus,omitempty"`
	UseBYOA            bool                 `json:"use_byoa"`
	ProviderAccountIDs []uuid.UUID          `json:"provider_account_ids,omitempty"`
}

// QuoteAddressRequest represents an address in a quote request.
type QuoteAddressRequest struct {
	ContactName      string  `json:"contact_name"`
	CompanyName      *string `json:"company_name,omitempty"`
	Phone            string  `json:"phone"`
	Email            *string `json:"email,omitempty"`
	StreetLine1      string  `json:"street_line_1"`
	StreetLine2      *string `json:"street_line_2,omitempty"`
	NeighborhoodCode string  `json:"neighborhood_code"`
	DistrictCode     string  `json:"district_code"`
	CityCode         string  `json:"city_code"`
	PostalCode       *string `json:"postal_code,omitempty"`
}

// QuoteParcelRequest represents parcel details in a quote request.
type QuoteParcelRequest struct {
	WeightGrams int32  `json:"weight_grams"`
	LengthCm    int32  `json:"length_cm"`
	WidthCm     int32  `json:"width_cm"`
	HeightCm    int32  `json:"height_cm"`
	Description string `json:"description,omitempty"`
}

// CreateQuote handles POST /quotes
// Creates a new quote and generates offers.
func (h *QuoteHandler) CreateQuote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req CreateQuoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.IdempotencyKey == "" {
		fieldErrors = append(fieldErrors, NewFieldError("idempotency_key", "REQUIRED", "Idempotency key is required"))
	}
	if req.OriginAddress == nil {
		fieldErrors = append(fieldErrors, NewFieldError("origin_address", "REQUIRED", "Origin address is required"))
	} else {
		fieldErrors = append(fieldErrors, validateQuoteAddress(req.OriginAddress, "origin_address")...)
	}
	if req.DestinationAddress == nil {
		fieldErrors = append(fieldErrors, NewFieldError("destination_address", "REQUIRED", "Destination address is required"))
	} else {
		fieldErrors = append(fieldErrors, validateQuoteAddress(req.DestinationAddress, "destination_address")...)
	}
	if req.Parcel == nil {
		fieldErrors = append(fieldErrors, NewFieldError("parcel", "REQUIRED", "Parcel details are required"))
	} else {
		fieldErrors = append(fieldErrors, validateQuoteParcel(req.Parcel)...)
	}

	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	// Warn if origin and destination appear identical
	if req.OriginAddress != nil && req.DestinationAddress != nil &&
		req.OriginAddress.CityCode == req.DestinationAddress.CityCode &&
		req.OriginAddress.DistrictCode == req.DestinationAddress.DistrictCode &&
		req.OriginAddress.StreetLine1 == req.DestinationAddress.StreetLine1 {
		h.logger.Warn("Quote origin and destination appear identical",
			"city_code", req.OriginAddress.CityCode,
			"district_code", req.OriginAddress.DistrictCode,
		)
	}

	// Convert to domain types
	originAddress := &domain.Address{
		ContactName:      req.OriginAddress.ContactName,
		CompanyName:      req.OriginAddress.CompanyName,
		Phone:            req.OriginAddress.Phone,
		Email:            req.OriginAddress.Email,
		StreetLine1:      req.OriginAddress.StreetLine1,
		StreetLine2:      req.OriginAddress.StreetLine2,
		NeighborhoodCode: req.OriginAddress.NeighborhoodCode,
		DistrictCode:     req.OriginAddress.DistrictCode,
		CityCode:         req.OriginAddress.CityCode,
		PostalCode:       req.OriginAddress.PostalCode,
	}

	destAddress := &domain.Address{
		ContactName:      req.DestinationAddress.ContactName,
		CompanyName:      req.DestinationAddress.CompanyName,
		Phone:            req.DestinationAddress.Phone,
		Email:            req.DestinationAddress.Email,
		StreetLine1:      req.DestinationAddress.StreetLine1,
		StreetLine2:      req.DestinationAddress.StreetLine2,
		NeighborhoodCode: req.DestinationAddress.NeighborhoodCode,
		DistrictCode:     req.DestinationAddress.DistrictCode,
		CityCode:         req.DestinationAddress.CityCode,
		PostalCode:       req.DestinationAddress.PostalCode,
	}

	var description *string
	if req.Parcel.Description != "" {
		description = &req.Parcel.Description
	}

	parcel := &domain.Parcel{
		WeightGrams: int(req.Parcel.WeightGrams),
		LengthCm:    int(req.Parcel.LengthCm),
		WidthCm:     int(req.Parcel.WidthCm),
		HeightCm:    int(req.Parcel.HeightCm),
		Description: description,
	}

	// Create quote
	result, err := h.quoteService.CreateQuote(ctx, &quote.CreateQuoteRequest{
		OrganizationID:     orgID,
		IdempotencyKey:     req.IdempotencyKey,
		OriginAddress:      originAddress,
		DestinationAddress: destAddress,
		Parcel:             parcel,
		CODAmountKurus:     req.CODAmountKurus,
		DeclaredValueKurus: req.DeclaredValueKurus,
		UseBYOA:            req.UseBYOA,
		ProviderAccountIDs: req.ProviderAccountIDs,
	})
	if err != nil {
		h.logger.Error("Failed to create quote", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to create quote"))
		return
	}

	writeJSON(w, http.StatusCreated, quoteResultToResponse(result))
}

// GetQuote handles GET /quotes/{id}
// Returns a specific quote.
func (h *QuoteHandler) GetQuote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get quote ID from URL
	quoteIDStr := chi.URLParam(r, "id")
	quoteID, err := uuid.Parse(quoteIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid quote ID"))
		return
	}

	result, err := h.quoteService.GetQuoteWithOffers(ctx, quoteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Quote"))
			return
		}
		h.logger.Error("Failed to get quote", "error", err, "quote_id", quoteID)
		WriteError(w, r, NewInternalError("Failed to get quote"))
		return
	}

	// Verify organization ownership
	if result.Quote.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Quote"))
		return
	}

	writeJSON(w, http.StatusOK, quoteResultToResponse(result))
}

// ListQuotes handles GET /quotes
// Returns quotes for the current organization.
func (h *QuoteHandler) ListQuotes(w http.ResponseWriter, r *http.Request) {
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

	quotes, count, err := h.quoteService.ListQuotes(ctx, orgID, status, offset, limit)
	if err != nil {
		h.logger.Error("Failed to list quotes", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list quotes"))
		return
	}

	response := make([]QuoteResponse, len(quotes))
	for i, q := range quotes {
		response[i] = quoteToResponse(q)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  count,
		Offset: offset,
		Limit:  limit,
	})
}

// GetQuoteOffers handles GET /quotes/{id}/offers
// Returns offers for a specific quote.
func (h *QuoteHandler) GetQuoteOffers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get quote ID from URL
	quoteIDStr := chi.URLParam(r, "id")
	quoteID, err := uuid.Parse(quoteIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid quote ID"))
		return
	}

	// Verify quote exists and belongs to organization
	q, err := h.quoteService.GetQuote(ctx, quoteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Quote"))
			return
		}
		h.logger.Error("Failed to get quote", "error", err, "quote_id", quoteID)
		WriteError(w, r, NewInternalError("Failed to get quote"))
		return
	}
	if q.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Quote"))
		return
	}

	// Check for active_only filter
	activeOnly := r.URL.Query().Get("active_only") == "true"

	var offers []*db.Offer
	if activeOnly {
		offers, err = h.quoteService.GetActiveOffers(ctx, quoteID)
	} else {
		offers, err = h.queries.ListOffersByQuote(ctx, quoteID)
	}
	if err != nil {
		h.logger.Error("Failed to get offers", "error", err, "quote_id", quoteID)
		WriteError(w, r, NewInternalError("Failed to get offers"))
		return
	}

	response := make([]OfferResponse, len(offers))
	for i, o := range offers {
		response[i] = offerToResponse(o)
	}

	writeJSON(w, http.StatusOK, response)
}

// RefreshQuote handles POST /quotes/{id}/refresh
// Refreshes a quote by regenerating offers.
func (h *QuoteHandler) RefreshQuote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get quote ID from URL
	quoteIDStr := chi.URLParam(r, "id")
	quoteID, err := uuid.Parse(quoteIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid quote ID"))
		return
	}

	result, err := h.quoteService.RefreshQuote(ctx, quoteID, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || err.Error() == "quote not found" {
			WriteError(w, r, NewNotFoundError("Quote"))
			return
		}
		h.logger.Error("Failed to refresh quote", "error", err, "quote_id", quoteID)
		WriteError(w, r, NewInternalError("Failed to refresh quote"))
		return
	}

	writeJSON(w, http.StatusOK, quoteResultToResponse(result))
}

// GetOffer handles GET /offers/{id}
// Returns a specific offer.
func (h *QuoteHandler) GetOffer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get offer ID from URL
	offerIDStr := chi.URLParam(r, "id")
	offerID, err := uuid.Parse(offerIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid offer ID"))
		return
	}

	offer, err := h.quoteService.GetOffer(ctx, offerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Offer"))
			return
		}
		h.logger.Error("Failed to get offer", "error", err, "offer_id", offerID)
		WriteError(w, r, NewInternalError("Failed to get offer"))
		return
	}

	// Verify organization ownership via quote
	q, err := h.quoteService.GetQuote(ctx, offer.QuoteID)
	if err != nil {
		h.logger.Error("Failed to get quote for offer", "error", err, "offer_id", offerID)
		WriteError(w, r, NewInternalError("Failed to get offer"))
		return
	}
	if q.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Offer"))
		return
	}

	writeJSON(w, http.StatusOK, offerToResponse(offer))
}

// Response types

// QuoteResponse represents a quote in API responses.
type QuoteResponse struct {
	ID                 uuid.UUID       `json:"id"`
	OrganizationID     uuid.UUID       `json:"organization_id"`
	IdempotencyKey     string          `json:"idempotency_key"`
	OriginAddress      json.RawMessage `json:"origin_address"`
	DestinationAddress json.RawMessage `json:"destination_address"`
	Parcel             json.RawMessage `json:"parcel"`
	CODAmountKurus     *int32          `json:"cod_amount_kurus,omitempty"`
	DeclaredValueKurus *int32          `json:"declared_value_kurus,omitempty"`
	UseBYOA            bool            `json:"use_byoa"`
	Status             string          `json:"status"`
	ExpiresAt          string          `json:"expires_at"`
	CreatedAt          string          `json:"created_at"`
}

// QuoteWithOffersResponse represents a quote with its offers.
type QuoteWithOffersResponse struct {
	QuoteResponse
	Offers []OfferResponse `json:"offers"`
}

// OfferResponse represents an offer in API responses.
type OfferResponse struct {
	ID                 uuid.UUID  `json:"id"`
	QuoteID            uuid.UUID  `json:"quote_id"`
	ProviderCode       string     `json:"provider_code"`
	ServiceCode        string     `json:"service_code"`
	ProviderAccountID  *uuid.UUID `json:"provider_account_id,omitempty"`
	IsMarketplace      bool       `json:"is_marketplace"`
	BasePriceKurus     int32      `json:"base_price_kurus"`
	FuelSurchargeKurus int32      `json:"fuel_surcharge_kurus"`
	CODFeeKurus        int32      `json:"cod_fee_kurus"`
	InsuranceFeeKurus  int32      `json:"insurance_fee_kurus"`
	OtherFeesKurus     int32      `json:"other_fees_kurus"`
	SubtotalKurus      int32      `json:"subtotal_kurus"`
	VATKurus           int32      `json:"vat_kurus"`
	TotalPriceKurus    int32      `json:"total_price_kurus"`
	EstimatedDaysMin   int32      `json:"estimated_days_min"`
	EstimatedDaysMax   int32      `json:"estimated_days_max"`
	ExpiresAt          string     `json:"expires_at"`
	CreatedAt          string     `json:"created_at"`
}

// Validation helpers

func validateQuoteAddress(addr *QuoteAddressRequest, prefix string) []ErrorDetail {
	var errors []ErrorDetail
	if addr.ContactName == "" {
		errors = append(errors, NewFieldError(prefix+".contact_name", "REQUIRED", "Contact name is required"))
	}
	if addr.Phone == "" {
		errors = append(errors, NewFieldError(prefix+".phone", "REQUIRED", "Phone is required"))
	}
	if addr.StreetLine1 == "" {
		errors = append(errors, NewFieldError(prefix+".street_line_1", "REQUIRED", "Street line 1 is required"))
	}
	if addr.NeighborhoodCode == "" {
		errors = append(errors, NewFieldError(prefix+".neighborhood_code", "REQUIRED", "Neighborhood code is required"))
	}
	if addr.DistrictCode == "" {
		errors = append(errors, NewFieldError(prefix+".district_code", "REQUIRED", "District code is required"))
	}
	if addr.CityCode == "" {
		errors = append(errors, NewFieldError(prefix+".city_code", "REQUIRED", "City code is required"))
	}
	return errors
}

func validateQuoteParcel(parcel *QuoteParcelRequest) []ErrorDetail {
	var errors []ErrorDetail
	if parcel.WeightGrams <= 0 {
		errors = append(errors, NewFieldError("parcel.weight_grams", "INVALID", "Weight must be greater than 0"))
	}
	if parcel.LengthCm <= 0 {
		errors = append(errors, NewFieldError("parcel.length_cm", "INVALID", "Length must be greater than 0"))
	}
	if parcel.WidthCm <= 0 {
		errors = append(errors, NewFieldError("parcel.width_cm", "INVALID", "Width must be greater than 0"))
	}
	if parcel.HeightCm <= 0 {
		errors = append(errors, NewFieldError("parcel.height_cm", "INVALID", "Height must be greater than 0"))
	}
	return errors
}

// Conversion helpers

func quoteToResponse(q *db.Quote) QuoteResponse {
	return QuoteResponse{
		ID:                 q.ID,
		OrganizationID:     q.OrganizationID,
		IdempotencyKey:     q.IdempotencyKey,
		OriginAddress:      q.OriginAddress,
		DestinationAddress: q.DestinationAddress,
		Parcel:             q.Parcel,
		CODAmountKurus:     q.CodAmountKurus,
		DeclaredValueKurus: q.DeclaredValueKurus,
		UseBYOA:            q.UseByoa,
		Status:             q.Status,
		ExpiresAt:          q.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:          q.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func quoteResultToResponse(result *quote.QuoteResult) QuoteWithOffersResponse {
	offers := make([]OfferResponse, len(result.Offers))
	for i, o := range result.Offers {
		offers[i] = offerToResponse(o)
	}

	return QuoteWithOffersResponse{
		QuoteResponse: quoteToResponse(result.Quote),
		Offers:        offers,
	}
}

func offerToResponse(o *db.Offer) OfferResponse {
	resp := OfferResponse{
		ID:                 o.ID,
		QuoteID:            o.QuoteID,
		ProviderCode:       o.ProviderCode,
		ServiceCode:        o.ServiceCode,
		IsMarketplace:      o.IsMarketplace,
		BasePriceKurus:     o.BasePriceKurus,
		FuelSurchargeKurus: o.FuelSurchargeKurus,
		CODFeeKurus:        o.CodFeeKurus,
		InsuranceFeeKurus:  o.InsuranceFeeKurus,
		OtherFeesKurus:     o.OtherFeesKurus,
		SubtotalKurus:      o.SubtotalKurus,
		VATKurus:           o.VatKurus,
		TotalPriceKurus:    o.TotalPriceKurus,
		EstimatedDaysMin:   o.EstimatedDaysMin,
		EstimatedDaysMax:   o.EstimatedDaysMax,
		ExpiresAt:          o.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:          o.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}

	if o.ProviderAccountID.Valid {
		id := uuid.UUID(o.ProviderAccountID.Bytes)
		resp.ProviderAccountID = &id
	}

	return resp
}
