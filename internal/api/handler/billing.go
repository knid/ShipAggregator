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

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/service/billing"
)

// BillingHandler handles billing-related HTTP requests.
type BillingHandler struct {
	walletService *billing.WalletService
	ledgerService *billing.LedgerService
	codService    *billing.CODService
	topupService  *billing.TopupService
	refundService *billing.RefundService
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

// NewBillingHandlerFull creates a new billing handler with all services.
func NewBillingHandlerFull(
	walletService *billing.WalletService,
	ledgerService *billing.LedgerService,
	codService *billing.CODService,
	topupService *billing.TopupService,
	logger *observability.Logger,
) *BillingHandler {
	return &BillingHandler{
		walletService: walletService,
		ledgerService: ledgerService,
		codService:    codService,
		topupService:  topupService,
		logger:        logger,
	}
}

// NewBillingHandlerComplete creates a new billing handler with all services including refunds.
func NewBillingHandlerComplete(
	walletService *billing.WalletService,
	ledgerService *billing.LedgerService,
	codService *billing.CODService,
	topupService *billing.TopupService,
	refundService *billing.RefundService,
	logger *observability.Logger,
) *BillingHandler {
	return &BillingHandler{
		walletService: walletService,
		ledgerService: ledgerService,
		codService:    codService,
		topupService:  topupService,
		refundService: refundService,
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

// TransactionResponse represents a unified transaction in API responses.
type TransactionResponse struct {
	ID            uuid.UUID              `json:"id"`
	Type          string                 `json:"type"` // ledger, topup, refund, cod_settlement
	AmountKurus   int64                  `json:"amount_kurus"`
	Direction     string                 `json:"direction"` // credit, debit
	Status        string                 `json:"status"`
	Description   string                 `json:"description"`
	ReferenceType string                 `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID             `json:"reference_id,omitempty"`
	BalanceAfter  *int64                 `json:"balance_after_kurus,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     string                 `json:"created_at"`
}

// TransactionsResponse represents the combined transactions response.
type TransactionsResponse struct {
	Data   []TransactionResponse `json:"data"`
	Total  int64                 `json:"total"`
	Offset int                   `json:"offset"`
	Limit  int                   `json:"limit"`
}

// GetTransactions handles GET /billing/transactions
// Returns a combined view of all billing transactions.
func (h *BillingHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
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

	// Parse type filter (ledger, topup, refund, cod_settlement)
	typeFilter := r.URL.Query().Get("type")

	// Parse date range
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

	// Collect transactions from all sources
	var transactions []TransactionResponse
	var totalCount int64

	// Get ledger entries if no type filter or type is "ledger"
	if typeFilter == "" || typeFilter == "ledger" {
		if h.ledgerService != nil {
			result, err := h.ledgerService.List(ctx, &billing.ListLedgerRequest{
				OrganizationID: orgID,
				CreatedAfter:   createdAfter,
				CreatedBefore:  createdBefore,
				Offset:         0,
				Limit:          limit * 2, // Get more to allow for sorting
			})
			if err == nil {
				for _, entry := range result.Entries {
					direction := "credit"
					if entry.AmountKurus < 0 {
						direction = "debit"
					}
					absAmount := entry.AmountKurus
					if absAmount < 0 {
						absAmount = -absAmount
					}

					var refID *uuid.UUID
					if entry.ReferenceID != uuid.Nil {
						refID = &entry.ReferenceID
					}

					transactions = append(transactions, TransactionResponse{
						ID:            entry.ID,
						Type:          "ledger",
						AmountKurus:   absAmount,
						Direction:     direction,
						Status:        "completed",
						Description:   entry.Description,
						ReferenceType: entry.ReferenceType,
						ReferenceID:   refID,
						BalanceAfter:  &entry.BalanceAfterKurus,
						Metadata:      entry.Metadata,
						CreatedAt:     entry.CreatedAt.Format(time.RFC3339),
					})
				}
				totalCount += result.Total
			}
		}
	}

	// Get topups if no type filter or type is "topup"
	if typeFilter == "" || typeFilter == "topup" {
		if h.topupService != nil {
			result, err := h.topupService.List(ctx, &billing.ListTopupsRequest{
				OrganizationID: orgID,
				Offset:         0,
				Limit:          limit * 2,
			})
			if err == nil {
				for _, topup := range result.Topups {
					// Filter by date if specified
					if createdAfter != nil && topup.CreatedAt.Before(*createdAfter) {
						continue
					}
					if createdBefore != nil && topup.CreatedAt.After(*createdBefore) {
						continue
					}

					transactions = append(transactions, TransactionResponse{
						ID:          topup.ID,
						Type:        "topup",
						AmountKurus: topup.AmountKurus,
						Direction:   "credit",
						Status:      topup.Status,
						Description: "Wallet top-up via " + topup.PaymentProvider,
						CreatedAt:   topup.CreatedAt.Format(time.RFC3339),
					})
				}
				totalCount += result.Total
			}
		}
	}

	// Get refunds if no type filter or type is "refund"
	if typeFilter == "" || typeFilter == "refund" {
		if h.refundService != nil {
			result, err := h.refundService.List(ctx, &billing.ListRefundsRequest{
				OrganizationID: orgID,
				CreatedAfter:   createdAfter,
				CreatedBefore:  createdBefore,
				Offset:         0,
				Limit:          limit * 2,
			})
			if err == nil {
				for _, refund := range result.Refunds {
					transactions = append(transactions, TransactionResponse{
						ID:            refund.ID,
						Type:          "refund",
						AmountKurus:   refund.AmountKurus,
						Direction:     "credit",
						Status:        refund.Status,
						Description:   refund.Reason,
						ReferenceType: refund.ReferenceType,
						ReferenceID:   &refund.ReferenceID,
						CreatedAt:     refund.CreatedAt.Format(time.RFC3339),
					})
				}
				totalCount += result.Total
			}
		}
	}

	// Get COD settlements if no type filter or type is "cod_settlement"
	if typeFilter == "" || typeFilter == "cod_settlement" {
		if h.codService != nil {
			result, err := h.codService.ListSettlements(ctx, &billing.ListSettlementsRequest{
				OrganizationID: orgID,
				Offset:         0,
				Limit:          limit * 2,
			})
			if err == nil {
				for _, settlement := range result.Settlements {
					// Filter by date if specified
					if createdAfter != nil && settlement.CreatedAt.Before(*createdAfter) {
						continue
					}
					if createdBefore != nil && settlement.CreatedAt.After(*createdBefore) {
						continue
					}

					amount := int64(settlement.ExpectedAmountKurus)
					if settlement.ActualAmountKurus != nil {
						amount = int64(*settlement.ActualAmountKurus)
					}

					transactions = append(transactions, TransactionResponse{
						ID:            settlement.ID,
						Type:          "cod_settlement",
						AmountKurus:   amount,
						Direction:     "credit",
						Status:        settlement.Status,
						Description:   "COD settlement from " + settlement.ProviderCode,
						ReferenceType: "shipment",
						ReferenceID:   &settlement.ShipmentID,
						CreatedAt:     settlement.CreatedAt.Format(time.RFC3339),
					})
				}
				totalCount += result.Total
			}
		}
	}

	// Sort transactions by created_at descending
	sortTransactionsByDate(transactions)

	// Apply pagination
	start := offset
	if start > len(transactions) {
		start = len(transactions)
	}
	end := start + limit
	if end > len(transactions) {
		end = len(transactions)
	}
	paginatedTransactions := transactions[start:end]

	writeJSON(w, http.StatusOK, TransactionsResponse{
		Data:   paginatedTransactions,
		Total:  totalCount,
		Offset: offset,
		Limit:  limit,
	})
}

// sortTransactionsByDate sorts transactions by created_at in descending order.
func sortTransactionsByDate(transactions []TransactionResponse) {
	for i := 0; i < len(transactions)-1; i++ {
		for j := i + 1; j < len(transactions); j++ {
			// Parse dates and compare
			ti, _ := time.Parse(time.RFC3339, transactions[i].CreatedAt)
			tj, _ := time.Parse(time.RFC3339, transactions[j].CreatedAt)
			if tj.After(ti) {
				transactions[i], transactions[j] = transactions[j], transactions[i]
			}
		}
	}
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

// Topup Endpoints (US4: Wallet Top-Up via PayTR)

// CreateTopup handles POST /billing/topups
// Creates a new wallet top-up and returns the payment URL.
func (h *BillingHandler) CreateTopup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if topup service is available
	if h.topupService == nil {
		WriteError(w, r, NewInternalError("Topup service not available"))
		return
	}

	// Parse request body
	var req CreateTopupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if req.AmountKurus <= 0 {
		WriteError(w, r, NewBadRequestError("amount_kurus must be positive"))
		return
	}
	// Get idempotency key from header or generate one
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Get client IP
	clientIP := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		clientIP = forwardedFor
	}

	// Create topup
	result, err := h.topupService.Create(ctx, &billing.CreateTopupRequest{
		OrganizationID: orgID,
		AmountKurus:    req.AmountKurus,
		IdempotencyKey: idempotencyKey,
		OrgName:        req.OrgName,
		OrgEmail:       req.OrgEmail,
		OrgPhone:       req.OrgPhone,
		OrgAddress:     req.OrgAddress,
		OrgCity:        req.OrgCity,
		ClientIP:       clientIP,
	})
	if err != nil {
		if errors.Is(err, billing.ErrTopupAmountTooLow) {
			WriteError(w, r, NewBadRequestError("Amount is below minimum"))
			return
		}
		if errors.Is(err, billing.ErrTopupAmountTooHigh) {
			WriteError(w, r, NewBadRequestError("Amount exceeds maximum"))
			return
		}
		if errors.Is(err, billing.ErrPaymentInitFailed) {
			WriteError(w, r, NewBadRequestError("Payment initialization failed"))
			return
		}
		h.logger.Error("Failed to create topup", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to create topup"))
		return
	}

	writeJSON(w, http.StatusCreated, CreateTopupResponse{
		Topup:      topupToResponse(result.Topup),
		PaymentURL: result.PaymentURL,
	})
}

// GetTopup handles GET /billing/topups/{id}
// Returns a specific topup.
func (h *BillingHandler) GetTopup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if topup service is available
	if h.topupService == nil {
		WriteError(w, r, NewInternalError("Topup service not available"))
		return
	}

	// Get topup ID from URL
	topupIDStr := chi.URLParam(r, "id")
	topupID, err := uuid.Parse(topupIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid topup ID"))
		return
	}

	// Get topup
	topup, err := h.topupService.GetByOrganization(ctx, topupID, orgID)
	if err != nil {
		if errors.Is(err, billing.ErrTopupNotFound) {
			WriteError(w, r, NewNotFoundError("Topup"))
			return
		}
		h.logger.Error("Failed to get topup", "error", err, "topup_id", topupID)
		WriteError(w, r, NewInternalError("Failed to get topup"))
		return
	}

	writeJSON(w, http.StatusOK, topupToResponse(topup))
}

// ListTopups handles GET /billing/topups
// Returns topups for the organization.
func (h *BillingHandler) ListTopups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if topup service is available
	if h.topupService == nil {
		WriteError(w, r, NewInternalError("Topup service not available"))
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

	// Get topups
	result, err := h.topupService.List(ctx, &billing.ListTopupsRequest{
		OrganizationID: orgID,
		Status:         status,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list topups", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list topups"))
		return
	}

	response := make([]TopupResponse, len(result.Topups))
	for i, topup := range result.Topups {
		response[i] = topupToResponse(topup)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  result.Total,
		Offset: offset,
		Limit:  limit,
	})
}

