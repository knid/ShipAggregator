package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// ShopifyHandler handles authenticated Shopify API endpoints.
type ShopifyHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewShopifyHandler creates a new Shopify handler.
func NewShopifyHandler(queries *db.Queries, logger *observability.Logger) *ShopifyHandler {
	return &ShopifyHandler{
		queries: queries,
		logger:  logger,
	}
}

// ListStores returns all connected Shopify stores for the organization.
func (h *ShopifyHandler) ListStores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	stores, err := h.queries.ListShopifyStoresByOrganization(ctx, orgID)
	if err != nil {
		h.logger.Error("failed to list shopify stores", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("failed to list stores"))
		return
	}

	writeJSON(w, http.StatusOK, stores)
}

// GetStore returns details for a specific Shopify store.
func (h *ShopifyHandler) GetStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid store ID"))
		return
	}

	store, err := h.queries.GetShopifyStoreByID(ctx, storeID)
	if err != nil {
		WriteError(w, r, NewNotFoundError("Shopify store"))
		return
	}

	if store.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Shopify store"))
		return
	}

	// Don't expose encrypted token
	store.AccessTokenEncrypted = nil
	store.AccessTokenIv = nil

	writeJSON(w, http.StatusOK, store)
}

// UpdateStoreRequest is the request body for updating a Shopify store.
type UpdateStoreRequest struct {
	SyncMode *string `json:"sync_mode,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// UpdateStore updates a Shopify store's settings (sync mode, status).
func (h *ShopifyHandler) UpdateStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid store ID"))
		return
	}

	// Verify ownership
	store, err := h.queries.GetShopifyStoreByID(ctx, storeID)
	if err != nil || store.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Shopify store"))
		return
	}

	var req UpdateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("invalid request body"))
		return
	}

	// Validate sync_mode
	if req.SyncMode != nil {
		if *req.SyncMode != "manual" && *req.SyncMode != "auto" {
			WriteError(w, r, NewBadRequestError("sync_mode must be 'manual' or 'auto'"))
			return
		}
	}

	// Validate status
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "paused" {
			WriteError(w, r, NewBadRequestError("status must be 'active' or 'paused'"))
			return
		}
	}

	updated, err := h.queries.UpdateShopifyStore(ctx, db.UpdateShopifyStoreParams{
		ID:       storeID,
		SyncMode: req.SyncMode,
		Status:   req.Status,
	})
	if err != nil {
		h.logger.Error("failed to update shopify store", "error", err, "store_id", storeID)
		WriteError(w, r, NewInternalError("failed to update store"))
		return
	}

	updated.AccessTokenEncrypted = nil
	updated.AccessTokenIv = nil

	writeJSON(w, http.StatusOK, updated)
}

// DeleteStore disconnects a Shopify store.
func (h *ShopifyHandler) DeleteStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid store ID"))
		return
	}

	if err := h.queries.DeleteShopifyStore(ctx, db.DeleteShopifyStoreParams{
		ID:             storeID,
		OrganizationID: orgID,
	}); err != nil {
		h.logger.Error("failed to delete shopify store", "error", err, "store_id", storeID)
		WriteError(w, r, NewInternalError("failed to disconnect store"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListOrders returns order mappings for a Shopify store.
func (h *ShopifyHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid store ID"))
		return
	}

	// Verify ownership
	store, err := h.queries.GetShopifyStoreByID(ctx, storeID)
	if err != nil || store.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Shopify store"))
		return
	}

	// Parse pagination
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Status filter
	var statusFilter *string
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = &s
	}

	orders, err := h.queries.ListShopifyOrderMappingsByStore(ctx, db.ListShopifyOrderMappingsByStoreParams{
		ShopifyStoreID: storeID,
		StatusFilter:   statusFilter,
		LimitCount:     int32(limit),
		OffsetCount:    int32(offset),
	})
	if err != nil {
		h.logger.Error("failed to list shopify orders", "error", err, "store_id", storeID)
		WriteError(w, r, NewInternalError("failed to list orders"))
		return
	}

	total, err := h.queries.CountShopifyOrderMappingsByStore(ctx, db.CountShopifyOrderMappingsByStoreParams{
		ShopifyStoreID: storeID,
		StatusFilter:   statusFilter,
	})
	if err != nil {
		total = 0
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":   orders,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// RetryOrder re-processes a failed order mapping.
func (h *ShopifyHandler) RetryOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	mappingID, err := uuid.Parse(chi.URLParam(r, "mappingId"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid mapping ID"))
		return
	}

	mapping, err := h.queries.GetShopifyOrderMappingByID(ctx, mappingID)
	if err != nil || mapping.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("order mapping"))
		return
	}

	if mapping.Status != "failed" {
		WriteError(w, r, NewBadRequestError("can only retry failed orders"))
		return
	}

	// Reset status to pending for re-processing
	if err := h.queries.UpdateShopifyOrderMappingStatus(ctx, db.UpdateShopifyOrderMappingStatusParams{
		ID:     mappingID,
		Status: "pending",
	}); err != nil {
		h.logger.Error("failed to retry order", "error", err, "mapping_id", mappingID)
		WriteError(w, r, NewInternalError("failed to retry order"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}

// LinkOrderRequest is the request body for manually linking an order to a shipment.
type LinkOrderRequest struct {
	ShipmentID string `json:"shipment_id"`
}

// LinkOrder manually links a Shopify order mapping to a shipment.
func (h *ShopifyHandler) LinkOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	mappingID, err := uuid.Parse(chi.URLParam(r, "mappingId"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid mapping ID"))
		return
	}

	mapping, err := h.queries.GetShopifyOrderMappingByID(ctx, mappingID)
	if err != nil || mapping.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("order mapping"))
		return
	}

	var req LinkOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("invalid request body"))
		return
	}

	shipmentID, err := uuid.Parse(req.ShipmentID)
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid shipment_id"))
		return
	}

	if err := h.queries.LinkShopifyOrderToShipment(ctx, db.LinkShopifyOrderToShipmentParams{
		ID:         mappingID,
		ShipmentID: pgtype.UUID{Bytes: shipmentID, Valid: true},
		Status:     "draft_created",
	}); err != nil {
		h.logger.Error("failed to link order to shipment", "error", err, "mapping_id", mappingID)
		WriteError(w, r, NewInternalError("failed to link order"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "draft_created",
		"shipment_id": shipmentID.String(),
	})
}

// GetStoreStats returns order counts by status for a store.
func (h *ShopifyHandler) GetStoreStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid store ID"))
		return
	}

	store, err := h.queries.GetShopifyStoreByID(ctx, storeID)
	if err != nil || store.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Shopify store"))
		return
	}

	statusCounts, err := h.queries.CountShopifyOrdersByStatus(ctx, storeID)
	if err != nil {
		h.logger.Error("failed to get store stats", "error", err, "store_id", storeID)
		WriteError(w, r, NewInternalError("failed to get stats"))
		return
	}

	stats := map[string]any{
		"status_counts": statusCounts,
		"last_sync_at":  store.LastSyncAt,
		"sync_mode":     store.SyncMode,
	}

	writeJSON(w, http.StatusOK, stats)
}

// ListRules returns shipping rules for the organization.
func (h *ShopifyHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	rules, err := h.queries.ListShopifyShippingRulesByOrganization(ctx, orgID)
	if err != nil {
		h.logger.Error("failed to list shipping rules", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("failed to list rules"))
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// CreateRuleRequest is the request body for creating a shipping rule.
type CreateRuleRequest struct {
	Name                          string    `json:"name"`
	ShopifyStoreID                *string   `json:"shopify_store_id,omitempty"`
	Priority                      int       `json:"priority"`
	IsActive                      bool      `json:"is_active"`
	ConditionDestinationCityCodes []string  `json:"condition_destination_city_codes,omitempty"`
	ConditionMinWeightGrams       *int32    `json:"condition_min_weight_grams,omitempty"`
	ConditionMaxWeightGrams       *int32    `json:"condition_max_weight_grams,omitempty"`
	ConditionMinOrderTotalKurus   *int64    `json:"condition_min_order_total_kurus,omitempty"`
	ConditionMaxOrderTotalKurus   *int64    `json:"condition_max_order_total_kurus,omitempty"`
	ConditionHasCod               *bool     `json:"condition_has_cod,omitempty"`
	ConditionTags                 []string  `json:"condition_tags,omitempty"`
	ActionProviderCode            string    `json:"action_provider_code"`
	ActionServiceCode             *string   `json:"action_service_code,omitempty"`
	ActionProviderAccountID       *string   `json:"action_provider_account_id,omitempty"`
}

// CreateRule creates a new shipping rule.
func (h *ShopifyHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("invalid request body"))
		return
	}

	if req.Name == "" {
		WriteError(w, r, NewBadRequestError("name is required"))
		return
	}
	if req.ActionProviderCode == "" {
		WriteError(w, r, NewBadRequestError("action_provider_code is required"))
		return
	}

	params := db.CreateShopifyShippingRuleParams{
		OrganizationID:                orgID,
		Name:                          req.Name,
		Priority:                      int32(req.Priority),
		IsActive:                      req.IsActive,
		ConditionDestinationCityCodes: req.ConditionDestinationCityCodes,
		ConditionMinWeightGrams:       req.ConditionMinWeightGrams,
		ConditionMaxWeightGrams:       req.ConditionMaxWeightGrams,
		ConditionMinOrderTotalKurus:   req.ConditionMinOrderTotalKurus,
		ConditionMaxOrderTotalKurus:   req.ConditionMaxOrderTotalKurus,
		ConditionHasCod:               req.ConditionHasCod,
		ConditionTags:                 req.ConditionTags,
		ActionProviderCode:            req.ActionProviderCode,
		ActionServiceCode:             req.ActionServiceCode,
	}

	if req.ShopifyStoreID != nil {
		sid, err := uuid.Parse(*req.ShopifyStoreID)
		if err != nil {
			WriteError(w, r, NewBadRequestError("invalid shopify_store_id"))
			return
		}
		params.ShopifyStoreID = pgtype.UUID{Bytes: sid, Valid: true}
	}

	if req.ActionProviderAccountID != nil {
		aid, err := uuid.Parse(*req.ActionProviderAccountID)
		if err != nil {
			WriteError(w, r, NewBadRequestError("invalid action_provider_account_id"))
			return
		}
		params.ActionProviderAccountID = pgtype.UUID{Bytes: aid, Valid: true}
	}

	rule, err := h.queries.CreateShopifyShippingRule(ctx, params)
	if err != nil {
		h.logger.Error("failed to create shipping rule", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("failed to create rule"))
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// UpdateRule updates a shipping rule.
func (h *ShopifyHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	ruleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid rule ID"))
		return
	}

	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("invalid request body"))
		return
	}

	priority := int32(req.Priority)
	updated, err := h.queries.UpdateShopifyShippingRule(ctx, db.UpdateShopifyShippingRuleParams{
		ID:                            ruleID,
		OrganizationID:                orgID,
		Name:                          &req.Name,
		Priority:                      &priority,
		IsActive:                      &req.IsActive,
		ConditionDestinationCityCodes: req.ConditionDestinationCityCodes,
		ConditionMinWeightGrams:       req.ConditionMinWeightGrams,
		ConditionMaxWeightGrams:       req.ConditionMaxWeightGrams,
		ConditionMinOrderTotalKurus:   req.ConditionMinOrderTotalKurus,
		ConditionMaxOrderTotalKurus:   req.ConditionMaxOrderTotalKurus,
		ConditionHasCod:               req.ConditionHasCod,
		ConditionTags:                 req.ConditionTags,
		ActionProviderCode:            &req.ActionProviderCode,
		ActionServiceCode:             req.ActionServiceCode,
	})
	if err != nil {
		h.logger.Error("failed to update shipping rule", "error", err, "rule_id", ruleID)
		WriteError(w, r, NewInternalError("failed to update rule"))
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteRule deletes a shipping rule.
func (h *ShopifyHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := middleware.GetOrganizationID(ctx)

	ruleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, r, NewBadRequestError("invalid rule ID"))
		return
	}

	if err := h.queries.DeleteShopifyShippingRule(ctx, db.DeleteShopifyShippingRuleParams{
		ID:             ruleID,
		OrganizationID: orgID,
	}); err != nil {
		h.logger.Error("failed to delete shipping rule", "error", err, "rule_id", ruleID)
		WriteError(w, r, NewInternalError("failed to delete rule"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
