// Package billing provides wallet and ledger management services.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// Common errors for ledger operations.
var (
	ErrLedgerEntryNotFound = errors.New("ledger entry not found")
	ErrInvariantViolation  = errors.New("ledger invariant violation detected")
	ErrDuplicateReference  = errors.New("duplicate reference: entry already exists")
)

// LedgerService provides ledger entry operations.
type LedgerService struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewLedgerService creates a new ledger service.
func NewLedgerService(queries *db.Queries, logger *observability.Logger) *LedgerService {
	return &LedgerService{
		queries: queries,
		logger:  logger,
	}
}

// CreditRequest represents a request to credit an organization's wallet.
type CreditRequest struct {
	OrganizationID uuid.UUID
	AmountKurus    int64
	ReferenceType  domain.ReferenceType
	ReferenceID    uuid.UUID
	Description    string
	Metadata       map[string]interface{}
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
}

// DebitRequest represents a request to debit an organization's wallet.
type DebitRequest struct {
	OrganizationID uuid.UUID
	AmountKurus    int64
	ReferenceType  domain.ReferenceType
	ReferenceID    uuid.UUID
	Description    string
	Metadata       map[string]interface{}
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
}

// LedgerEntry represents a ledger entry result.
type LedgerEntry struct {
	ID                uuid.UUID              `json:"id"`
	OrganizationID    uuid.UUID              `json:"organization_id"`
	EntryType         string                 `json:"entry_type"`
	AmountKurus       int64                  `json:"amount_kurus"`
	Currency          string                 `json:"currency"`
	BalanceAfterKurus int64                  `json:"balance_after_kurus"`
	ReferenceType     string                 `json:"reference_type"`
	ReferenceID       uuid.UUID              `json:"reference_id"`
	Description       string                 `json:"description"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	ActorID           *uuid.UUID             `json:"actor_id,omitempty"`
	ActorType         string                 `json:"actor_type"`
	CreatedAt         time.Time              `json:"created_at"`
}

// Credit adds funds to an organization's wallet.
func (s *LedgerService) Credit(ctx context.Context, req *CreditRequest) (*LedgerEntry, error) {
	if req.AmountKurus <= 0 {
		return nil, ErrInvalidAmount
	}

	// Get current balance (with lock for consistency).
	// The lock must be acquired BEFORE the duplicate check to prevent a TOCTOU race
	// where two concurrent requests both pass the duplicate check before either inserts.
	currentBalance, err := s.queries.GetWalletBalanceWithLock(ctx, req.OrganizationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get current balance: %w", err)
	}
	balanceKurus := interfaceToInt64(currentBalance)

	// Check for duplicate reference (now protected by the wallet row lock)
	_, err = s.queries.GetLedgerEntryByReference(ctx, db.GetLedgerEntryByReferenceParams{
		ReferenceType: req.ReferenceType.String(),
		ReferenceID:   req.ReferenceID,
	})
	if err == nil {
		return nil, ErrDuplicateReference
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check duplicate reference: %w", err)
	}

	// Calculate new balance
	newBalance := balanceKurus + req.AmountKurus

	// Serialize metadata
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// Create actor ID pgtype
	var actorID pgtype.UUID
	if req.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *req.ActorID, Valid: true}
	}

	// Create ledger entry
	entry, err := s.queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		OrganizationID:    req.OrganizationID,
		EntryType:         domain.EntryTypeCredit.String(),
		AmountKurus:       int32(req.AmountKurus),
		Currency:          "TRY",
		BalanceAfterKurus: int32(newBalance),
		ReferenceType:     req.ReferenceType.String(),
		ReferenceID:       req.ReferenceID,
		Description:       req.Description,
		Metadata:          metadataJSON,
		ActorID:           actorID,
		ActorType:         req.ActorType.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create credit entry: %w", err)
	}

	s.logger.Info("Credit entry created",
		"entry_id", entry.ID,
		"organization_id", req.OrganizationID,
		"amount_kurus", req.AmountKurus,
		"new_balance", newBalance,
		"reference_type", req.ReferenceType,
		"reference_id", req.ReferenceID,
	)

	return dbEntryToLedgerEntry(entry), nil
}

// Debit removes funds from an organization's wallet.
func (s *LedgerService) Debit(ctx context.Context, req *DebitRequest) (*LedgerEntry, error) {
	if req.AmountKurus <= 0 {
		return nil, ErrInvalidAmount
	}

	// Get current balance (with lock for consistency).
	// The lock must be acquired BEFORE the duplicate check to prevent a TOCTOU race
	// where two concurrent requests both pass the duplicate check before either inserts.
	currentBalance, err := s.queries.GetWalletBalanceWithLock(ctx, req.OrganizationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get current balance: %w", err)
	}
	balanceKurus := interfaceToInt64(currentBalance)

	// Check for duplicate reference (now protected by the wallet row lock)
	_, err = s.queries.GetLedgerEntryByReference(ctx, db.GetLedgerEntryByReferenceParams{
		ReferenceType: req.ReferenceType.String(),
		ReferenceID:   req.ReferenceID,
	})
	if err == nil {
		return nil, ErrDuplicateReference
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check duplicate reference: %w", err)
	}

	// Check sufficient balance
	if balanceKurus < req.AmountKurus {
		return nil, ErrInsufficientBalance
	}

	// Calculate new balance
	newBalance := balanceKurus - req.AmountKurus

	// Serialize metadata
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// Create actor ID pgtype
	var actorID pgtype.UUID
	if req.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *req.ActorID, Valid: true}
	}

	// Create ledger entry
	entry, err := s.queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		OrganizationID:    req.OrganizationID,
		EntryType:         domain.EntryTypeDebit.String(),
		AmountKurus:       int32(req.AmountKurus),
		Currency:          "TRY",
		BalanceAfterKurus: int32(newBalance),
		ReferenceType:     req.ReferenceType.String(),
		ReferenceID:       req.ReferenceID,
		Description:       req.Description,
		Metadata:          metadataJSON,
		ActorID:           actorID,
		ActorType:         req.ActorType.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create debit entry: %w", err)
	}

	s.logger.Info("Debit entry created",
		"entry_id", entry.ID,
		"organization_id", req.OrganizationID,
		"amount_kurus", req.AmountKurus,
		"new_balance", newBalance,
		"reference_type", req.ReferenceType,
		"reference_id", req.ReferenceID,
	)

	return dbEntryToLedgerEntry(entry), nil
}

// GetEntry retrieves a ledger entry by ID.
func (s *LedgerService) GetEntry(ctx context.Context, entryID uuid.UUID) (*LedgerEntry, error) {
	entry, err := s.queries.GetLedgerEntryByID(ctx, entryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLedgerEntryNotFound
		}
		return nil, err
	}
	return dbEntryToLedgerEntry(entry), nil
}

// GetEntryByReference retrieves a ledger entry by reference type and ID.
func (s *LedgerService) GetEntryByReference(ctx context.Context, refType domain.ReferenceType, refID uuid.UUID) (*LedgerEntry, error) {
	entry, err := s.queries.GetLedgerEntryByReference(ctx, db.GetLedgerEntryByReferenceParams{
		ReferenceType: refType.String(),
		ReferenceID:   refID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLedgerEntryNotFound
		}
		return nil, err
	}
	return dbEntryToLedgerEntry(entry), nil
}

// ListEntriesRequest represents a request to list ledger entries.
type ListEntriesRequest struct {
	OrganizationID uuid.UUID
	EntryType      *string
	ReferenceType  *string
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int
	Limit          int
}

// ListEntriesResult represents the result of listing ledger entries.
type ListEntriesResult struct {
	Entries []*LedgerEntry
	Total   int64
}

// ListEntries lists ledger entries for an organization with filtering.
func (s *LedgerService) ListEntries(ctx context.Context, req *ListEntriesRequest) (*ListEntriesResult, error) {
	// Build query params
	params := db.ListLedgerEntriesByOrganizationParams{
		OrganizationID: req.OrganizationID,
		EntryType:      req.EntryType,
		ReferenceType:  req.ReferenceType,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	}

	if req.CreatedAfter != nil {
		params.CreatedAfter = pgtype.Timestamptz{Time: *req.CreatedAfter, Valid: true}
	}
	if req.CreatedBefore != nil {
		params.CreatedBefore = pgtype.Timestamptz{Time: *req.CreatedBefore, Valid: true}
	}

	// Get count
	countParams := db.CountLedgerEntriesByOrganizationParams{
		OrganizationID: req.OrganizationID,
		EntryType:      req.EntryType,
		ReferenceType:  req.ReferenceType,
		CreatedAfter:   params.CreatedAfter,
		CreatedBefore:  params.CreatedBefore,
	}
	total, err := s.queries.CountLedgerEntriesByOrganization(ctx, countParams)
	if err != nil {
		return nil, fmt.Errorf("failed to count entries: %w", err)
	}

	// Get entries
	entries, err := s.queries.ListLedgerEntriesByOrganization(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries: %w", err)
	}

	result := &ListEntriesResult{
		Entries: make([]*LedgerEntry, 0, len(entries)),
		Total:   total,
	}

	for _, entry := range entries {
		result.Entries = append(result.Entries, dbEntryToLedgerEntry(entry))
	}

	return result, nil
}

// ListEntriesByReferenceType lists ledger entries by reference type.
func (s *LedgerService) ListEntriesByReferenceType(ctx context.Context, orgID uuid.UUID, refType domain.ReferenceType, offset, limit int) ([]*LedgerEntry, error) {
	entries, err := s.queries.ListLedgerEntriesByReferenceType(ctx, db.ListLedgerEntriesByReferenceTypeParams{
		OrganizationID: orgID,
		ReferenceType:  refType.String(),
		OffsetCount:    int32(offset),
		LimitCount:     int32(limit),
	})
	if err != nil {
		return nil, err
	}

	result := make([]*LedgerEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, dbEntryToLedgerEntry(entry))
	}

	return result, nil
}

// GetSummaryByDateRange retrieves a summary of ledger entries for a date range.
func (s *LedgerService) GetSummaryByDateRange(ctx context.Context, orgID uuid.UUID, start, end time.Time) (*domain.LedgerSummary, error) {
	summary, err := s.queries.GetLedgerSummaryByDateRange(ctx, db.GetLedgerSummaryByDateRangeParams{
		OrganizationID: orgID,
		CreatedAt:      pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2:    pgtype.Timestamptz{Time: end, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	totalCredits := interfaceToInt64(summary.TotalCreditsKurus)
	totalDebits := interfaceToInt64(summary.TotalDebitsKurus)

	return &domain.LedgerSummary{
		OrganizationID:    orgID,
		PeriodStart:       start,
		PeriodEnd:         end,
		TotalCreditsKurus: totalCredits,
		TotalDebitsKurus:  totalDebits,
		NetChangeKurus:    totalCredits - totalDebits,
		TransactionCount:  int(summary.TotalEntries),
	}, nil
}

// VerifyInvariant checks that the ledger invariant holds for an organization.
func (s *LedgerService) VerifyInvariant(ctx context.Context, organizationID uuid.UUID) (bool, error) {
	result, err := s.queries.CheckLedgerInvariant(ctx, organizationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}

	if result.StoredBalance == 0 && result.ComputedBalance == 0 {
		return true, nil
	}

	isValid := result.ComputedBalance == result.StoredBalance
	if !isValid {
		s.logger.Error("Ledger invariant violation detected",
			"organization_id", organizationID,
			"computed_balance", result.ComputedBalance,
			"stored_balance", result.StoredBalance,
		)
	}

	return isValid, nil
}

// ListLedgerRequest represents a request to list ledger entries (for handler compatibility).
type ListLedgerRequest struct {
	OrganizationID uuid.UUID
	EntryType      *string
	ReferenceType  *domain.ReferenceType
	ReferenceID    *uuid.UUID
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int
	Limit          int
}

// ListLedgerResult represents the result of listing ledger entries.
type ListLedgerResult struct {
	Entries []*LedgerEntry
	Total   int64
}

// List lists ledger entries for an organization with filtering.
func (s *LedgerService) List(ctx context.Context, req *ListLedgerRequest) (*ListLedgerResult, error) {
	// Convert to internal request format
	var entryType *string
	if req.EntryType != nil {
		entryType = req.EntryType
	}

	var refType *string
	if req.ReferenceType != nil {
		rt := req.ReferenceType.String()
		refType = &rt
	}

	internalReq := &ListEntriesRequest{
		OrganizationID: req.OrganizationID,
		EntryType:      entryType,
		ReferenceType:  refType,
		CreatedAfter:   req.CreatedAfter,
		CreatedBefore:  req.CreatedBefore,
		Offset:         req.Offset,
		Limit:          req.Limit,
	}

	result, err := s.ListEntries(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return &ListLedgerResult{
		Entries: result.Entries,
		Total:   result.Total,
	}, nil
}

// Get retrieves a ledger entry by ID with organization verification.
func (s *LedgerService) Get(ctx context.Context, entryID, organizationID uuid.UUID) (*LedgerEntry, error) {
	entry, err := s.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}

	// Verify organization ownership
	if entry.OrganizationID != organizationID {
		return nil, ErrLedgerEntryNotFound
	}

	return entry, nil
}

// LedgerSummary represents a summary of ledger activity.
type LedgerSummary struct {
	TotalCredits     int64            `json:"total_credits"`
	TotalDebits      int64            `json:"total_debits"`
	NetChange        int64            `json:"net_change"`
	TransactionCount int64            `json:"transaction_count"`
	ByType           map[string]int64 `json:"by_type"`
	ByReferenceType  map[string]int64 `json:"by_reference_type"`
}

// GetSummary retrieves a summary of ledger activity for a date range.
func (s *LedgerService) GetSummary(ctx context.Context, organizationID uuid.UUID, startDate, endDate time.Time) (*LedgerSummary, error) {
	// Get basic summary
	summary, err := s.queries.GetLedgerSummaryByDateRange(ctx, db.GetLedgerSummaryByDateRangeParams{
		OrganizationID: organizationID,
		CreatedAt:      pgtype.Timestamptz{Time: startDate, Valid: true},
		CreatedAt_2:    pgtype.Timestamptz{Time: endDate, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	totalCredits := interfaceToInt64(summary.TotalCreditsKurus)
	totalDebits := interfaceToInt64(summary.TotalDebitsKurus)

	// Get breakdown by type
	byType := make(map[string]int64)
	byType["credit"] = totalCredits
	byType["debit"] = totalDebits

	// Get breakdown by reference type (simplified - would need additional query for full breakdown)
	byReferenceType := make(map[string]int64)

	return &LedgerSummary{
		TotalCredits:     totalCredits,
		TotalDebits:      totalDebits,
		NetChange:        totalCredits - totalDebits,
		TransactionCount: summary.TotalEntries,
		ByType:           byType,
		ByReferenceType:  byReferenceType,
	}, nil
}

// Helper function to convert db.LedgerEntry to LedgerEntry
func dbEntryToLedgerEntry(entry *db.LedgerEntry) *LedgerEntry {
	result := &LedgerEntry{
		ID:                entry.ID,
		OrganizationID:    entry.OrganizationID,
		EntryType:         entry.EntryType,
		AmountKurus:       int64(entry.AmountKurus),
		Currency:          entry.Currency,
		BalanceAfterKurus: int64(entry.BalanceAfterKurus),
		ReferenceType:     entry.ReferenceType,
		ReferenceID:       entry.ReferenceID,
		Description:       entry.Description,
		ActorType:         entry.ActorType,
		CreatedAt:         entry.CreatedAt.Time,
	}

	// Parse metadata
	if len(entry.Metadata) > 0 {
		var metadata map[string]interface{}
		if err := json.Unmarshal(entry.Metadata, &metadata); err == nil {
			result.Metadata = metadata
		}
	}

	// Parse actor ID
	if entry.ActorID.Valid {
		actorID := uuid.UUID(entry.ActorID.Bytes)
		result.ActorID = &actorID
	}

	return result
}