// ConfirmTopup handles POST /billing/topups/{id}/confirm
// Confirms a topup after payment completion.
func (h *BillingHandler) ConfirmTopup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if topup service is available
	if h.topupService == nil {
		WriteError(w, r, NewInternalError("Topup service not available"))
		return
	}

	// Get topup ID from URL
	topupIDStr := chi.URLParam(r, "id")
	topupID, err := uuid.Parse(topupIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid topup ID"))
		return
	}

	// Verify organization ownership first
	existingTopup, err := h.topupService.GetByOrganization(ctx, topupID, orgID)
	if err != nil {
		if errors.Is(err, billing.ErrTopupNotFound) {
			WriteError(w, r, NewNotFoundError("Topup"))
			return
		}
		h.logger.Error("Failed to get topup", "error", err, "topup_id", topupID)
		WriteError(w, r, NewInternalError("Failed to get topup"))
		return
	}

	// Check if already completed
	if existingTopup.Status == string(domain.TopupStatusCompleted) {
		writeJSON(w, http.StatusOK, topupToResponse(existingTopup))
		return
	}

	// Confirm topup
	topup, err := h.topupService.Confirm(ctx, &billing.ConfirmTopupRequest{
		TopupID: topupID,
	})
	if err != nil {
		if errors.Is(err, billing.ErrTopupNotFound) {
			WriteError(w, r, NewNotFoundError("Topup"))
			return
		}
		if errors.Is(err, billing.ErrTopupExpired) {
			WriteError(w, r, NewBadRequestError("Topup has expired"))
			return
		}
		if errors.Is(err, billing.ErrTopupInvalidState) {
			WriteError(w, r, NewBadRequestError("Topup is in invalid state for confirmation"))
			return
		}
		if errors.Is(err, billing.ErrPaymentConfirmFailed) {
			WriteError(w, r, NewBadRequestError("Payment confirmation failed"))
			return
		}
		h.logger.Error("Failed to confirm topup", "error", err, "topup_id", topupID)
		WriteError(w, r, NewInternalError("Failed to confirm topup"))
		return
	}

	writeJSON(w, http.StatusOK, topupToResponse(topup))
}

