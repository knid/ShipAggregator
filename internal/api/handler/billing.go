// Package handler provides HTTP handlers for the API.
package handler

import (
	"net/http"
	"strconv"
	"time"

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
