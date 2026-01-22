// Package handler provides HTTP handlers for the API.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// ReferenceHandler handles reference data HTTP requests.
type ReferenceHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewReferenceHandler creates a new reference data handler.
func NewReferenceHandler(queries *db.Queries, logger *observability.Logger) *ReferenceHandler {
	return &ReferenceHandler{
		queries: queries,
		logger:  logger,
	}
}

// ListCities handles GET /ref/cities
// Returns all active cities (provinces) in Turkey.
func (h *ReferenceHandler) ListCities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for search query
	search := r.URL.Query().Get("q")
	if search != "" {
		h.searchCities(w, r, search)
		return
	}

	cities, err := h.queries.ListCities(ctx)
	if err != nil {
		h.logger.Error("Failed to list cities", "error", err)
		WriteError(w, r, NewInternalError("Failed to list cities"))
		return
	}

	response := make([]CityResponse, len(cities))
	for i, c := range cities {
		response[i] = cityToResponse(c)
	}

	writeJSON(w, http.StatusOK, response)
}

// searchCities searches cities by name.
func (h *ReferenceHandler) searchCities(w http.ResponseWriter, r *http.Request, search string) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	cities, err := h.queries.SearchCitiesByName(ctx, db.SearchCitiesByNameParams{
		Column1:    &search,
		LimitCount: int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to search cities", "error", err, "search", search)
		WriteError(w, r, NewInternalError("Failed to search cities"))
		return
	}

	response := make([]CityResponse, len(cities))
	for i, c := range cities {
		response[i] = cityToResponse(c)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetCity handles GET /ref/cities/{code}
// Returns a specific city by code.
func (h *ReferenceHandler) GetCity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("City code is required"))
		return
	}

	city, err := h.queries.GetCityByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("City"))
			return
		}
		h.logger.Error("Failed to get city", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get city"))
		return
	}

	writeJSON(w, http.StatusOK, cityToResponse(city))
}

// ListDistricts handles GET /ref/districts
// Returns districts, optionally filtered by city code.
func (h *ReferenceHandler) ListDistricts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cityCode := r.URL.Query().Get("city_code")
	if cityCode == "" {
		WriteError(w, r, NewBadRequestError("city_code query parameter is required"))
		return
	}

	// Check for search query
	search := r.URL.Query().Get("q")
	if search != "" {
		h.searchDistricts(w, r, search)
		return
	}

	districts, err := h.queries.ListDistrictsByCity(ctx, cityCode)
	if err != nil {
		h.logger.Error("Failed to list districts", "error", err, "city_code", cityCode)
		WriteError(w, r, NewInternalError("Failed to list districts"))
		return
	}

	response := make([]DistrictResponse, len(districts))
	for i, d := range districts {
		response[i] = districtToResponse(d)
	}

	writeJSON(w, http.StatusOK, response)
}

// searchDistricts searches districts by name.
func (h *ReferenceHandler) searchDistricts(w http.ResponseWriter, r *http.Request, search string) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	districts, err := h.queries.SearchDistrictsByName(ctx, db.SearchDistrictsByNameParams{
		Column1:    &search,
		LimitCount: int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to search districts", "error", err, "search", search)
		WriteError(w, r, NewInternalError("Failed to search districts"))
		return
	}

	response := make([]DistrictWithCityResponse, len(districts))
	for i, d := range districts {
		response[i] = DistrictWithCityResponse{
			Code:     d.Code,
			CityCode: d.CityCode,
			CityName: d.CityName,
			Name:     d.Name,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// GetDistrict handles GET /ref/districts/{code}
// Returns a specific district by code.
func (h *ReferenceHandler) GetDistrict(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("District code is required"))
		return
	}

	district, err := h.queries.GetDistrictWithCity(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("District"))
			return
		}
		h.logger.Error("Failed to get district", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get district"))
		return
	}

	writeJSON(w, http.StatusOK, DistrictWithCityResponse{
		Code:     district.Code,
		CityCode: district.CityCode,
		CityName: district.CityName,
		Name:     district.Name,
	})
}

// ListNeighborhoods handles GET /ref/neighborhoods
// Returns neighborhoods, filtered by district code.
func (h *ReferenceHandler) ListNeighborhoods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	districtCode := r.URL.Query().Get("district_code")
	if districtCode == "" {
		WriteError(w, r, NewBadRequestError("district_code query parameter is required"))
		return
	}

	// Check for search query
	search := r.URL.Query().Get("q")
	if search != "" {
		h.searchNeighborhoods(w, r, search)
		return
	}

	neighborhoods, err := h.queries.ListNeighborhoodsByDistrict(ctx, districtCode)
	if err != nil {
		h.logger.Error("Failed to list neighborhoods", "error", err, "district_code", districtCode)
		WriteError(w, r, NewInternalError("Failed to list neighborhoods"))
		return
	}

	response := make([]NeighborhoodResponse, len(neighborhoods))
	for i, n := range neighborhoods {
		response[i] = neighborhoodToResponse(n)
	}

	writeJSON(w, http.StatusOK, response)
}