// CancelTopup handles POST /billing/topups/{id}/cancel
// Cancels a pending topup.
func (h *BillingHandler) CancelTopup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if topup service is available
	if h.topupService == nil {
		WriteError(w, r, NewInternalError("Topup service not available"))
		return
	}

	// Get topup ID from URL
	topupIDStr := chi.URLParam(r, "id")
	topupID, err := uuid.Parse(topupIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid topup ID"))
		return
	}

	// Verify organization ownership first
	_, err = h.topupService.GetByOrganization(ctx, topupID, orgID)
	if err != nil {
		if errors.Is(err, billing.ErrTopupNotFound) {
			WriteError(w, r, NewNotFoundError("Topup"))
			return
		}
		h.logger.Error("Failed to get topup", "error", err, "topup_id", topupID)
		WriteError(w, r, NewInternalError("Failed to get topup"))
		return
	}

	// Cancel topup
	topup, err := h.topupService.Cancel(ctx, topupID)
	if err != nil {
		if errors.Is(err, billing.ErrTopupNotFound) {
			WriteError(w, r, NewNotFoundError("Topup"))
			return
		}
		if errors.Is(err, billing.ErrTopupInvalidState) {
			WriteError(w, r, NewBadRequestError("Topup cannot be cancelled in current state"))
			return
		}
		h.logger.Error("Failed to cancel topup", "error", err, "topup_id", topupID)
		WriteError(w, r, NewInternalError("Failed to cancel topup"))
		return
	}

	writeJSON(w, http.StatusOK, topupToResponse(topup))
}

