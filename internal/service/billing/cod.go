// Package billing provides wallet and ledger management services.
package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// Common errors for COD settlement operations.
var (
	ErrCODSettlementNotFound      = errors.New("COD settlement not found")
	ErrCODSettlementInvalidState  = errors.New("COD settlement is in invalid state for this operation")
	ErrCODSettlementAlreadyExists = errors.New("COD settlement already exists for this shipment")
	ErrCODReconciliationFailed    = errors.New("COD reconciliation failed")
)

// CODSettlementStatus represents the status of a COD settlement.
type CODSettlementStatus string

const (
	CODSettlementStatusPending    CODSettlementStatus = "pending"
	CODSettlementStatusCollected  CODSettlementStatus = "collected"
	CODSettlementStatusSettled    CODSettlementStatus = "settled"
	CODSettlementStatusDisputed   CODSettlementStatus = "disputed"
	CODSettlementStatusWrittenOff CODSettlementStatus = "written_off"
)

// String returns the string representation of the status.
func (s CODSettlementStatus) String() string {
	return string(s)
}

// CODService provides COD settlement tracking and reconciliation operations.
type CODService struct {
	queries       *db.Queries
	ledgerService *LedgerService
	logger        *observability.Logger
}

// NewCODService creates a new COD service.
func NewCODService(queries *db.Queries, ledgerService *LedgerService, logger *observability.Logger) *CODService {
	return &CODService{
		queries:       queries,
		ledgerService: ledgerService,
		logger:        logger,
	}
}