// searchNeighborhoods searches neighborhoods by name.
func (h *ReferenceHandler) searchNeighborhoods(w http.ResponseWriter, r *http.Request, search string) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	neighborhoods, err := h.queries.SearchNeighborhoodsByName(ctx, db.SearchNeighborhoodsByNameParams{
		Column1:    &search,
		LimitCount: int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to search neighborhoods", "error", err, "search", search)
		WriteError(w, r, NewInternalError("Failed to search neighborhoods"))
		return
	}

	response := make([]NeighborhoodWithHierarchyResponse, len(neighborhoods))
	for i, n := range neighborhoods {
		response[i] = NeighborhoodWithHierarchyResponse{
			Code:         n.Code,
			DistrictCode: n.DistrictCode,
			DistrictName: n.DistrictName,
			CityName:     n.CityName,
			Name:         n.Name,
			PostalCode:   n.PostalCode,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// GetNeighborhood handles GET /ref/neighborhoods/{code}
// Returns a specific neighborhood by code.
func (h *ReferenceHandler) GetNeighborhood(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Neighborhood code is required"))
		return
	}

	neighborhood, err := h.queries.GetNeighborhoodWithHierarchy(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Neighborhood"))
			return
		}
		h.logger.Error("Failed to get neighborhood", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get neighborhood"))
		return
	}

	writeJSON(w, http.StatusOK, NeighborhoodWithHierarchyResponse{
		Code:         neighborhood.Code,
		DistrictCode: neighborhood.DistrictCode,
		DistrictName: neighborhood.DistrictName,
		CityCode:     neighborhood.CityCode,
		CityName:     neighborhood.CityName,
		Name:         neighborhood.Name,
		PostalCode:   neighborhood.PostalCode,
	})
}

// ListPostalCodes handles GET /ref/postal-codes
// Returns postal codes, filtered by district code.
func (h *ReferenceHandler) ListPostalCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	districtCode := r.URL.Query().Get("district_code")
	if districtCode == "" {
		WriteError(w, r, NewBadRequestError("district_code query parameter is required"))
		return
	}

	postalCodes, err := h.queries.ListPostalCodesByDistrict(ctx, districtCode)
	if err != nil {
		h.logger.Error("Failed to list postal codes", "error", err, "district_code", districtCode)
		WriteError(w, r, NewInternalError("Failed to list postal codes"))
		return
	}

	response := make([]PostalCodeResponse, len(postalCodes))
	for i, p := range postalCodes {
		response[i] = postalCodeToResponse(p)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetPostalCode handles GET /ref/postal-codes/{code}
// Returns a specific postal code by code.
func (h *ReferenceHandler) GetPostalCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Postal code is required"))
		return
	}

	postalCode, err := h.queries.GetPostalCodeByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Postal code"))
			return
		}
		h.logger.Error("Failed to get postal code", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to get postal code"))
		return
	}

	writeJSON(w, http.StatusOK, postalCodeToResponse(postalCode))
}

// LookupByPostalCode handles GET /ref/lookup/postal-code/{code}
// Returns full location hierarchy for a postal code.
func (h *ReferenceHandler) LookupByPostalCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := chi.URLParam(r, "code")
	if code == "" {
		WriteError(w, r, NewBadRequestError("Postal code is required"))
		return
	}

	location, err := h.queries.GetLocationByPostalCode(ctx, &code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Location"))
			return
		}
		h.logger.Error("Failed to lookup by postal code", "error", err, "code", code)
		WriteError(w, r, NewInternalError("Failed to lookup location"))
		return
	}

	writeJSON(w, http.StatusOK, LocationResponse{
		NeighborhoodCode: location.NeighborhoodCode,
		NeighborhoodName: location.NeighborhoodName,
		DistrictCode:     location.DistrictCode,
		DistrictName:     location.DistrictName,
		CityCode:         location.CityCode,
		CityName:         location.CityName,
		PostalCode:       location.PostalCode,
	})
}

