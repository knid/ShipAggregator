// Package handler provides HTTP handlers for the API.
package handler

import (
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
	"github.com/e-port/shipping-aggregator/internal/service/billing"
)

// BillingHandler handles billing-related HTTP requests.
type BillingHandler struct {
	walletService *billing.WalletService
	ledgerService *billing.LedgerService
	codService    *billing.CODService
	logger        *observability.Logger
}

// NewBillingHandler creates a new billing handler.
func NewBillingHandler(walletService *billing.WalletService, ledgerService *billing.LedgerService, logger *observability.Logger) *BillingHandler {
	return &BillingHandler{
		walletService: walletService,
		ledgerService: ledgerService,
		logger:        logger,
	}
}

// NewBillingHandlerWithCOD creates a new billing handler with COD support.
func NewBillingHandlerWithCOD(walletService *billing.WalletService, ledgerService *billing.LedgerService, codService *billing.CODService, logger *observability.Logger) *BillingHandler {
	return &BillingHandler{
		walletService: walletService,
		ledgerService: ledgerService,
		codService:    codService,
		logger:        logger,
	}
}

// GetBalance handles GET /billing/balance
// Returns the current wallet balance for the organization.
func (h *BillingHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get balance
	balance, err := h.walletService.GetBalance(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to get balance", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get balance"))
		return
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		OrganizationID:    orgID,
		BalanceKurus:      balance.BalanceKurus,
		AvailableKurus:    balance.AvailableKurus,
		PendingKurus:      balance.PendingKurus,
		TotalCredits:      balance.TotalCredits,
		TotalDebits:       balance.TotalDebits,
		LastTransactionAt: formatOptionalTime(balance.LastTransactionAt),
		AsOf:              balance.AsOf.Format(time.RFC3339),
	})
}

// GetLedger handles GET /billing/ledger
// Returns ledger entries for the organization.
func (h *BillingHandler) GetLedger(w http.ResponseWriter, r *http.Request) {
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

	var entryType *string
	if t := r.URL.Query().Get("type"); t != "" {
		entryType = &t
	}

	var referenceType *domain.ReferenceType
	if rt := r.URL.Query().Get("reference_type"); rt != "" {
		refType := domain.ReferenceType(rt)
		referenceType = &refType
	}

	var referenceID *uuid.UUID
	if rid := r.URL.Query().Get("reference_id"); rid != "" {
		if id, err := uuid.Parse(rid); err == nil {
			referenceID = &id
		}
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

	// Get ledger entries
	result, err := h.ledgerService.List(ctx, &billing.ListLedgerRequest{
		OrganizationID: orgID,
		EntryType:      entryType,
		ReferenceType:  referenceType,
		ReferenceID:    referenceID,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list ledger entries", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list ledger entries"))
		return
	}

	response := make([]LedgerEntryResponse, len(result.Entries))
	for i, entry := range result.Entries {
		response[i] = ledgerEntryToResponse(entry)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  result.Total,
		Offset: offset,
		Limit:  limit,
	})
}

// GetLedgerEntry handles GET /billing/ledger/{id}
// Returns a specific ledger entry.
func (h *BillingHandler) GetLedgerEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get entry ID from URL
	entryIDStr := r.URL.Query().Get("id")
	entryID, err := uuid.Parse(entryIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid entry ID"))
		return
	}

	// Get ledger entry
	entry, err := h.ledgerService.Get(ctx, entryID, orgID)
	if err != nil {
		if err.Error() == "ledger entry not found" {
			WriteError(w, r, NewNotFoundError("Ledger entry"))
			return
		}
		h.logger.Error("Failed to get ledger entry", "error", err, "entry_id", entryID)
		WriteError(w, r, NewInternalError("Failed to get ledger entry"))
		return
	}

	writeJSON(w, http.StatusOK, ledgerEntryToResponse(entry))
}