// CODSettlement represents a COD settlement record.
type CODSettlement struct {
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	ShipmentID            uuid.UUID  `json:"shipment_id"`
	ProviderCode          string     `json:"provider_code"`
	ExpectedAmountKurus   int32      `json:"expected_amount_kurus"`
	ActualAmountKurus     *int32     `json:"actual_amount_kurus,omitempty"`
	CarrierSettlementID   *string    `json:"carrier_settlement_id,omitempty"`
	CarrierSettlementDate *time.Time `json:"carrier_settlement_date,omitempty"`
	Status                string     `json:"status"`
	LedgerEntryID         *uuid.UUID `json:"ledger_entry_id,omitempty"`
	DiscrepancyKurus      *int32     `json:"discrepancy_kurus,omitempty"`
	DiscrepancyReason     *string    `json:"discrepancy_reason,omitempty"`
	SettledAt             *time.Time `json:"settled_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// CreateCODSettlementRequest represents a request to create a COD settlement.
type CreateCODSettlementRequest struct {
	OrganizationID      uuid.UUID
	ShipmentID          uuid.UUID
	ProviderCode        string
	ExpectedAmountKurus int32
}

// CreateSettlement creates a new COD settlement record for a shipment.
func (s *CODService) CreateSettlement(ctx context.Context, req *CreateCODSettlementRequest) (*CODSettlement, error) {
	// Check if settlement already exists for this shipment
	_, err := s.queries.GetCODSettlementByShipment(ctx, req.ShipmentID)
	if err == nil {
		return nil, ErrCODSettlementAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing settlement: %w", err)
	}

	// Create settlement
	settlement, err := s.queries.CreateCODSettlement(ctx, db.CreateCODSettlementParams{
		OrganizationID:      req.OrganizationID,
		ShipmentID:          req.ShipmentID,
		ProviderCode:        req.ProviderCode,
		ExpectedAmountKurus: req.ExpectedAmountKurus,
		Status:              CODSettlementStatusPending.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create COD settlement: %w", err)
	}

	s.logger.Info("COD settlement created",
		"settlement_id", settlement.ID,
		"shipment_id", req.ShipmentID,
		"organization_id", req.OrganizationID,
		"expected_amount_kurus", req.ExpectedAmountKurus,
	)

	return dbSettlementToCODSettlement(settlement), nil
}

// GetSettlement retrieves a COD settlement by ID.
func (s *CODService) GetSettlement(ctx context.Context, settlementID uuid.UUID) (*CODSettlement, error) {
	settlement, err := s.queries.GetCODSettlementByID(ctx, settlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}
	return dbSettlementToCODSettlement(settlement), nil
}

// GetSettlementByShipment retrieves a COD settlement by shipment ID.
func (s *CODService) GetSettlementByShipment(ctx context.Context, shipmentID uuid.UUID) (*CODSettlement, error) {
	settlement, err := s.queries.GetCODSettlementByShipment(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}
	return dbSettlementToCODSettlement(settlement), nil
}

// ListSettlementsRequest represents a request to list COD settlements.
type ListSettlementsRequest struct {
	OrganizationID uuid.UUID
	Status         *string
	ProviderCode   *string
	Offset         int
	Limit          int
}

// ListSettlementsResult represents the result of listing COD settlements.
type ListSettlementsResult struct {
	Settlements []*CODSettlement
	Total       int64
}

// ListSettlements lists COD settlements for an organization with filtering.
func (s *CODService) ListSettlements(ctx context.Context, req *ListSettlementsRequest) (*ListSettlementsResult, error) {
	// Get count
	total, err := s.queries.CountCODSettlementsByOrganization(ctx, db.CountCODSettlementsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
		ProviderCode:   req.ProviderCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count settlements: %w", err)
	}

	// Get settlements
	settlements, err := s.queries.ListCODSettlementsByOrganization(ctx, db.ListCODSettlementsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
		ProviderCode:   req.ProviderCode,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list settlements: %w", err)
	}

	result := &ListSettlementsResult{
		Settlements: make([]*CODSettlement, 0, len(settlements)),
		Total:       total,
	}

	for _, settlement := range settlements {
		result.Settlements = append(result.Settlements, dbSettlementToCODSettlement(settlement))
	}

	return result, nil
}

// MarkCollectedRequest represents a request to mark a COD settlement as collected.
type MarkCollectedRequest struct {
	SettlementID          uuid.UUID
	ActualAmountKurus     int32
	CarrierSettlementID   string
	CarrierSettlementDate time.Time
}

// MarkCollected marks a COD settlement as collected by the carrier.
func (s *CODService) MarkCollected(ctx context.Context, req *MarkCollectedRequest) (*CODSettlement, error) {
	// Get current settlement
	settlement, err := s.queries.GetCODSettlementByID(ctx, req.SettlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}

	// Verify status
	if settlement.Status != CODSettlementStatusPending.String() {
		return nil, ErrCODSettlementInvalidState
	}

	// Update settlement
	actualAmount := req.ActualAmountKurus
	carrierID := req.CarrierSettlementID
	updated, err := s.queries.UpdateCODSettlementCollected(ctx, db.UpdateCODSettlementCollectedParams{
		ID:                    req.SettlementID,
		ActualAmountKurus:     &actualAmount,
		CarrierSettlementID:   &carrierID,
		CarrierSettlementDate: pgtype.Date{Time: req.CarrierSettlementDate, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to mark settlement as collected: %w", err)
	}

	s.logger.Info("COD settlement marked as collected",
		"settlement_id", req.SettlementID,
		"actual_amount_kurus", req.ActualAmountKurus,
		"carrier_settlement_id", req.CarrierSettlementID,
	)

	return dbSettlementToCODSettlement(updated), nil
}

// ReconcileRequest represents a request to reconcile a COD settlement.
type ReconcileRequest struct {
	SettlementID uuid.UUID
	ActorID      *uuid.UUID
	ActorType    domain.ActorType
}

// Reconcile reconciles a collected COD settlement by crediting the organization's wallet.
func (s *CODService) Reconcile(ctx context.Context, req *ReconcileRequest) (*CODSettlement, error) {
	// Get current settlement
	settlement, err := s.queries.GetCODSettlementByID(ctx, req.SettlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}

	// Verify status
	if settlement.Status != CODSettlementStatusCollected.String() {
		return nil, ErrCODSettlementInvalidState
	}

	// Verify actual amount is set
	if settlement.ActualAmountKurus == nil || *settlement.ActualAmountKurus <= 0 {
		return nil, fmt.Errorf("%w: actual amount not set", ErrCODReconciliationFailed)
	}

	// Credit the organization's wallet
	ledgerEntry, err := s.ledgerService.Credit(ctx, &CreditRequest{
		OrganizationID: settlement.OrganizationID,
		AmountKurus:    int64(*settlement.ActualAmountKurus),
		ReferenceType:  domain.ReferenceTypeCODSettlement,
		ReferenceID:    settlement.ID,
		Description:    fmt.Sprintf("COD settlement for shipment - %s", settlement.ShipmentID),
		Metadata: map[string]interface{}{
			"settlement_id":         settlement.ID.String(),
			"shipment_id":           settlement.ShipmentID.String(),
			"provider_code":         settlement.ProviderCode,
			"expected_amount_kurus": settlement.ExpectedAmountKurus,
			"actual_amount_kurus":   *settlement.ActualAmountKurus,
			"carrier_settlement_id": settlement.CarrierSettlementID,
		},
		ActorID:   req.ActorID,
		ActorType: req.ActorType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to credit wallet for COD settlement: %w", err)
	}

	// Update settlement as settled
	updated, err := s.queries.UpdateCODSettlementSettled(ctx, db.UpdateCODSettlementSettledParams{
		ID:            req.SettlementID,
		LedgerEntryID: pgtype.UUID{Bytes: ledgerEntry.ID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to mark settlement as settled: %w", err)
	}

	s.logger.Info("COD settlement reconciled",
		"settlement_id", req.SettlementID,
		"ledger_entry_id", ledgerEntry.ID,
		"amount_kurus", *settlement.ActualAmountKurus,
	)

	return dbSettlementToCODSettlement(updated), nil
}

// BatchReconcileRequest represents a request to reconcile multiple COD settlements.
type BatchReconcileRequest struct {
	ProviderCode          string
	CarrierSettlementDate time.Time
	ActorID               *uuid.UUID
	ActorType             domain.ActorType
}

// BatchReconcileResult represents the result of batch reconciliation.
type BatchReconcileResult struct {
	Reconciled  int
	Failed      int
	TotalKurus  int64
	Errors      []string
	Settlements []*CODSettlement
}

// BatchReconcile reconciles all collected COD settlements for a provider and date.
func (s *CODService) BatchReconcile(ctx context.Context, req *BatchReconcileRequest) (*BatchReconcileResult, error) {
	// Get settlements for reconciliation
	settlements, err := s.queries.ListCODSettlementsForReconciliation(ctx, db.ListCODSettlementsForReconciliationParams{
		ProviderCode:          req.ProviderCode,
		CarrierSettlementDate: pgtype.Date{Time: req.CarrierSettlementDate, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list settlements for reconciliation: %w", err)
	}

	result := &BatchReconcileResult{
		Settlements: make([]*CODSettlement, 0),
		Errors:      make([]string, 0),
	}

	for _, settlement := range settlements {
		// Only reconcile collected settlements
		if settlement.Status != CODSettlementStatusCollected.String() {
			continue
		}

		reconciled, err := s.Reconcile(ctx, &ReconcileRequest{
			SettlementID: settlement.ID,
			ActorID:      req.ActorID,
			ActorType:    req.ActorType,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("settlement %s: %v", settlement.ID, err))
			continue
		}

		result.Reconciled++
		if settlement.ActualAmountKurus != nil {
			result.TotalKurus += int64(*settlement.ActualAmountKurus)
		}
		result.Settlements = append(result.Settlements, reconciled)
	}

	s.logger.Info("Batch COD reconciliation completed",
		"provider_code", req.ProviderCode,
		"settlement_date", req.CarrierSettlementDate,
		"reconciled", result.Reconciled,
		"failed", result.Failed,
		"total_kurus", result.TotalKurus,
	)

	return result, nil
}

// DisputeRequest represents a request to dispute a COD settlement.
type DisputeRequest struct {
	SettlementID      uuid.UUID
	DiscrepancyReason string
}

// Dispute marks a COD settlement as disputed.
func (s *CODService) Dispute(ctx context.Context, req *DisputeRequest) (*CODSettlement, error) {
	// Get current settlement
	settlement, err := s.queries.GetCODSettlementByID(ctx, req.SettlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}

	// Verify status
	if settlement.Status != CODSettlementStatusPending.String() && settlement.Status != CODSettlementStatusCollected.String() {
		return nil, ErrCODSettlementInvalidState
	}

	// Update settlement
	updated, err := s.queries.UpdateCODSettlementDisputed(ctx, db.UpdateCODSettlementDisputedParams{
		ID:                req.SettlementID,
		DiscrepancyReason: &req.DiscrepancyReason,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to mark settlement as disputed: %w", err)
	}

	s.logger.Info("COD settlement disputed",
		"settlement_id", req.SettlementID,
		"reason", req.DiscrepancyReason,
	)

	return dbSettlementToCODSettlement(updated), nil
}

// WriteOffRequest represents a request to write off a disputed COD settlement.
type WriteOffRequest struct {
	SettlementID      uuid.UUID
	DiscrepancyReason string
}

// WriteOff writes off a disputed COD settlement.
func (s *CODService) WriteOff(ctx context.Context, req *WriteOffRequest) (*CODSettlement, error) {
	// Get current settlement
	settlement, err := s.queries.GetCODSettlementByID(ctx, req.SettlementID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCODSettlementNotFound
		}
		return nil, err
	}

	// Verify status
	if settlement.Status != CODSettlementStatusDisputed.String() {
		return nil, ErrCODSettlementInvalidState
	}

	// Update settlement
	updated, err := s.queries.UpdateCODSettlementWrittenOff(ctx, db.UpdateCODSettlementWrittenOffParams{
		ID:                req.SettlementID,
		DiscrepancyReason: &req.DiscrepancyReason,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to write off settlement: %w", err)
	}

	s.logger.Info("COD settlement written off",
		"settlement_id", req.SettlementID,
		"reason", req.DiscrepancyReason,
	)

	return dbSettlementToCODSettlement(updated), nil
}

// GetSummary retrieves a summary of COD settlements for an organization.
func (s *CODService) GetSummary(ctx context.Context, organizationID uuid.UUID) (*CODSettlementSummary, error) {
	summary, err := s.queries.GetCODSettlementSummaryByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get COD settlement summary: %w", err)
	}

	return &CODSettlementSummary{
		TotalSettlements:   summary.TotalSettlements,
		TotalExpectedKurus: interfaceToInt64(summary.TotalExpectedKurus),
		TotalSettledKurus:  interfaceToInt64(summary.TotalSettledKurus),
		TotalPendingKurus:  interfaceToInt64(summary.TotalPendingKurus),
		TotalDisputedKurus: interfaceToInt64(summary.TotalDisputedKurus),
	}, nil
}

// GetSummaryByProvider retrieves a summary of COD settlements by provider for an organization.
func (s *CODService) GetSummaryByProvider(ctx context.Context, organizationID uuid.UUID) ([]*CODProviderSummary, error) {
	summaries, err := s.queries.GetCODSettlementSummaryByProvider(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get COD settlement summary by provider: %w", err)
	}

	result := make([]*CODProviderSummary, 0, len(summaries))
	for _, summary := range summaries {
		result = append(result, &CODProviderSummary{
			ProviderCode:       summary.ProviderCode,
			TotalSettlements:   summary.TotalSettlements,
			TotalExpectedKurus: interfaceToInt64(summary.TotalExpectedKurus),
			TotalSettledKurus:  interfaceToInt64(summary.TotalSettledKurus),
			TotalPendingKurus:  interfaceToInt64(summary.TotalPendingKurus),
		})
	}

	return result, nil
}

// CODSettlementSummary represents a summary of COD settlements.
type CODSettlementSummary struct {
	TotalSettlements   int64 `json:"total_settlements"`
	TotalExpectedKurus int64 `json:"total_expected_kurus"`
	TotalSettledKurus  int64 `json:"total_settled_kurus"`
	TotalPendingKurus  int64 `json:"total_pending_kurus"`
	TotalDisputedKurus int64 `json:"total_disputed_kurus"`
}

// CODProviderSummary represents a summary of COD settlements for a provider.
type CODProviderSummary struct {
	ProviderCode       string `json:"provider_code"`
	TotalSettlements   int64  `json:"total_settlements"`
	TotalExpectedKurus int64  `json:"total_expected_kurus"`
	TotalSettledKurus  int64  `json:"total_settled_kurus"`
	TotalPendingKurus  int64  `json:"total_pending_kurus"`
}

// Helper function to convert db.CodSettlement to CODSettlement
func dbSettlementToCODSettlement(settlement *db.CodSettlement) *CODSettlement {
	result := &CODSettlement{
		ID:                  settlement.ID,
		OrganizationID:      settlement.OrganizationID,
		ShipmentID:          settlement.ShipmentID,
		ProviderCode:        settlement.ProviderCode,
		ExpectedAmountKurus: settlement.ExpectedAmountKurus,
		ActualAmountKurus:   settlement.ActualAmountKurus,
		CarrierSettlementID: settlement.CarrierSettlementID,
		Status:              settlement.Status,
		DiscrepancyKurus:    settlement.DiscrepancyKurus,
		DiscrepancyReason:   settlement.DiscrepancyReason,
		CreatedAt:           settlement.CreatedAt.Time,
		UpdatedAt:           settlement.UpdatedAt.Time,
	}

	// Parse carrier settlement date
	if settlement.CarrierSettlementDate.Valid {
		t := settlement.CarrierSettlementDate.Time
		result.CarrierSettlementDate = &t
	}

	// Parse ledger entry ID
	if settlement.LedgerEntryID.Valid {
		id := uuid.UUID(settlement.LedgerEntryID.Bytes)
		result.LedgerEntryID = &id
	}

	// Parse settled at
	if settlement.SettledAt.Valid {
		t := settlement.SettledAt.Time
		result.SettledAt = &t
	}

	return result
}

// Note: interfaceToInt64 is defined in wallet.go