// SearchLocations handles GET /ref/search
// Searches across all location types.
func (h *ReferenceHandler) SearchLocations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	search := r.URL.Query().Get("q")
	if search == "" {
		WriteError(w, r, NewBadRequestError("q query parameter is required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	locations, err := h.queries.SearchLocations(ctx, db.SearchLocationsParams{
		Column1:    &search,
		LimitCount: int32(limit),
	})
	if err != nil {
		h.logger.Error("Failed to search locations", "error", err, "search", search)
		WriteError(w, r, NewInternalError("Failed to search locations"))
		return
	}

	response := make([]LocationResponse, len(locations))
	for i, l := range locations {
		response[i] = LocationResponse{
			NeighborhoodCode: l.NeighborhoodCode,
			NeighborhoodName: l.NeighborhoodName,
			DistrictCode:     l.DistrictCode,
			DistrictName:     l.DistrictName,
			CityCode:         l.CityCode,
			CityName:         l.CityName,
			PostalCode:       l.PostalCode,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// GetReferenceDataStats handles GET /ref/stats
// Returns counts of reference data.
func (h *ReferenceHandler) GetReferenceDataStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	counts, err := h.queries.GetReferenceDataCounts(ctx)
	if err != nil {
		h.logger.Error("Failed to get reference data counts", "error", err)
		WriteError(w, r, NewInternalError("Failed to get reference data stats"))
		return
	}

	writeJSON(w, http.StatusOK, ReferenceDataStatsResponse{
		CityCount:         counts.CityCount,
		DistrictCount:     counts.DistrictCount,
		NeighborhoodCount: counts.NeighborhoodCount,
		PostalCodeCount:   counts.PostalCodeCount,
	})
}

// Response types

// CityResponse represents a city in API responses.
type CityResponse struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// DistrictResponse represents a district in API responses.
type DistrictResponse struct {
	Code     string `json:"code"`
	CityCode string `json:"city_code"`
	Name     string `json:"name"`
}

// DistrictWithCityResponse represents a district with city info in API responses.
type DistrictWithCityResponse struct {
	Code     string `json:"code"`
	CityCode string `json:"city_code"`
	CityName string `json:"city_name"`
	Name     string `json:"name"`
}

// NeighborhoodResponse represents a neighborhood in API responses.
type NeighborhoodResponse struct {
	Code         string  `json:"code"`
	DistrictCode string  `json:"district_code"`
	Name         string  `json:"name"`
	PostalCode   *string `json:"postal_code,omitempty"`
}

// NeighborhoodWithHierarchyResponse represents a neighborhood with full hierarchy.
type NeighborhoodWithHierarchyResponse struct {
	Code         string  `json:"code"`
	DistrictCode string  `json:"district_code"`
	DistrictName string  `json:"district_name"`
	CityCode     string  `json:"city_code,omitempty"`
	CityName     string  `json:"city_name"`
	Name         string  `json:"name"`
	PostalCode   *string `json:"postal_code,omitempty"`
}

// PostalCodeResponse represents a postal code in API responses.
type PostalCodeResponse struct {
	Code         string `json:"code"`
	DistrictCode string `json:"district_code"`
}

// LocationResponse represents a full location hierarchy.
type LocationResponse struct {
	NeighborhoodCode string  `json:"neighborhood_code"`
	NeighborhoodName string  `json:"neighborhood_name"`
	DistrictCode     string  `json:"district_code"`
	DistrictName     string  `json:"district_name"`
	CityCode         string  `json:"city_code"`
	CityName         string  `json:"city_name"`
	PostalCode       *string `json:"postal_code,omitempty"`
}

// ReferenceDataStatsResponse represents reference data statistics.
type ReferenceDataStatsResponse struct {
	CityCount         int64 `json:"city_count"`
	DistrictCount     int64 `json:"district_count"`
	NeighborhoodCount int64 `json:"neighborhood_count"`
	PostalCodeCount   int64 `json:"postal_code_count"`
}

// Helper functions

func cityToResponse(c *db.RefCity) CityResponse {
	return CityResponse{
		Code: c.Code,
		Name: c.Name,
	}
}

func districtToResponse(d *db.RefDistrict) DistrictResponse {
	return DistrictResponse{
		Code:     d.Code,
		CityCode: d.CityCode,
		Name:     d.Name,
	}
}

func neighborhoodToResponse(n *db.RefNeighborhood) NeighborhoodResponse {
	return NeighborhoodResponse{
		Code:         n.Code,
		DistrictCode: n.DistrictCode,
		Name:         n.Name,
		PostalCode:   n.PostalCode,
	}
}

func postalCodeToResponse(p *db.RefPostalCode) PostalCodeResponse {
	return PostalCodeResponse{
		Code:         p.Code,
		DistrictCode: p.DistrictCode,
	}
}
