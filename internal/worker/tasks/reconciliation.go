package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// ReconciliationHandler handles daily financial reconciliation tasks.
// It verifies that ledger entries match expected values and identifies discrepancies.
type ReconciliationHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewReconciliationHandler creates a new reconciliation handler.
func NewReconciliationHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *ReconciliationHandler {
	return &ReconciliationHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles a reconciliation task.
func (h *ReconciliationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseReconciliationPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse reconciliation payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting reconciliation",
		"date", payload.Date,
		"full_check", payload.FullCheck,
	)

	// Parse the date
	date, err := time.Parse("2006-01-02", payload.Date)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	// Run reconciliation checks
	report := &ReconciliationReport{
		Date:      payload.Date,
		StartedAt: time.Now(),
	}

	// 1. Verify wallet balances match ledger entries
	if err := h.reconcileWalletBalances(ctx, date, report); err != nil {
		h.logger.Error("Wallet balance reconciliation failed", "error", err)
		report.Errors = append(report.Errors, fmt.Sprintf("wallet_balances: %v", err))
	}

	// 2. Verify shipment charges match ledger debits
	if err := h.reconcileShipmentCharges(ctx, date, report); err != nil {
		h.logger.Error("Shipment charge reconciliation failed", "error", err)
		report.Errors = append(report.Errors, fmt.Sprintf("shipment_charges: %v", err))
	}

	// 3. Verify topup credits match ledger credits
	if err := h.reconcileTopupCredits(ctx, date, report); err != nil {
		h.logger.Error("Topup credit reconciliation failed", "error", err)
		report.Errors = append(report.Errors, fmt.Sprintf("topup_credits: %v", err))
	}

	// 4. Verify COD settlements
	if err := h.reconcileCODSettlements(ctx, date, report); err != nil {
		h.logger.Error("COD settlement reconciliation failed", "error", err)
		report.Errors = append(report.Errors, fmt.Sprintf("cod_settlements: %v", err))
	}

	// 5. Full check: verify all historical balances (if requested)
	if payload.FullCheck {
		if err := h.reconcileHistoricalBalances(ctx, report); err != nil {
			h.logger.Error("Historical balance reconciliation failed", "error", err)
			report.Errors = append(report.Errors, fmt.Sprintf("historical_balances: %v", err))
		}
	}

	report.CompletedAt = time.Now()
	report.Duration = report.CompletedAt.Sub(report.StartedAt)

	// Log the report
	h.logReconciliationReport(report)

	// If there are discrepancies, we don't fail the task but log warnings
	if len(report.Discrepancies) > 0 {
		h.logger.Warn("Reconciliation completed with discrepancies",
			"discrepancy_count", len(report.Discrepancies),
			"date", payload.Date,
		)
	}

	h.logger.Info("Completed reconciliation",
		"date", payload.Date,
		"duration", report.Duration,
		"discrepancies", len(report.Discrepancies),
		"errors", len(report.Errors),
	)

	return nil
}

// ReconciliationReport holds the results of a reconciliation run.
type ReconciliationReport struct {
	Date          string
	StartedAt     time.Time
	CompletedAt   time.Time
	Duration      time.Duration
	Discrepancies []Discrepancy
	Errors        []string
}

// Discrepancy represents a reconciliation discrepancy.
type Discrepancy struct {
	Type           string
	OrganizationID string
	Expected       decimal.Decimal
	Actual         decimal.Decimal
	Difference     decimal.Decimal
	Details        string
}