// Topup Request/Response Types

// CreateTopupRequest represents a request to create a topup.
type CreateTopupRequest struct {
	AmountKurus int64  `json:"amount_kurus"`
	OrgName     string `json:"org_name,omitempty"`
	OrgEmail    string `json:"org_email,omitempty"`
	OrgPhone    string `json:"org_phone,omitempty"`
	OrgAddress  string `json:"org_address,omitempty"`
	OrgCity     string `json:"org_city,omitempty"`
}

// CreateTopupResponse represents the response from creating a topup.
type CreateTopupResponse struct {
	Topup      TopupResponse `json:"topup"`
	PaymentURL string        `json:"payment_url"`
}

// TopupResponse represents a topup in API responses.
type TopupResponse struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	AmountKurus     int64      `json:"amount_kurus"`
	Currency        string     `json:"currency"`
	Status          string     `json:"status"`
	PaymentProvider string     `json:"payment_provider"`
	PaymentURL      *string    `json:"payment_url,omitempty"`
	LedgerEntryID   *uuid.UUID `json:"ledger_entry_id,omitempty"`
	FailureReason   *string    `json:"failure_reason,omitempty"`
	CompletedAt     *string    `json:"completed_at,omitempty"`
	ExpiresAt       string     `json:"expires_at"`
	CreatedAt       string     `json:"created_at"`
	UpdatedAt       string     `json:"updated_at"`
}

