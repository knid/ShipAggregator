// Package billing provides wallet and ledger management services.
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// Common errors for wallet operations.
var (
	ErrInsufficientBalance = errors.New("insufficient wallet balance")
	ErrInvalidAmount       = errors.New("invalid amount: must be positive")
	ErrWalletNotFound      = errors.New("wallet not found")
)

// WalletService provides wallet balance operations.
type WalletService struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewWalletService creates a new wallet service.
func NewWalletService(queries *db.Queries, logger *observability.Logger) *WalletService {
	return &WalletService{
		queries: queries,
		logger:  logger,
	}
}

// WalletBalance represents the current wallet balance for an organization.
type WalletBalance struct {
	OrganizationID    uuid.UUID  `json:"organization_id"`
	BalanceKurus      int64      `json:"balance_kurus"`
	AvailableKurus    int64      `json:"available_kurus"`
	PendingKurus      int64      `json:"pending_kurus"`
	TotalCredits      int64      `json:"total_credits"`
	TotalDebits       int64      `json:"total_debits"`
	Currency          string     `json:"currency"`
	LastTransactionAt *time.Time `json:"last_transaction_at,omitempty"`
	AsOf              time.Time  `json:"as_of"`
}

// GetBalance retrieves the current wallet balance for an organization.
func (s *WalletService) GetBalance(ctx context.Context, organizationID uuid.UUID) (*WalletBalance, error) {
	// Get balance from ledger entries
	balanceResult, err := s.queries.GetWalletBalance(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	// Convert interface{} to int64
	balanceKurus := interfaceToInt64(balanceResult)

	// Get summary for totals
	summary, err := s.queries.GetLedgerSummaryByOrganization(ctx, organizationID)
	var totalCredits, totalDebits int64
	if err == nil {
		totalCredits = interfaceToInt64(summary.TotalCreditsKurus)
		totalDebits = interfaceToInt64(summary.TotalDebitsKurus)
	}

	// Get last transaction timestamp
	var lastTransactionAt *time.Time
	latestEntry, err := s.queries.GetLatestLedgerEntry(ctx, organizationID)
	if err == nil {
		lastTransactionAt = &latestEntry.CreatedAt.Time
	} else if !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error("Failed to get latest ledger entry", "error", err, "organization_id", organizationID)
	}

	return &WalletBalance{
		OrganizationID:    organizationID,
		BalanceKurus:      balanceKurus,
		AvailableKurus:    balanceKurus, // For now, available = balance (no holds)
		PendingKurus:      0,            // No pending transactions yet
		TotalCredits:      totalCredits,
		TotalDebits:       totalDebits,
		Currency:          "TRY",
		LastTransactionAt: lastTransactionAt,
		AsOf:              time.Now(),
	}, nil
}

// GetBalanceWithLock retrieves the current wallet balance with a row lock for updates.
// This should be used within a transaction when performing debit/credit operations.
func (s *WalletService) GetBalanceWithLock(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	balanceResult, err := s.queries.GetWalletBalanceWithLock(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	return interfaceToInt64(balanceResult), nil
}

// HasSufficientBalance checks if the organization has sufficient balance for a debit.
func (s *WalletService) HasSufficientBalance(ctx context.Context, organizationID uuid.UUID, amountKurus int64) (bool, error) {
	if amountKurus <= 0 {
		return false, ErrInvalidAmount
	}

	balance, err := s.GetBalance(ctx, organizationID)
	if err != nil {
		return false, err
	}

	return balance.BalanceKurus >= amountKurus, nil
}

// GetSummary retrieves a summary of ledger entries for an organization.
func (s *WalletService) GetSummary(ctx context.Context, organizationID uuid.UUID) (*domain.LedgerSummary, error) {
	summary, err := s.queries.GetLedgerSummaryByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	// Get current balance
	balance, err := s.GetBalance(ctx, organizationID)
	if err != nil {
		return nil, err
	}

	totalCredits := interfaceToInt64(summary.TotalCreditsKurus)
	totalDebits := interfaceToInt64(summary.TotalDebitsKurus)

	return &domain.LedgerSummary{
		OrganizationID:      organizationID,
		TotalCreditsKurus:   totalCredits,
		TotalDebitsKurus:    totalDebits,
		NetChangeKurus:      totalCredits - totalDebits,
		ClosingBalanceKurus: balance.BalanceKurus,
		TransactionCount:    int(summary.TotalEntries),
	}, nil
}

// ListAllBalances retrieves all organization balances (admin function).
func (s *WalletService) ListAllBalances(ctx context.Context) ([]*WalletBalance, error) {
	rows, err := s.queries.ListAllOrganizationBalances(ctx)
	if err != nil {
		return nil, err
	}

	balances := make([]*WalletBalance, 0, len(rows))
	for _, row := range rows {
		var lastTxAt *time.Time
		if row.LastTransactionAt != nil {
			if t, ok := row.LastTransactionAt.(time.Time); ok {
				lastTxAt = &t
			}
		}

		balances = append(balances, &WalletBalance{
			OrganizationID:    row.OrganizationID,
			BalanceKurus:      int64(row.BalanceKurus),
			Currency:          "TRY",
			LastTransactionAt: lastTxAt,
		})
	}

	return balances, nil
}

// VerifyInvariant checks that the ledger invariant holds for an organization.
// The invariant is: sum(credits) - sum(debits) == latest balance_after_kurus
func (s *WalletService) VerifyInvariant(ctx context.Context, organizationID uuid.UUID) (bool, error) {
	result, err := s.queries.CheckLedgerInvariant(ctx, organizationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No entries yet, invariant holds trivially
			return true, nil
		}
		return false, err
	}

	// If there are no entries, the invariant holds
	if result.StoredBalance == 0 && result.ComputedBalance == 0 {
		return true, nil
	}

	return result.ComputedBalance == result.StoredBalance, nil
}

// Helper function to convert interface{} to int64
func interfaceToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int32:
		return int64(val)
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}