// reconcileWalletBalances verifies that computed wallet balances match ledger sums.
func (h *ReconciliationHandler) reconcileWalletBalances(ctx context.Context, date time.Time, report *ReconciliationReport) error {
	// Get all organizations with ledger entries
	rows, err := h.pool.Query(ctx, `
		SELECT 
			organization_id,
			SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE -amount_kurus END) as computed_balance
		FROM ledger_entries
		WHERE created_at <= $1::date + INTERVAL '1 day'
		GROUP BY organization_id
	`, date)
	if err != nil {
		return fmt.Errorf("failed to query ledger balances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		var computedBalance int64
		if err := rows.Scan(&orgID, &computedBalance); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Get the stored balance from the view
		var storedBalance int64
		err := h.pool.QueryRow(ctx, `
			SELECT COALESCE(balance_kurus, 0) 
			FROM v_wallet_balances 
			WHERE organization_id = $1
		`, orgID).Scan(&storedBalance)
		if err != nil {
			h.logger.Warn("Failed to get stored balance",
				"organization_id", orgID,
				"error", err,
			)
			continue
		}

		if computedBalance != storedBalance {
			report.Discrepancies = append(report.Discrepancies, Discrepancy{
				Type:           "wallet_balance",
				OrganizationID: orgID,
				Expected:       decimal.NewFromInt(computedBalance),
				Actual:         decimal.NewFromInt(storedBalance),
				Difference:     decimal.NewFromInt(computedBalance - storedBalance),
				Details:        "Computed balance from ledger entries does not match stored balance",
			})
		}
	}

	return rows.Err()
}

// reconcileShipmentCharges verifies that shipment charges match ledger debits.
func (h *ReconciliationHandler) reconcileShipmentCharges(ctx context.Context, date time.Time, report *ReconciliationReport) error {
	// Get shipment charges for the date
	rows, err := h.pool.Query(ctx, `
		SELECT 
			s.organization_id,
			SUM(s.price_charged_kurus) as total_charged
		FROM shipments s
		WHERE s.created_at >= $1::date 
		  AND s.created_at < $1::date + INTERVAL '1 day'
		  AND s.price_charged_kurus IS NOT NULL
		GROUP BY s.organization_id
	`, date)
	if err != nil {
		return fmt.Errorf("failed to query shipment charges: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		var totalCharged int64
		if err := rows.Scan(&orgID, &totalCharged); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Get ledger debits for shipments on the same date
		var ledgerDebits int64
		err := h.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_kurus), 0)
			FROM ledger_entries
			WHERE organization_id = $1
			  AND entry_type = 'debit'
			  AND reference_type = 'shipment'
			  AND created_at >= $2::date
			  AND created_at < $2::date + INTERVAL '1 day'
		`, orgID, date).Scan(&ledgerDebits)
		if err != nil {
			h.logger.Warn("Failed to get ledger debits",
				"organization_id", orgID,
				"error", err,
			)
			continue
		}

		if totalCharged != ledgerDebits {
			report.Discrepancies = append(report.Discrepancies, Discrepancy{
				Type:           "shipment_charges",
				OrganizationID: orgID,
				Expected:       decimal.NewFromInt(totalCharged),
				Actual:         decimal.NewFromInt(ledgerDebits),
				Difference:     decimal.NewFromInt(totalCharged - ledgerDebits),
				Details:        fmt.Sprintf("Shipment charges for %s do not match ledger debits", date.Format("2006-01-02")),
			})
		}
	}

	return rows.Err()
}

// reconcileTopupCredits verifies that completed topups match ledger credits.
func (h *ReconciliationHandler) reconcileTopupCredits(ctx context.Context, date time.Time, report *ReconciliationReport) error {
	// Get completed topups for the date
	rows, err := h.pool.Query(ctx, `
		SELECT 
			organization_id,
			SUM(amount_kurus) as total_topups
		FROM topups
		WHERE status = 'completed'
		  AND completed_at >= $1::date
		  AND completed_at < $1::date + INTERVAL '1 day'
		GROUP BY organization_id
	`, date)
	if err != nil {
		return fmt.Errorf("failed to query topups: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		var totalTopups int64
		if err := rows.Scan(&orgID, &totalTopups); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Get ledger credits for topups on the same date
		var ledgerCredits int64
		err := h.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_kurus), 0)
			FROM ledger_entries
			WHERE organization_id = $1
			  AND entry_type = 'credit'
			  AND reference_type = 'topup'
			  AND created_at >= $2::date
			  AND created_at < $2::date + INTERVAL '1 day'
		`, orgID, date).Scan(&ledgerCredits)
		if err != nil {
			h.logger.Warn("Failed to get ledger credits",
				"organization_id", orgID,
				"error", err,
			)
			continue
		}

		if totalTopups != ledgerCredits {
			report.Discrepancies = append(report.Discrepancies, Discrepancy{
				Type:           "topup_credits",
				OrganizationID: orgID,
				Expected:       decimal.NewFromInt(totalTopups),
				Actual:         decimal.NewFromInt(ledgerCredits),
				Difference:     decimal.NewFromInt(totalTopups - ledgerCredits),
				Details:        fmt.Sprintf("Topup credits for %s do not match ledger credits", date.Format("2006-01-02")),
			})
		}
	}

	return rows.Err()
}