// Conversion helper for topups
func topupToResponse(topup *billing.Topup) TopupResponse {
	resp := TopupResponse{
		ID:              topup.ID,
		OrganizationID:  topup.OrganizationID,
		AmountKurus:     topup.AmountKurus,
		Currency:        topup.Currency,
		Status:          topup.Status,
		PaymentProvider: topup.PaymentProvider,
		PaymentURL:      topup.PaymentURL,
		LedgerEntryID:   topup.LedgerEntryID,
		FailureReason:   topup.FailureReason,
		ExpiresAt:       topup.ExpiresAt.Format(time.RFC3339),
		CreatedAt:       topup.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       topup.UpdatedAt.Format(time.RFC3339),
	}

	if topup.CompletedAt != nil {
		completedAt := topup.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &completedAt
	}

	return resp
}

// Refund Endpoints (US7: Shipment Cancellation & Refund)

// ListRefunds handles GET /billing/refunds
// Returns refunds for the organization.
func (h *BillingHandler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if refund service is available
	if h.refundService == nil {
		WriteError(w, r, NewInternalError("Refund service not available"))
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

	var referenceType *string
	if rt := r.URL.Query().Get("reference_type"); rt != "" {
		referenceType = &rt
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

	// Get refunds
	result, err := h.refundService.List(ctx, &billing.ListRefundsRequest{
		OrganizationID: orgID,
		Status:         status,
		ReferenceType:  referenceType,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list refunds", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list refunds"))
		return
	}

	response := make([]RefundResponse, len(result.Refunds))
	for i, refund := range result.Refunds {
		response[i] = refundToResponse(refund)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  result.Total,
		Offset: offset,
		Limit:  limit,
	})
}

// GetRefund handles GET /billing/refunds/{id}
// Returns a specific refund.
func (h *BillingHandler) GetRefund(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if refund service is available
	if h.refundService == nil {
		WriteError(w, r, NewInternalError("Refund service not available"))
		return
	}

	// Get refund ID from URL
	refundIDStr := chi.URLParam(r, "id")
	refundID, err := uuid.Parse(refundIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid refund ID"))
		return
	}

	// Get refund
	refund, err := h.refundService.Get(ctx, refundID, orgID)
	if err != nil {
		if errors.Is(err, billing.ErrRefundNotFound) {
			WriteError(w, r, NewNotFoundError("Refund"))
			return
		}
		h.logger.Error("Failed to get refund", "error", err, "refund_id", refundID)
		WriteError(w, r, NewInternalError("Failed to get refund"))
		return
	}

	writeJSON(w, http.StatusOK, refundToResponse(refund))
}