// GetLedgerSummary handles GET /billing/ledger/summary
// Returns a summary of ledger activity.
func (h *BillingHandler) GetLedgerSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse date range
	var startDate, endDate time.Time
	if sd := r.URL.Query().Get("start_date"); sd != "" {
		if t, err := time.Parse("2006-01-02", sd); err == nil {
			startDate = t
		}
	}
	if ed := r.URL.Query().Get("end_date"); ed != "" {
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			endDate = t.Add(24*time.Hour - time.Second) // End of day
		}
	}

	// Default to last 30 days
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, 0, -30)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}

	// Get summary
	summary, err := h.ledgerService.GetSummary(ctx, orgID, startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get ledger summary", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get ledger summary"))
		return
	}

	writeJSON(w, http.StatusOK, LedgerSummaryResponse{
		OrganizationID:   orgID,
		StartDate:        startDate.Format("2006-01-02"),
		EndDate:          endDate.Format("2006-01-02"),
		TotalCredits:     summary.TotalCredits,
		TotalDebits:      summary.TotalDebits,
		NetChange:        summary.NetChange,
		TransactionCount: summary.TransactionCount,
		ByType:           summary.ByType,
		ByReferenceType:  summary.ByReferenceType,
	})
}

// Response types

// BalanceResponse represents a wallet balance in API responses.
type BalanceResponse struct {
	OrganizationID    uuid.UUID `json:"organization_id"`
	BalanceKurus      int64     `json:"balance_kurus"`
	AvailableKurus    int64     `json:"available_kurus"`
	PendingKurus      int64     `json:"pending_kurus"`
	TotalCredits      int64     `json:"total_credits"`
	TotalDebits       int64     `json:"total_debits"`
	LastTransactionAt *string   `json:"last_transaction_at,omitempty"`
	AsOf              string    `json:"as_of"`
}

