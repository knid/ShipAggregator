// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// ProviderHandler handles provider-related HTTP requests.
type ProviderHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewProviderHandler creates a new provider handler.
func NewProviderHandler(queries *db.Queries, logger *observability.Logger) *ProviderHandler {
	return &ProviderHandler{
		queries: queries,
		logger:  logger,
	}
}

// ListProviders handles GET /providers
// Returns all active providers.
func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for active filter
	var isActive *bool
	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		isActive = &active
	} else {
		// Default to active only
		active := true
		isActive = &active
	}

	providers, err := h.queries.ListProviders(ctx, isActive)
	if err != nil {
		h.logger.Error("Failed to list providers", "error", err)
		WriteError(w, r, NewInternalError("Failed to list providers"))
		return
	}

	response := make([]ProviderResponse, len(providers))
	for i, p := range providers {
		response[i] = providerToResponse(p)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetProvider handles GET /providers/{code}
// Returns a specific provider by code.
func (h *ProviderHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	provider, err := h.queries.GetProviderByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Provider"))
			return
		}
		h.logger.Error("Failed to get provider", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get provider"))
		return
	}

	writeJSON(w, http.StatusOK, providerToResponse(provider))
}

// GetProviderWithServices handles GET /providers/{code}/details
// Returns a provider with all its services.
func (h *ProviderHandler) GetProviderWithServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	rows, err := h.queries.GetProviderWithServices(ctx, code)
	if err != nil {
		h.logger.Error("Failed to get provider with services", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get provider"))
		return
	}

	if len(rows) == 0 {
		WriteError(w, r, NewNotFoundError("Provider"))
		return
	}

	// Build response from rows
	firstRow := rows[0]
	response := ProviderWithServicesResponse{
		Code:         firstRow.Code,
		Name:         firstRow.Name,
		LogoURL:      firstRow.LogoUrl,
		Website:      firstRow.Website,
		SupportEmail: firstRow.SupportEmail,
		SupportPhone: firstRow.SupportPhone,
		APIType:      firstRow.ApiType,
		IsActive:     firstRow.ProviderIsActive,
		Capabilities: firstRow.Capabilities,
		Services:     make([]ProviderServiceResponse, 0),
	}

	// Add services
	for _, row := range rows {
		if row.ServiceCode != nil {
			response.Services = append(response.Services, ProviderServiceResponse{
				ID:                row.ServiceID.Bytes,
				ProviderCode:      row.Code,
				ServiceCode:       *row.ServiceCode,
				Name:              derefString(row.ServiceName),
				Description:       row.ServiceDescription,
				EstimatedDaysMin:  derefInt32(row.EstimatedDaysMin),
				EstimatedDaysMax:  derefInt32(row.EstimatedDaysMax),
				SupportsCOD:       derefBool(row.SupportsCod),
				SupportsInsurance: derefBool(row.SupportsInsurance),
				SupportsReturn:    derefBool(row.SupportsReturn),
				MaxWeightGrams:    derefInt32(row.MaxWeightGrams),
				MaxLengthCm:       derefInt32(row.MaxLengthCm),
				MaxWidthCm:        derefInt32(row.MaxWidthCm),
				MaxHeightCm:       derefInt32(row.MaxHeightCm),
				IsActive:          derefBool(row.ServiceIsActive),
			})
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// ListProviderServices handles GET /providers/{code}/services
// Returns all services for a specific provider.
func (h *ProviderHandler) ListProviderServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	// Check for active filter
	var isActive *bool
	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		isActive = &active
	}

	services, err := h.queries.ListProviderServices(ctx, db.ListProviderServicesParams{
		ProviderCode: code,
		IsActive:     isActive,
	})
	if err != nil {
		h.logger.Error("Failed to list provider services", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to list provider services"))
		return
	}

	response := make([]ProviderServiceResponse, len(services))
	for i, s := range services {
		response[i] = providerServiceToResponse(s)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetProviderService handles GET /providers/{code}/services/{serviceCode}
// Returns a specific service for a provider.
func (h *ProviderHandler) GetProviderService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	serviceCode := chi.URLParam(r, "serviceCode")
	if serviceCode == "" {
		WriteError(w, r, NewBadRequestError("Service code is required"))
		return
	}

	service, err := h.queries.GetProviderService(ctx, db.GetProviderServiceParams{
		ProviderCode: code,
		ServiceCode:  serviceCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Provider service"))
			return
		}
		h.logger.Error("Failed to get provider service", "error", err, "code", code, "service_code", serviceCode)
		WriteError(w, r, NewInternalError("Failed to get provider service"))
		return
	}

	writeJSON(w, http.StatusOK, providerServiceToResponse(service))
}

// ListProviderServicesWithCOD handles GET /providers/{code}/services/cod
// Returns services that support COD for a provider.
func (h *ProviderHandler) ListProviderServicesWithCOD(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	services, err := h.queries.ListProviderServicesWithCOD(ctx, code)
	if err != nil {
		h.logger.Error("Failed to list COD services", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to list COD services"))
		return
	}

	response := make([]ProviderServiceResponse, len(services))
	for i, s := range services {
		response[i] = providerServiceToResponse(s)
	}

	writeJSON(w, http.StatusOK, response)
}

// ListProviderServicesWithReturn handles GET /providers/{code}/services/return
// Returns services that support returns for a provider.
func (h *ProviderHandler) ListProviderServicesWithReturn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	services, err := h.queries.ListProviderServicesWithReturn(ctx, code)
	if err != nil {
		h.logger.Error("Failed to list return services", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to list return services"))
		return
	}

	response := make([]ProviderServiceResponse, len(services))
	for i, s := range services {
		response[i] = providerServiceToResponse(s)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetProviderCapabilities handles GET /providers/{code}/capabilities
// Returns the capabilities of a provider.
func (h *ProviderHandler) GetProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	capabilities, err := h.queries.GetProviderCapabilities(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Provider"))
			return
		}
		h.logger.Error("Failed to get provider capabilities", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get provider capabilities"))
		return
	}

	// Return raw JSON capabilities
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(capabilities)
}

// ListMarketplaceRatePlans handles GET /providers/{code}/rate-plans
// Returns marketplace rate plans for a provider.
func (h *ProviderHandler) ListMarketplaceRatePlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	ratePlans, err := h.queries.ListMarketplaceRatePlans(ctx, code)
	if err != nil {
		h.logger.Error("Failed to list rate plans", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to list rate plans"))
		return
	}

	response := make([]MarketplaceRatePlanResponse, len(ratePlans))
	for i, rp := range ratePlans {
		response[i] = ratePlanToResponse(rp)
	}

	writeJSON(w, http.StatusOK, response)
}

// ListCurrentMarketplaceRatePlans handles GET /rate-plans
// Returns all current marketplace rate plans.
func (h *ProviderHandler) ListCurrentMarketplaceRatePlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ratePlans, err := h.queries.ListCurrentMarketplaceRatePlans(ctx)
	if err != nil {
		h.logger.Error("Failed to list current rate plans", "error", err)
		WriteError(w, r, NewInternalError("Failed to list rate plans"))
		return
	}

	response := make([]MarketplaceRatePlanResponse, len(ratePlans))
	for i, rp := range ratePlans {
		response[i] = ratePlanToResponse(rp)
	}

	writeJSON(w, http.StatusOK, response)
}

// Response types

// ProviderResponse represents a provider in API responses.
type ProviderResponse struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	LogoURL      *string         `json:"logo_url,omitempty"`
	Website      *string         `json:"website,omitempty"`
	SupportEmail *string         `json:"support_email,omitempty"`
	SupportPhone *string         `json:"support_phone,omitempty"`
	APIType      string          `json:"api_type"`
	IsActive     bool            `json:"is_active"`
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
}

// ProviderWithServicesResponse represents a provider with its services.
type ProviderWithServicesResponse struct {
	Code         string                    `json:"code"`
	Name         string                    `json:"name"`
	LogoURL      *string                   `json:"logo_url,omitempty"`
	Website      *string                   `json:"website,omitempty"`
	SupportEmail *string                   `json:"support_email,omitempty"`
	SupportPhone *string                   `json:"support_phone,omitempty"`
	APIType      string                    `json:"api_type"`
	IsActive     bool                      `json:"is_active"`
	Capabilities json.RawMessage           `json:"capabilities,omitempty"`
	Services     []ProviderServiceResponse `json:"services"`
}

// ProviderServiceResponse represents a provider service in API responses.
type ProviderServiceResponse struct {
	ID                uuid.UUID `json:"id"`
	ProviderCode      string    `json:"provider_code"`
	ServiceCode       string    `json:"service_code"`
	Name              string    `json:"name"`
	Description       *string   `json:"description,omitempty"`
	EstimatedDaysMin  int32     `json:"estimated_days_min"`
	EstimatedDaysMax  int32     `json:"estimated_days_max"`
	SupportsCOD       bool      `json:"supports_cod"`
	SupportsInsurance bool      `json:"supports_insurance"`
	SupportsReturn    bool      `json:"supports_return"`
	MaxWeightGrams    int32     `json:"max_weight_grams"`
	MaxLengthCm       int32     `json:"max_length_cm"`
	MaxWidthCm        int32     `json:"max_width_cm"`
	MaxHeightCm       int32     `json:"max_height_cm"`
	IsActive          bool      `json:"is_active"`
}

// MarketplaceRatePlanResponse represents a marketplace rate plan in API responses.
type MarketplaceRatePlanResponse struct {
	ID                   uuid.UUID `json:"id"`
	ProviderCode         string    `json:"provider_code"`
	ServiceCode          string    `json:"service_code"`
	Name                 string    `json:"name"`
	BasePriceKurus       int32     `json:"base_price_kurus"`
	PricePerKgKurus      int32     `json:"price_per_kg_kurus"`
	PricePerDesiKurus    int32     `json:"price_per_desi_kurus"`
	FuelSurchargePercent float64   `json:"fuel_surcharge_percent"`
	CODFeePercent        float64   `json:"cod_fee_percent"`
	CODFeeMinKurus       int32     `json:"cod_fee_min_kurus"`
	VATPercent           float64   `json:"vat_percent"`
	EffectiveFrom        string    `json:"effective_from"`
	EffectiveTo          *string   `json:"effective_to,omitempty"`
	IsActive             bool      `json:"is_active"`
}

// Helper functions

func providerToResponse(p *db.Provider) ProviderResponse {
	return ProviderResponse{
		Code:         p.Code,
		Name:         p.Name,
		LogoURL:      p.LogoUrl,
		Website:      p.Website,
		SupportEmail: p.SupportEmail,
		SupportPhone: p.SupportPhone,
		APIType:      p.ApiType,
		IsActive:     p.IsActive,
		Capabilities: p.Capabilities,
	}
}

func providerServiceToResponse(s *db.ProviderService) ProviderServiceResponse {
	return ProviderServiceResponse{
		ID:                s.ID,
		ProviderCode:      s.ProviderCode,
		ServiceCode:       s.ServiceCode,
		Name:              s.Name,
		Description:       s.Description,
		EstimatedDaysMin:  s.EstimatedDaysMin,
		EstimatedDaysMax:  s.EstimatedDaysMax,
		SupportsCOD:       s.SupportsCod,
		SupportsInsurance: s.SupportsInsurance,
		SupportsReturn:    s.SupportsReturn,
		MaxWeightGrams:    s.MaxWeightGrams,
		MaxLengthCm:       s.MaxLengthCm,
		MaxWidthCm:        s.MaxWidthCm,
		MaxHeightCm:       s.MaxHeightCm,
		IsActive:          s.IsActive,
	}
}

func ratePlanToResponse(rp *db.MarketplaceRatePlan) MarketplaceRatePlanResponse {
	fuelSurcharge := numericToFloat64(rp.FuelSurchargePercent)
	codFee := numericToFloat64(rp.CodFeePercent)
	vat := numericToFloat64(rp.VatPercent)

	resp := MarketplaceRatePlanResponse{
		ID:                   rp.ID,
		ProviderCode:         rp.ProviderCode,
		ServiceCode:          rp.ServiceCode,
		Name:                 rp.Name,
		BasePriceKurus:       rp.BasePriceKurus,
		PricePerKgKurus:      rp.PricePerKgKurus,
		PricePerDesiKurus:    rp.PricePerDesiKurus,
		FuelSurchargePercent: fuelSurcharge,
		CODFeePercent:        codFee,
		CODFeeMinKurus:       rp.CodFeeMinKurus,
		VATPercent:           vat,
		EffectiveFrom:        rp.EffectiveFrom.Time.Format("2006-01-02"),
		IsActive:             rp.IsActive,
	}

	if rp.EffectiveTo.Valid {
		t := rp.EffectiveTo.Time.Format("2006-01-02")
		resp.EffectiveTo = &t
	}

	return resp
}

func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
