// Package tasks provides background task handlers for the worker.
package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// ReconciliationHandler handles daily financial reconciliation tasks.
// It verifies ledger invariants, reconciles with payment providers,
// and checks for any financial discrepancies.
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

// ReconciliationResult holds the results of a reconciliation run.
type ReconciliationResult struct {
	Date                  string                 `json:"date"`
	StartedAt             time.Time              `json:"started_at"`
	CompletedAt           time.Time              `json:"completed_at"`
	OrganizationsChecked  int                    `json:"organizations_checked"`
	InvariantViolations   []InvariantViolation   `json:"invariant_violations,omitempty"`
	TopupDiscrepancies    []TopupDiscrepancy     `json:"topup_discrepancies,omitempty"`
	CODSettlementIssues   []CODSettlementIssue   `json:"cod_settlement_issues,omitempty"`
	ShipmentBillingIssues []ShipmentBillingIssue `json:"shipment_billing_issues,omitempty"`
	TotalCreditsKurus     int64                  `json:"total_credits_kurus"`
	TotalDebitsKurus      int64                  `json:"total_debits_kurus"`
	NetChangeKurus        int64                  `json:"net_change_kurus"`
	Status                string                 `json:"status"` // "success", "warning", "error"
}

// InvariantViolation represents a ledger invariant violation.
type InvariantViolation struct {
	OrganizationID  uuid.UUID `json:"organization_id"`
	ComputedBalance int64     `json:"computed_balance"`
	StoredBalance   int64     `json:"stored_balance"`
	Difference      int64     `json:"difference"`
}

// TopupDiscrepancy represents a discrepancy in top-up records.
type TopupDiscrepancy struct {
	TopupID        uuid.UUID `json:"topup_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Status         string    `json:"status"`
	AmountKurus    int64     `json:"amount_kurus"`
	Issue          string    `json:"issue"`
}

// CODSettlementIssue represents an issue with COD settlement.
type CODSettlementIssue struct {
	SettlementID     uuid.UUID `json:"settlement_id"`
	ShipmentID       uuid.UUID `json:"shipment_id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	ExpectedKurus    int64     `json:"expected_kurus"`
	ActualKurus      int64     `json:"actual_kurus"`
	DiscrepancyKurus int64     `json:"discrepancy_kurus"`
	Issue            string    `json:"issue"`
}