// LedgerEntryResponse represents a ledger entry in API responses.
type LedgerEntryResponse struct {
	ID                uuid.UUID              `json:"id"`
	OrganizationID    uuid.UUID              `json:"organization_id"`
	EntryType         string                 `json:"entry_type"`
	AmountKurus       int64                  `json:"amount_kurus"`
	BalanceAfterKurus int64                  `json:"balance_after_kurus"`
	ReferenceType     string                 `json:"reference_type"`
	ReferenceID       uuid.UUID              `json:"reference_id"`
	Description       string                 `json:"description"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ActorType         *string                `json:"actor_type,omitempty"`
	ActorID           *uuid.UUID             `json:"actor_id,omitempty"`
	CreatedAt         string                 `json:"created_at"`
}

// LedgerSummaryResponse represents a ledger summary in API responses.
type LedgerSummaryResponse struct {
	OrganizationID   uuid.UUID        `json:"organization_id"`
	StartDate        string           `json:"start_date"`
	EndDate          string           `json:"end_date"`
	TotalCredits     int64            `json:"total_credits"`
	TotalDebits      int64            `json:"total_debits"`
	NetChange        int64            `json:"net_change"`
	TransactionCount int64            `json:"transaction_count"`
	ByType           map[string]int64 `json:"by_type"`
	ByReferenceType  map[string]int64 `json:"by_reference_type"`
}

// Conversion helpers

func ledgerEntryToResponse(entry *billing.LedgerEntry) LedgerEntryResponse {
	resp := LedgerEntryResponse{
		ID:                entry.ID,
		OrganizationID:    entry.OrganizationID,
		EntryType:         entry.EntryType,
		AmountKurus:       entry.AmountKurus,
		BalanceAfterKurus: entry.BalanceAfterKurus,
		ReferenceType:     entry.ReferenceType,
		ReferenceID:       entry.ReferenceID,
		Description:       entry.Description,
		Metadata:          entry.Metadata,
		CreatedAt:         entry.CreatedAt.Format(time.RFC3339),
	}

	if entry.ActorType != "" {
		resp.ActorType = &entry.ActorType
	}
	if entry.ActorID != nil {
		resp.ActorID = entry.ActorID
	}

	return resp
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// COD Settlement Endpoints

// ListCODSettlements handles GET /billing/cod/settlements
// Returns COD settlements for the organization.
func (h *BillingHandler) ListCODSettlements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
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
	if pc := r.URL.Query().Get("provider_code"); pc != "" {
		providerCode = &pc
	}

	// Get COD settlements
	result, err := h.codService.ListSettlements(ctx, &billing.ListSettlementsRequest{
		OrganizationID: orgID,
		Status:         status,
		ProviderCode:   providerCode,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list COD settlements", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list COD settlements"))
		return
	}

	response := make([]CODSettlementResponse, len(result.Settlements))
	for i, settlement := range result.Settlements {
		response[i] = codSettlementToResponse(settlement)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  result.Total,
		Offset: offset,
		Limit:  limit,
	})
}

// GetCODSettlement handles GET /billing/cod/settlements/{id}
// Returns a specific COD settlement.
func (h *BillingHandler) GetCODSettlement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
		return
	}

	// Get settlement ID from URL
	settlementIDStr := chi.URLParam(r, "id")
	settlementID, err := uuid.Parse(settlementIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid settlement ID"))
		return
	}

	// Get COD settlement
	settlement, err := h.codService.GetSettlement(ctx, settlementID)
	if err != nil {
		if errors.Is(err, billing.ErrCODSettlementNotFound) {
			WriteError(w, r, NewNotFoundError("COD settlement"))
			return
		}
		h.logger.Error("Failed to get COD settlement", "error", err, "settlement_id", settlementID)
		WriteError(w, r, NewInternalError("Failed to get COD settlement"))
		return
	}

	// Verify organization ownership
	if settlement.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("COD settlement"))
		return
	}

	writeJSON(w, http.StatusOK, codSettlementToResponse(settlement))
}

// GetCODSettlementSummary handles GET /billing/cod/settlements/summary
// Returns a summary of COD settlements for the organization.
func (h *BillingHandler) GetCODSettlementSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
		return
	}

	// Get summary
	summary, err := h.codService.GetSummary(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to get COD settlement summary", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get COD settlement summary"))
		return
	}

	// Get summary by provider
	providerSummaries, err := h.codService.GetSummaryByProvider(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to get COD settlement summary by provider", "error", err, "org_id", orgID)
		// Continue without provider breakdown
		providerSummaries = nil
	}

	providerResponses := make([]CODProviderSummaryResponse, len(providerSummaries))
	for i, ps := range providerSummaries {
		providerResponses[i] = CODProviderSummaryResponse{
			ProviderCode:       ps.ProviderCode,
			TotalSettlements:   ps.TotalSettlements,
			TotalExpectedKurus: ps.TotalExpectedKurus,
			TotalSettledKurus:  ps.TotalSettledKurus,
			TotalPendingKurus:  ps.TotalPendingKurus,
		}
	}

	writeJSON(w, http.StatusOK, CODSettlementSummaryResponse{
		OrganizationID:     orgID,
		TotalSettlements:   summary.TotalSettlements,
		TotalExpectedKurus: summary.TotalExpectedKurus,
		TotalSettledKurus:  summary.TotalSettledKurus,
		TotalPendingKurus:  summary.TotalPendingKurus,
		TotalDisputedKurus: summary.TotalDisputedKurus,
		ByProvider:         providerResponses,
	})
}

// ReconcileCODSettlements handles POST /billing/cod/settlements/reconcile
// Reconciles COD settlements (admin only).
func (h *BillingHandler) ReconcileCODSettlements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context (for admin, this might be different)
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
		return
	}

	// Parse request body
	var req ReconcileCODSettlementsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if req.ProviderCode == "" {
		WriteError(w, r, NewBadRequestError("provider_code is required"))
		return
	}
	if req.SettlementDate == "" {
		WriteError(w, r, NewBadRequestError("settlement_date is required"))
		return
	}

	// Parse settlement date
	settlementDate, err := time.Parse("2006-01-02", req.SettlementDate)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid settlement_date format (expected YYYY-MM-DD)"))
		return
	}

	// Get actor info
	actorID := middleware.GetActorID(ctx)
	actorType := domain.ActorType(middleware.GetActorType(ctx))

	// Perform batch reconciliation
	var actorIDPtr *uuid.UUID
	if actorID != uuid.Nil {
		actorIDPtr = &actorID
	}
	result, err := h.codService.BatchReconcile(ctx, &billing.BatchReconcileRequest{
		ProviderCode:          req.ProviderCode,
		CarrierSettlementDate: settlementDate,
		ActorID:               actorIDPtr,
		ActorType:             actorType,
	})
	if err != nil {
		h.logger.Error("Failed to reconcile COD settlements", "error", err, "provider_code", req.ProviderCode)
		WriteError(w, r, NewInternalError("Failed to reconcile COD settlements"))
		return
	}

	// Build response
	settlements := make([]CODSettlementResponse, len(result.Settlements))
	for i, settlement := range result.Settlements {
		settlements[i] = codSettlementToResponse(settlement)
	}

	writeJSON(w, http.StatusOK, ReconcileCODSettlementsResponse{
		Reconciled:  result.Reconciled,
		Failed:      result.Failed,
		TotalKurus:  result.TotalKurus,
		Errors:      result.Errors,
		Settlements: settlements,
	})
}

// MarkCODSettlementCollected handles POST /billing/cod/settlements/{id}/collected
// Marks a COD settlement as collected (admin only).
func (h *BillingHandler) MarkCODSettlementCollected(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
		return
	}

	// Get settlement ID from URL
	settlementIDStr := chi.URLParam(r, "id")
	settlementID, err := uuid.Parse(settlementIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid settlement ID"))
		return
	}

	// Parse request body
	var req MarkCODCollectedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if req.ActualAmountKurus <= 0 {
		WriteError(w, r, NewBadRequestError("actual_amount_kurus must be positive"))
		return
	}
	if req.CarrierSettlementID == "" {
		WriteError(w, r, NewBadRequestError("carrier_settlement_id is required"))
		return
	}
	if req.CarrierSettlementDate == "" {
		WriteError(w, r, NewBadRequestError("carrier_settlement_date is required"))
		return
	}

	// Parse settlement date
	settlementDate, err := time.Parse("2006-01-02", req.CarrierSettlementDate)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid carrier_settlement_date format (expected YYYY-MM-DD)"))
		return
	}

	// Mark as collected
	settlement, err := h.codService.MarkCollected(ctx, &billing.MarkCollectedRequest{
		SettlementID:          settlementID,
		ActualAmountKurus:     req.ActualAmountKurus,
		CarrierSettlementID:   req.CarrierSettlementID,
		CarrierSettlementDate: settlementDate,
	})
	if err != nil {
		if errors.Is(err, billing.ErrCODSettlementNotFound) {
			WriteError(w, r, NewNotFoundError("COD settlement"))
			return
		}
		if errors.Is(err, billing.ErrCODSettlementInvalidState) {
			WriteError(w, r, NewBadRequestError("COD settlement is not in pending state"))
			return
		}
		h.logger.Error("Failed to mark COD settlement as collected", "error", err, "settlement_id", settlementID)
		WriteError(w, r, NewInternalError("Failed to mark COD settlement as collected"))
		return
	}

	writeJSON(w, http.StatusOK, codSettlementToResponse(settlement))
}

// DisputeCODSettlement handles POST /billing/cod/settlements/{id}/dispute
// Marks a COD settlement as disputed.
func (h *BillingHandler) DisputeCODSettlement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if COD service is available
	if h.codService == nil {
		WriteError(w, r, NewInternalError("COD service not available"))
		return
	}

	// Get settlement ID from URL
	settlementIDStr := chi.URLParam(r, "id")
	settlementID, err := uuid.Parse(settlementIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid settlement ID"))
		return
	}

	// Parse request body
	var req DisputeCODSettlementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if req.Reason == "" {
		WriteError(w, r, NewBadRequestError("reason is required"))
		return
	}

	// Verify ownership first
	existingSettlement, err := h.codService.GetSettlement(ctx, settlementID)
	if err != nil {
		if errors.Is(err, billing.ErrCODSettlementNotFound) {
			WriteError(w, r, NewNotFoundError("COD settlement"))
			return
		}
		h.logger.Error("Failed to get COD settlement", "error", err, "settlement_id", settlementID)
		WriteError(w, r, NewInternalError("Failed to get COD settlement"))
		return
	}

	if existingSettlement.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("COD settlement"))
		return
	}

	// Dispute settlement
	settlement, err := h.codService.Dispute(ctx, &billing.DisputeRequest{
		SettlementID:      settlementID,
		DiscrepancyReason: req.Reason,
	})
	if err != nil {
		if errors.Is(err, billing.ErrCODSettlementInvalidState) {
			WriteError(w, r, NewBadRequestError("COD settlement cannot be disputed in current state"))
			return
		}
		h.logger.Error("Failed to dispute COD settlement", "error", err, "settlement_id", settlementID)
		WriteError(w, r, NewInternalError("Failed to dispute COD settlement"))
		return
	}

	writeJSON(w, http.StatusOK, codSettlementToResponse(settlement))
}

// COD Settlement Response Types

// CODSettlementResponse represents a COD settlement in API responses.
type CODSettlementResponse struct {
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	ShipmentID            uuid.UUID  `json:"shipment_id"`
	ProviderCode          string     `json:"provider_code"`
	ExpectedAmountKurus   int32      `json:"expected_amount_kurus"`
	ActualAmountKurus     *int32     `json:"actual_amount_kurus,omitempty"`
	CarrierSettlementID   *string    `json:"carrier_settlement_id,omitempty"`
	CarrierSettlementDate *string    `json:"carrier_settlement_date,omitempty"`
	Status                string     `json:"status"`
	LedgerEntryID         *uuid.UUID `json:"ledger_entry_id,omitempty"`
	DiscrepancyKurus      *int32     `json:"discrepancy_kurus,omitempty"`
	DiscrepancyReason     *string    `json:"discrepancy_reason,omitempty"`
	SettledAt             *string    `json:"settled_at,omitempty"`
	CreatedAt             string     `json:"created_at"`
	UpdatedAt             string     `json:"updated_at"`
}

// CODSettlementSummaryResponse represents a COD settlement summary in API responses.
type CODSettlementSummaryResponse struct {
	OrganizationID     uuid.UUID                    `json:"organization_id"`
	TotalSettlements   int64                        `json:"total_settlements"`
	TotalExpectedKurus int64                        `json:"total_expected_kurus"`
	TotalSettledKurus  int64                        `json:"total_settled_kurus"`
	TotalPendingKurus  int64                        `json:"total_pending_kurus"`
	TotalDisputedKurus int64                        `json:"total_disputed_kurus"`
	ByProvider         []CODProviderSummaryResponse `json:"by_provider,omitempty"`
}

// CODProviderSummaryResponse represents a COD settlement summary by provider.
type CODProviderSummaryResponse struct {
	ProviderCode       string `json:"provider_code"`
	TotalSettlements   int64  `json:"total_settlements"`
	TotalExpectedKurus int64  `json:"total_expected_kurus"`
	TotalSettledKurus  int64  `json:"total_settled_kurus"`
	TotalPendingKurus  int64  `json:"total_pending_kurus"`
}

// ReconcileCODSettlementsRequest represents a request to reconcile COD settlements.
type ReconcileCODSettlementsRequest struct {
	ProviderCode   string `json:"provider_code"`
	SettlementDate string `json:"settlement_date"`
}

// ReconcileCODSettlementsResponse represents the result of reconciling COD settlements.
type ReconcileCODSettlementsResponse struct {
	Reconciled  int                     `json:"reconciled"`
	Failed      int                     `json:"failed"`
	TotalKurus  int64                   `json:"total_kurus"`
	Errors      []string                `json:"errors,omitempty"`
	Settlements []CODSettlementResponse `json:"settlements"`
}

// MarkCODCollectedRequest represents a request to mark a COD settlement as collected.
type MarkCODCollectedRequest struct {
	ActualAmountKurus     int32  `json:"actual_amount_kurus"`
	CarrierSettlementID   string `json:"carrier_settlement_id"`
	CarrierSettlementDate string `json:"carrier_settlement_date"`
}

// DisputeCODSettlementRequest represents a request to dispute a COD settlement.
type DisputeCODSettlementRequest struct {
	Reason string `json:"reason"`
}

// Conversion helper for COD settlements
func codSettlementToResponse(settlement *billing.CODSettlement) CODSettlementResponse {
	resp := CODSettlementResponse{
		ID:                  settlement.ID,
		OrganizationID:      settlement.OrganizationID,
		ShipmentID:          settlement.ShipmentID,
		ProviderCode:        settlement.ProviderCode,
		ExpectedAmountKurus: settlement.ExpectedAmountKurus,
		ActualAmountKurus:   settlement.ActualAmountKurus,
		CarrierSettlementID: settlement.CarrierSettlementID,
		Status:              settlement.Status,
		LedgerEntryID:       settlement.LedgerEntryID,
		DiscrepancyKurus:    settlement.DiscrepancyKurus,
		DiscrepancyReason:   settlement.DiscrepancyReason,
		CreatedAt:           settlement.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           settlement.UpdatedAt.Format(time.RFC3339),
	}

	if settlement.CarrierSettlementDate != nil {
		date := settlement.CarrierSettlementDate.Format("2006-01-02")
		resp.CarrierSettlementDate = &date
	}

	if settlement.SettledAt != nil {
		settledAt := settlement.SettledAt.Format(time.RFC3339)
		resp.SettledAt = &settledAt
	}

	return resp
}