// GetRefundByShipment handles GET /billing/refunds/shipment/{shipment_id}
// Returns the refund for a specific shipment.
func (h *BillingHandler) GetRefundByShipment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if refund service is available
	if h.refundService == nil {
		WriteError(w, r, NewInternalError("Refund service not available"))
		return
	}

	// Get shipment ID from URL
	shipmentIDStr := chi.URLParam(r, "shipment_id")
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid shipment ID"))
		return
	}

	// Get refund by shipment
	refund, err := h.refundService.GetByShipmentID(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, billing.ErrRefundNotFound) {
			WriteError(w, r, NewNotFoundError("Refund"))
			return
		}
		h.logger.Error("Failed to get refund by shipment", "error", err, "shipment_id", shipmentID)
		WriteError(w, r, NewInternalError("Failed to get refund"))
		return
	}

	// Verify organization ownership
	if refund.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Refund"))
		return
	}

	writeJSON(w, http.StatusOK, refundToResponse(refund))
}

// GetRefundSummary handles GET /billing/refunds/summary
// Returns a summary of refunds for the organization.
func (h *BillingHandler) GetRefundSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Check if refund service is available
	if h.refundService == nil {
		WriteError(w, r, NewInternalError("Refund service not available"))
		return
	}

	// Get summary
	summary, err := h.refundService.GetSummary(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to get refund summary", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get refund summary"))
		return
	}

	writeJSON(w, http.StatusOK, RefundSummaryResponse{
		OrganizationID:   summary.OrganizationID,
		TotalRefunds:     summary.TotalRefunds,
		TotalAmountKurus: summary.TotalAmountKurus,
		PendingCount:     summary.PendingCount,
		CompletedCount:   summary.CompletedCount,
	})
}

// Refund Response Types

// RefundResponse represents a refund in API responses.
type RefundResponse struct {
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	OriginalLedgerEntryID *uuid.UUID `json:"original_ledger_entry_id,omitempty"`
	RefundLedgerEntryID   *uuid.UUID `json:"refund_ledger_entry_id,omitempty"`
	AmountKurus           int64      `json:"amount_kurus"`
	Reason                string     `json:"reason"`
	Status                string     `json:"status"`
	ReferenceType         string     `json:"reference_type"`
	ReferenceID           uuid.UUID  `json:"reference_id"`
	ApprovedBy            *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt            *string    `json:"approved_at,omitempty"`
	CompletedAt           *string    `json:"completed_at,omitempty"`
	CreatedAt             string     `json:"created_at"`
	UpdatedAt             string     `json:"updated_at"`
}

// RefundSummaryResponse represents a refund summary in API responses.
type RefundSummaryResponse struct {
	OrganizationID   uuid.UUID `json:"organization_id"`
	TotalRefunds     int64     `json:"total_refunds"`
	TotalAmountKurus int64     `json:"total_amount_kurus"`
	PendingCount     int64     `json:"pending_count"`
	CompletedCount   int64     `json:"completed_count"`
}

// Conversion helper for refunds
func refundToResponse(refund *billing.Refund) RefundResponse {
	resp := RefundResponse{
		ID:                  refund.ID,
		OrganizationID:      refund.OrganizationID,
		RefundLedgerEntryID: refund.RefundLedgerEntryID,
		AmountKurus:         refund.AmountKurus,
		Reason:              refund.Reason,
		Status:              refund.Status,
		ReferenceType:       refund.ReferenceType,
		ReferenceID:         refund.ReferenceID,
		ApprovedBy:          refund.ApprovedBy,
		CreatedAt:           refund.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           refund.UpdatedAt.Format(time.RFC3339),
	}

	if refund.OriginalLedgerEntryID != uuid.Nil {
		resp.OriginalLedgerEntryID = &refund.OriginalLedgerEntryID
	}

	if refund.ApprovedAt != nil {
		approvedAt := refund.ApprovedAt.Format(time.RFC3339)
		resp.ApprovedAt = &approvedAt
	}

	if refund.CompletedAt != nil {
		completedAt := refund.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &completedAt
	}

	return resp
}
