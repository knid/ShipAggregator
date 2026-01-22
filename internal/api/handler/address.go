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
	"github.com/shopspring/decimal"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// AddressHandler handles address-related HTTP requests.
type AddressHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewAddressHandler creates a new address handler.
func NewAddressHandler(queries *db.Queries, logger *observability.Logger) *AddressHandler {
	return &AddressHandler{
		queries: queries,
		logger:  logger,
	}
}

// ListAddresses handles GET /addresses
// Returns addresses for the current organization.
func (h *AddressHandler) ListAddresses(w http.ResponseWriter, r *http.Request) {
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

	// Check for search query
	search := r.URL.Query().Get("q")
	if search != "" {
		h.searchAddresses(w, r, orgID, search, limit)
		return
	}

	// Get count
	count, err := h.queries.CountAddressesByOrganization(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to count addresses", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to count addresses"))
		return
	}

	// Get addresses
	addresses, err := h.queries.ListAddressesByOrganization(ctx, db.ListAddressesByOrganizationParams{
		OrganizationID: orgID,
		OffsetCount:    int32(offset),
		LimitCount:     int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to list addresses", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list addresses"))
		return
	}

	response := make([]AddressResponse, len(addresses))
	for i, a := range addresses {
		response[i] = addressToResponse(a)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  count,
		Offset: offset,
		Limit:  limit,
	})
}