// ShipmentBillingIssue represents a billing issue with a shipment.
type ShipmentBillingIssue struct {
	ShipmentID     uuid.UUID `json:"shipment_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Issue          string    `json:"issue"`
}

// ProcessTask handles the reconciliation task.
// It runs daily at 4:00 AM Turkey time as configured in the scheduler.
func (h *ReconciliationHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseReconciliationPayload(task)
	if err != nil {
		h.logger.Error("Failed to parse reconciliation payload", "error", err)
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	h.logger.Info("Starting daily reconciliation",
		"date", payload.Date,
		"full_check", payload.FullCheck,
	)

	result := &ReconciliationResult{
		Date:      payload.Date,
		StartedAt: time.Now(),
		Status:    "success",
	}

	// Parse the date for filtering
	reconcileDate, err := time.Parse("2006-01-02", payload.Date)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	// Step 1: Verify ledger invariants for all organizations
	if err := h.verifyLedgerInvariants(ctx, result); err != nil {
		h.logger.Error("Failed to verify ledger invariants", "error", err)
		result.Status = "error"
	}

	// Step 2: Check for pending/stuck top-ups
	if err := h.checkTopupDiscrepancies(ctx, result, reconcileDate); err != nil {
		h.logger.Error("Failed to check topup discrepancies", "error", err)
		result.Status = "error"
	}

	// Step 3: Check COD settlements
	if err := h.checkCODSettlements(ctx, result, reconcileDate); err != nil {
		h.logger.Error("Failed to check COD settlements", "error", err)
		result.Status = "error"
	}

	// Step 4: Verify shipment billing
	if err := h.verifyShipmentBilling(ctx, result, reconcileDate); err != nil {
		h.logger.Error("Failed to verify shipment billing", "error", err)
		result.Status = "error"
	}

	// Step 5: Calculate daily totals
	if err := h.calculateDailyTotals(ctx, result, reconcileDate); err != nil {
		h.logger.Error("Failed to calculate daily totals", "error", err)
		// Don't fail the whole job for this
	}

	result.CompletedAt = time.Now()

	// Determine final status
	if len(result.InvariantViolations) > 0 {
		result.Status = "error"
	} else if len(result.TopupDiscrepancies) > 0 || len(result.CODSettlementIssues) > 0 || len(result.ShipmentBillingIssues) > 0 {
		if result.Status != "error" {
			result.Status = "warning"
		}
	}

	// Log the result
	resultJSON, _ := json.Marshal(result)
	h.logger.Info("Completed daily reconciliation",
		"date", payload.Date,
		"status", result.Status,
		"organizations_checked", result.OrganizationsChecked,
		"invariant_violations", len(result.InvariantViolations),
		"topup_discrepancies", len(result.TopupDiscrepancies),
		"cod_issues", len(result.CODSettlementIssues),
		"shipment_issues", len(result.ShipmentBillingIssues),
		"duration_ms", result.CompletedAt.Sub(result.StartedAt).Milliseconds(),
	)

	// Store reconciliation result in audit log
	if err := h.storeReconciliationResult(ctx, result, resultJSON); err != nil {
		h.logger.Warn("Failed to store reconciliation result", "error", err)
	}

	// If there are critical issues (invariant violations), return an error
	// so the task can be retried or alerted on
	if len(result.InvariantViolations) > 0 {
		return fmt.Errorf("reconciliation found %d invariant violations", len(result.InvariantViolations))
	}

	return nil
}

// verifyLedgerInvariants checks that all organization ledgers are consistent.
func (h *ReconciliationHandler) verifyLedgerInvariants(ctx context.Context, result *ReconciliationResult) error {
	// Get all organizations with ledger entries
	balances, err := h.queries.ListAllOrganizationBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list organization balances: %w", err)
	}

	result.OrganizationsChecked = len(balances)

	for _, balance := range balances {
		// Check invariant for each organization
		invariant, err := h.queries.CheckLedgerInvariant(ctx, balance.OrganizationID)
		if err != nil {
			h.logger.Warn("Failed to check invariant for organization",
				"organization_id", balance.OrganizationID,
				"error", err,
			)
			continue
		}

		// Compare computed vs stored balance
		if invariant.ComputedBalance != invariant.StoredBalance {
			violation := InvariantViolation{
				OrganizationID:  balance.OrganizationID,
				ComputedBalance: int64(invariant.ComputedBalance),
				StoredBalance:   int64(invariant.StoredBalance),
				Difference:      int64(invariant.ComputedBalance - invariant.StoredBalance),
			}
			result.InvariantViolations = append(result.InvariantViolations, violation)

			h.logger.Error("Ledger invariant violation detected",
				"organization_id", balance.OrganizationID,
				"computed_balance", invariant.ComputedBalance,
				"stored_balance", invariant.StoredBalance,
				"difference", violation.Difference,
			)
		}
	}

	return nil
}

// checkTopupDiscrepancies checks for stuck or inconsistent top-ups.
func (h *ReconciliationHandler) checkTopupDiscrepancies(ctx context.Context, result *ReconciliationResult, date time.Time) error {
	// Check for top-ups that are stuck in "processing" status for too long
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	topups, err := h.queries.ListTopupsByDateRange(ctx, db.ListTopupsByDateRangeParams{
		CreatedAt:   pgtype.Timestamptz{Time: startOfDay, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: endOfDay, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to list topups: %w", err)
	}

	for _, topup := range topups {
		// Check for stuck processing topups (older than 1 hour)
		if topup.Status == "processing" {
			if time.Since(topup.CreatedAt.Time) > time.Hour {
				result.TopupDiscrepancies = append(result.TopupDiscrepancies, TopupDiscrepancy{
					TopupID:        topup.ID,
					OrganizationID: topup.OrganizationID,
					Status:         topup.Status,
					AmountKurus:    int64(topup.AmountKurus),
					Issue:          "Topup stuck in processing status for over 1 hour",
				})
			}
		}

		// Check for completed topups without ledger entry
		if topup.Status == "completed" && !topup.LedgerEntryID.Valid {
			result.TopupDiscrepancies = append(result.TopupDiscrepancies, TopupDiscrepancy{
				TopupID:        topup.ID,
				OrganizationID: topup.OrganizationID,
				Status:         topup.Status,
				AmountKurus:    int64(topup.AmountKurus),
				Issue:          "Completed topup missing ledger entry",
			})
		}

		// Check for expired pending topups
		if topup.Status == "pending" && topup.ExpiresAt.Time.Before(time.Now()) {
			result.TopupDiscrepancies = append(result.TopupDiscrepancies, TopupDiscrepancy{
				TopupID:        topup.ID,
				OrganizationID: topup.OrganizationID,
				Status:         topup.Status,
				AmountKurus:    int64(topup.AmountKurus),
				Issue:          "Pending topup has expired but not marked as failed",
			})
		}
	}

	return nil
}

// checkCODSettlements checks for COD settlement issues.
func (h *ReconciliationHandler) checkCODSettlements(ctx context.Context, result *ReconciliationResult, date time.Time) error {
	// Check for COD settlements that are pending for too long
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	settlements, err := h.queries.ListCODSettlementsByDateRange(ctx, db.ListCODSettlementsByDateRangeParams{
		CreatedAt:   pgtype.Timestamptz{Time: startOfDay, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: endOfDay, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to list COD settlements: %w", err)
	}

	for _, settlement := range settlements {
		// Check for discrepancies between expected and actual amounts
		if settlement.ActualAmountKurus != nil {
			expected := int64(settlement.ExpectedAmountKurus)
			actual := int64(*settlement.ActualAmountKurus)
			if expected != actual {
				result.CODSettlementIssues = append(result.CODSettlementIssues, CODSettlementIssue{
					SettlementID:     settlement.ID,
					ShipmentID:       settlement.ShipmentID,
					OrganizationID:   settlement.OrganizationID,
					ExpectedKurus:    expected,
					ActualKurus:      actual,
					DiscrepancyKurus: expected - actual,
					Issue:            "COD amount discrepancy between expected and actual",
				})
			}
		}

		// Check for settled COD without ledger entry
		if settlement.Status == "settled" && !settlement.LedgerEntryID.Valid {
			result.CODSettlementIssues = append(result.CODSettlementIssues, CODSettlementIssue{
				SettlementID:   settlement.ID,
				ShipmentID:     settlement.ShipmentID,
				OrganizationID: settlement.OrganizationID,
				ExpectedKurus:  int64(settlement.ExpectedAmountKurus),
				Issue:          "Settled COD missing ledger entry",
			})
		}

		// Check for old pending COD settlements (delivered shipments)
		if settlement.Status == "pending" {
			// Get shipment to check if delivered
			shipment, err := h.queries.GetShipmentByID(ctx, settlement.ShipmentID)
			if err == nil && shipment.Status == "delivered" {
				// If shipment delivered more than 7 days ago, flag it
				if shipment.DeliveredAt.Valid && time.Since(shipment.DeliveredAt.Time) > 7*24*time.Hour {
					result.CODSettlementIssues = append(result.CODSettlementIssues, CODSettlementIssue{
						SettlementID:   settlement.ID,
						ShipmentID:     settlement.ShipmentID,
						OrganizationID: settlement.OrganizationID,
						ExpectedKurus:  int64(settlement.ExpectedAmountKurus),
						Issue:          "COD settlement pending for delivered shipment older than 7 days",
					})
				}
			}
		}
	}

	return nil
}

// verifyShipmentBilling checks for shipment billing issues.
func (h *ReconciliationHandler) verifyShipmentBilling(ctx context.Context, result *ReconciliationResult, date time.Time) error {
	// Check for marketplace shipments with label_purchased status but no ledger entry
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	shipments, err := h.queries.ListShipmentsByDateRange(ctx, db.ListShipmentsByDateRangeParams{
		CreatedAt:   pgtype.Timestamptz{Time: startOfDay, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: endOfDay, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to list shipments: %w", err)
	}

	for _, shipment := range shipments {
		// Only check marketplace shipments (they should have wallet charges)
		if !shipment.IsMarketplace {
			continue
		}

		// Check if shipment has label purchased but no price charged
		if shipment.Status != "draft" && shipment.PriceChargedKurus == nil {
			result.ShipmentBillingIssues = append(result.ShipmentBillingIssues, ShipmentBillingIssue{
				ShipmentID:     shipment.ID,
				OrganizationID: shipment.OrganizationID,
				Issue:          "Marketplace shipment with label but no price charged",
			})
		}

		// Check for shipments with price charged but verify ledger entry exists
		if shipment.PriceChargedKurus != nil && *shipment.PriceChargedKurus > 0 {
			_, err := h.queries.GetLedgerEntryByReference(ctx, db.GetLedgerEntryByReferenceParams{
				ReferenceType: "shipment",
				ReferenceID:   shipment.ID,
			})
			if err != nil {
				result.ShipmentBillingIssues = append(result.ShipmentBillingIssues, ShipmentBillingIssue{
					ShipmentID:     shipment.ID,
					OrganizationID: shipment.OrganizationID,
					Issue:          "Shipment with price charged but no ledger entry found",
				})
			}
		}
	}

	return nil
}

// calculateDailyTotals calculates the daily financial totals.
func (h *ReconciliationHandler) calculateDailyTotals(ctx context.Context, result *ReconciliationResult, date time.Time) error {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Get all ledger entries for the day
	entries, err := h.queries.GetLedgerTotalsByDateRange(ctx, db.GetLedgerTotalsByDateRangeParams{
		CreatedAt:   pgtype.Timestamptz{Time: startOfDay, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: endOfDay, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to get ledger totals: %w", err)
	}

	result.TotalCreditsKurus = interfaceToInt64(entries.TotalCreditsKurus)
	result.TotalDebitsKurus = interfaceToInt64(entries.TotalDebitsKurus)
	result.NetChangeKurus = result.TotalCreditsKurus - result.TotalDebitsKurus

	return nil
}

// interfaceToInt64 converts an interface{} to int64.
// This is needed because COALESCE in PostgreSQL returns interface{} type.
func interfaceToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
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

// storeReconciliationResult stores the reconciliation result in the audit log.
func (h *ReconciliationHandler) storeReconciliationResult(ctx context.Context, result *ReconciliationResult, resultJSON []byte) error {
	_, err := h.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorType:    "system",
		Action:       "reconciliation.completed",
		ResourceType: "reconciliation",
		NewValue:     resultJSON,
	})
	return err
}