// reconcileCODSettlements verifies that COD settlements match ledger entries.
func (h *ReconciliationHandler) reconcileCODSettlements(ctx context.Context, date time.Time, report *ReconciliationReport) error {
	// Get settled COD amounts for the date
	rows, err := h.pool.Query(ctx, `
		SELECT 
			organization_id,
			SUM(actual_amount_kurus) as total_settled
		FROM cod_settlements
		WHERE status = 'settled'
		  AND settled_at >= $1::date
		  AND settled_at < $1::date + INTERVAL '1 day'
		GROUP BY organization_id
	`, date)
	if err != nil {
		return fmt.Errorf("failed to query COD settlements: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		var totalSettled int64
		if err := rows.Scan(&orgID, &totalSettled); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Get ledger credits for COD settlements on the same date
		var ledgerCredits int64
		err := h.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_kurus), 0)
			FROM ledger_entries
			WHERE organization_id = $1
			  AND entry_type = 'credit'
			  AND reference_type = 'cod_settlement'
			  AND created_at >= $2::date
			  AND created_at < $2::date + INTERVAL '1 day'
		`, orgID, date).Scan(&ledgerCredits)
		if err != nil {
			h.logger.Warn("Failed to get COD ledger credits",
				"organization_id", orgID,
				"error", err,
			)
			continue
		}

		if totalSettled != ledgerCredits {
			report.Discrepancies = append(report.Discrepancies, Discrepancy{
				Type:           "cod_settlements",
				OrganizationID: orgID,
				Expected:       decimal.NewFromInt(totalSettled),
				Actual:         decimal.NewFromInt(ledgerCredits),
				Difference:     decimal.NewFromInt(totalSettled - ledgerCredits),
				Details:        fmt.Sprintf("COD settlements for %s do not match ledger credits", date.Format("2006-01-02")),
			})
		}
	}

	return rows.Err()
}

// reconcileHistoricalBalances performs a full check of all historical balances.
func (h *ReconciliationHandler) reconcileHistoricalBalances(ctx context.Context, report *ReconciliationReport) error {
	// This is a more expensive check that verifies the running balance
	// for each organization matches the balance_after_kurus in ledger entries
	rows, err := h.pool.Query(ctx, `
		WITH running_balances AS (
			SELECT 
				id,
				organization_id,
				entry_type,
				amount_kurus,
				balance_after_kurus,
				SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE -amount_kurus END) 
					OVER (PARTITION BY organization_id ORDER BY created_at, id) as computed_balance
			FROM ledger_entries
		)
		SELECT 
			id,
			organization_id,
			balance_after_kurus,
			computed_balance
		FROM running_balances
		WHERE balance_after_kurus != computed_balance
		LIMIT 100
	`)
	if err != nil {
		return fmt.Errorf("failed to query historical balances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entryID, orgID string
		var storedBalance, computedBalance int64
		if err := rows.Scan(&entryID, &orgID, &storedBalance, &computedBalance); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		report.Discrepancies = append(report.Discrepancies, Discrepancy{
			Type:           "historical_balance",
			OrganizationID: orgID,
			Expected:       decimal.NewFromInt(computedBalance),
			Actual:         decimal.NewFromInt(storedBalance),
			Difference:     decimal.NewFromInt(computedBalance - storedBalance),
			Details:        fmt.Sprintf("Ledger entry %s has incorrect balance_after_kurus", entryID),
		})
	}

	return rows.Err()
}

// logReconciliationReport logs the reconciliation report.
func (h *ReconciliationHandler) logReconciliationReport(report *ReconciliationReport) {
	h.logger.Info("Reconciliation report",
		"date", report.Date,
		"duration", report.Duration,
		"discrepancy_count", len(report.Discrepancies),
		"error_count", len(report.Errors),
	)

	for _, d := range report.Discrepancies {
		h.logger.Warn("Reconciliation discrepancy",
			"type", d.Type,
			"organization_id", d.OrganizationID,
			"expected", d.Expected.String(),
			"actual", d.Actual.String(),
			"difference", d.Difference.String(),
			"details", d.Details,
		)
	}

	for _, e := range report.Errors {
		h.logger.Error("Reconciliation error", "error", e)
	}
}

// CODSettlementHandler handles COD settlement processing tasks.
type CODSettlementHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewCODSettlementHandler creates a new COD settlement handler.
func NewCODSettlementHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	logger *observability.Logger,
) *CODSettlementHandler {
	return &CODSettlementHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// ProcessTask handles a COD settlement task.
func (h *CODSettlementHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseCODSettlementPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse COD settlement payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Processing COD settlement", "settlement_id", payload.SettlementID)

	// TODO: Implement COD settlement processing
	// This would typically:
	// 1. Get the settlement record
	// 2. Verify the settlement amount with the carrier
	// 3. Create a ledger entry for the credit
	// 4. Update the settlement status

	h.logger.Info("Completed COD settlement processing", "settlement_id", payload.SettlementID)
	return nil
}