// searchAddresses searches addresses by label, contact name, company name, or street.
func (h *AddressHandler) searchAddresses(w http.ResponseWriter, r *http.Request, orgID uuid.UUID, search string, limit int) {
	ctx := r.Context()

	addresses, err := h.queries.SearchAddressesByOrganization(ctx, db.SearchAddressesByOrganizationParams{
		OrganizationID: orgID,
		Column2:        &search,
		LimitCount:     int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to search addresses", "error", err, "org_id", orgID, "search", search)
		WriteError(w, r, NewInternalError("Failed to search addresses"))
		return
	}

	response := make([]AddressResponse, len(addresses))
	for i, a := range addresses {
		response[i] = addressToResponse(a)
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateAddressRequest represents the request body for creating an address.
type CreateAddressRequest struct {
	Label              *string `json:"label,omitempty"`
	ContactName        string  `json:"contact_name"`
	CompanyName        *string `json:"company_name,omitempty"`
	Phone              string  `json:"phone"`
	Email              *string `json:"email,omitempty"`
	StreetLine1        string  `json:"street_line_1"`
	StreetLine2        *string `json:"street_line_2,omitempty"`
	NeighborhoodCode   string  `json:"neighborhood_code"`
	DistrictCode       string  `json:"district_code"`
	CityCode           string  `json:"city_code"`
	PostalCode         *string `json:"postal_code,omitempty"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	IsDefaultSender    bool    `json:"is_default_sender"`
	IsDefaultRecipient bool    `json:"is_default_recipient"`
}

// CreateAddress handles POST /addresses
// Creates a new address for the current organization.
func (h *AddressHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req CreateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.ContactName == "" {
		fieldErrors = append(fieldErrors, NewFieldError("contact_name", "REQUIRED", "Contact name is required"))
	}
	if req.Phone == "" {
		fieldErrors = append(fieldErrors, NewFieldError("phone", "REQUIRED", "Phone is required"))
	}
	if req.StreetLine1 == "" {
		fieldErrors = append(fieldErrors, NewFieldError("street_line_1", "REQUIRED", "Street line 1 is required"))
	}
	if req.NeighborhoodCode == "" {
		fieldErrors = append(fieldErrors, NewFieldError("neighborhood_code", "REQUIRED", "Neighborhood code is required"))
	}
	if req.DistrictCode == "" {
		fieldErrors = append(fieldErrors, NewFieldError("district_code", "REQUIRED", "District code is required"))
	}
	if req.CityCode == "" {
		fieldErrors = append(fieldErrors, NewFieldError("city_code", "REQUIRED", "City code is required"))
	}

	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	// Clear existing defaults if setting new defaults
	if req.IsDefaultSender {
		if err := h.queries.ClearDefaultSenderByOrganization(ctx, orgID); err != nil {
			h.logger.Error("Failed to clear default sender", "error", err, "org_id", orgID)
			WriteError(w, r, NewInternalError("Failed to create address"))
			return
		}
	}
	if req.IsDefaultRecipient {
		if err := h.queries.ClearDefaultRecipientByOrganization(ctx, orgID); err != nil {
			h.logger.Error("Failed to clear default recipient", "error", err, "org_id", orgID)
			WriteError(w, r, NewInternalError("Failed to create address"))
			return
		}
	}

	// Create address
	address, err := h.queries.CreateAddress(ctx, db.CreateAddressParams{
		OrganizationID:     orgID,
		Label:              req.Label,
		ContactName:        req.ContactName,
		CompanyName:        req.CompanyName,
		Phone:              req.Phone,
		Email:              req.Email,
		StreetLine1:        req.StreetLine1,
		StreetLine2:        req.StreetLine2,
		NeighborhoodCode:   req.NeighborhoodCode,
		DistrictCode:       req.DistrictCode,
		CityCode:           req.CityCode,
		PostalCode:         req.PostalCode,
		Latitude:           decimal.NewFromFloat(req.Latitude),
		Longitude:          decimal.NewFromFloat(req.Longitude),
		IsDefaultSender:    req.IsDefaultSender,
		IsDefaultRecipient: req.IsDefaultRecipient,
	})
	if err != nil {
		h.logger.Error("Failed to create address", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to create address"))
		return
	}

	writeJSON(w, http.StatusCreated, addressToResponse(address))
}

// GetAddress handles GET /addresses/{id}
// Returns a specific address.
func (h *AddressHandler) GetAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get address ID from URL
	addressIDStr := chi.URLParam(r, "id")
	addressID, err := uuid.Parse(addressIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid address ID"))
		return
	}

	address, err := h.queries.GetAddressByID(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to get address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to get address"))
		return
	}

	// Verify address belongs to the organization
	if address.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Address"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// UpdateAddressRequest represents the request body for updating an address.
type UpdateAddressRequest struct {
	Label              *string  `json:"label,omitempty"`
	ContactName        *string  `json:"contact_name,omitempty"`
	CompanyName        *string  `json:"company_name,omitempty"`
	Phone              *string  `json:"phone,omitempty"`
	Email              *string  `json:"email,omitempty"`
	StreetLine1        *string  `json:"street_line_1,omitempty"`
	StreetLine2        *string  `json:"street_line_2,omitempty"`
	NeighborhoodCode   *string  `json:"neighborhood_code,omitempty"`
	DistrictCode       *string  `json:"district_code,omitempty"`
	CityCode           *string  `json:"city_code,omitempty"`
	PostalCode         *string  `json:"postal_code,omitempty"`
	Latitude           *float64 `json:"latitude,omitempty"`
	Longitude          *float64 `json:"longitude,omitempty"`
	IsDefaultSender    *bool    `json:"is_default_sender,omitempty"`
	IsDefaultRecipient *bool    `json:"is_default_recipient,omitempty"`
}

// UpdateAddress handles PATCH /addresses/{id}
// Updates a specific address.
func (h *AddressHandler) UpdateAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get address ID from URL
	addressIDStr := chi.URLParam(r, "id")
	addressID, err := uuid.Parse(addressIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid address ID"))
		return
	}

	// Verify address exists and belongs to the organization
	existingAddress, err := h.queries.GetAddressByID(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to get address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to get address"))
		return
	}
	if existingAddress.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Address"))
		return
	}

	var req UpdateAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Clear existing defaults if setting new defaults
	if req.IsDefaultSender != nil && *req.IsDefaultSender {
		if err := h.queries.ClearDefaultSenderByOrganization(ctx, orgID); err != nil {
			h.logger.Error("Failed to clear default sender", "error", err, "org_id", orgID)
			WriteError(w, r, NewInternalError("Failed to update address"))
			return
		}
	}
	if req.IsDefaultRecipient != nil && *req.IsDefaultRecipient {
		if err := h.queries.ClearDefaultRecipientByOrganization(ctx, orgID); err != nil {
			h.logger.Error("Failed to clear default recipient", "error", err, "org_id", orgID)
			WriteError(w, r, NewInternalError("Failed to update address"))
			return
		}
	}

	// Prepare update params
	params := db.UpdateAddressParams{
		ID:                 addressID,
		Label:              req.Label,
		ContactName:        req.ContactName,
		CompanyName:        req.CompanyName,
		Phone:              req.Phone,
		Email:              req.Email,
		StreetLine1:        req.StreetLine1,
		StreetLine2:        req.StreetLine2,
		NeighborhoodCode:   req.NeighborhoodCode,
		DistrictCode:       req.DistrictCode,
		CityCode:           req.CityCode,
		PostalCode:         req.PostalCode,
		IsDefaultSender:    req.IsDefaultSender,
		IsDefaultRecipient: req.IsDefaultRecipient,
	}

	// Handle latitude/longitude
	if req.Latitude != nil {
		params.Latitude = decimal.NewFromFloat(*req.Latitude)
	}
	if req.Longitude != nil {
		params.Longitude = decimal.NewFromFloat(*req.Longitude)
	}

	// Update address
	address, err := h.queries.UpdateAddress(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to update address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to update address"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// DeleteAddress handles DELETE /addresses/{id}
// Soft deletes a specific address.
func (h *AddressHandler) DeleteAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get address ID from URL
	addressIDStr := chi.URLParam(r, "id")
	addressID, err := uuid.Parse(addressIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid address ID"))
		return
	}

	// Verify address exists and belongs to the organization
	existingAddress, err := h.queries.GetAddressByID(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to get address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to get address"))
		return
	}
	if existingAddress.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Address"))
		return
	}

	// Soft delete address
	_, err = h.queries.SoftDeleteAddress(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to delete address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to delete address"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultSender handles POST /addresses/{id}/default-sender
// Sets an address as the default sender.
func (h *AddressHandler) SetDefaultSender(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get address ID from URL
	addressIDStr := chi.URLParam(r, "id")
	addressID, err := uuid.Parse(addressIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid address ID"))
		return
	}

	// Verify address exists and belongs to the organization
	existingAddress, err := h.queries.GetAddressByID(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to get address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to get address"))
		return
	}
	if existingAddress.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Address"))
		return
	}

	// Clear existing default sender
	if err := h.queries.ClearDefaultSenderByOrganization(ctx, orgID); err != nil {
		h.logger.Error("Failed to clear default sender", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to set default sender"))
		return
	}

	// Set new default sender
	address, err := h.queries.SetDefaultSender(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to set default sender", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to set default sender"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// SetDefaultRecipient handles POST /addresses/{id}/default-recipient
// Sets an address as the default recipient.
func (h *AddressHandler) SetDefaultRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get address ID from URL
	addressIDStr := chi.URLParam(r, "id")
	addressID, err := uuid.Parse(addressIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid address ID"))
		return
	}

	// Verify address exists and belongs to the organization
	existingAddress, err := h.queries.GetAddressByID(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to get address", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to get address"))
		return
	}
	if existingAddress.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Address"))
		return
	}

	// Clear existing default recipient
	if err := h.queries.ClearDefaultRecipientByOrganization(ctx, orgID); err != nil {
		h.logger.Error("Failed to clear default recipient", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to set default recipient"))
		return
	}

	// Set new default recipient
	address, err := h.queries.SetDefaultRecipient(ctx, addressID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Address"))
			return
		}
		h.logger.Error("Failed to set default recipient", "error", err, "address_id", addressID)
		WriteError(w, r, NewInternalError("Failed to set default recipient"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// GetDefaultSender handles GET /addresses/default-sender
// Returns the default sender address.
func (h *AddressHandler) GetDefaultSender(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	address, err := h.queries.GetDefaultSenderAddress(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Default sender address"))
			return
		}
		h.logger.Error("Failed to get default sender", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get default sender"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// GetDefaultRecipient handles GET /addresses/default-recipient
// Returns the default recipient address.
func (h *AddressHandler) GetDefaultRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	address, err := h.queries.GetDefaultRecipientAddress(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Default recipient address"))
			return
		}
		h.logger.Error("Failed to get default recipient", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get default recipient"))
		return
	}

	writeJSON(w, http.StatusOK, addressToResponse(address))
}

// Response types

// AddressResponse represents an address in API responses.
type AddressResponse struct {
	ID                 uuid.UUID `json:"id"`
	OrganizationID     uuid.UUID `json:"organization_id"`
	Label              *string   `json:"label,omitempty"`
	ContactName        string    `json:"contact_name"`
	CompanyName        *string   `json:"company_name,omitempty"`
	Phone              string    `json:"phone"`
	Email              *string   `json:"email,omitempty"`
	StreetLine1        string    `json:"street_line_1"`
	StreetLine2        *string   `json:"street_line_2,omitempty"`
	NeighborhoodCode   string    `json:"neighborhood_code"`
	DistrictCode       string    `json:"district_code"`
	CityCode           string    `json:"city_code"`
	PostalCode         *string   `json:"postal_code,omitempty"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	IsDefaultSender    bool      `json:"is_default_sender"`
	IsDefaultRecipient bool      `json:"is_default_recipient"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

// Helper functions

func addressToResponse(a *db.Address) AddressResponse {
	lat, _ := a.Latitude.Float64()
	lng, _ := a.Longitude.Float64()

	return AddressResponse{
		ID:                 a.ID,
		OrganizationID:     a.OrganizationID,
		Label:              a.Label,
		ContactName:        a.ContactName,
		CompanyName:        a.CompanyName,
		Phone:              a.Phone,
		Email:              a.Email,
		StreetLine1:        a.StreetLine1,
		StreetLine2:        a.StreetLine2,
		NeighborhoodCode:   a.NeighborhoodCode,
		DistrictCode:       a.DistrictCode,
		CityCode:           a.CityCode,
		PostalCode:         a.PostalCode,
		Latitude:           lat,
		Longitude:          lng,
		IsDefaultSender:    a.IsDefaultSender,
		IsDefaultRecipient: a.IsDefaultRecipient,
		CreatedAt:          a.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          a.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}
