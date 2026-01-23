// Package billing provides wallet and ledger management services.
package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// Common errors for refund operations.
var (
	ErrRefundNotFound        = errors.New("refund not found")
	ErrRefundInvalidState    = errors.New("refund is in invalid state for this operation")
	ErrRefundAlreadyExists   = errors.New("refund already exists for this reference")
	ErrRefundAmountExceeded  = errors.New("refund amount exceeds original charge")
	ErrOriginalEntryNotFound = errors.New("original ledger entry not found")
)

// RefundService provides refund processing operations.
type RefundService struct {
	queries       *db.Queries
	ledgerService *LedgerService
	logger        *observability.Logger
}

// NewRefundService creates a new refund service.
func NewRefundService(queries *db.Queries, ledgerService *LedgerService, logger *observability.Logger) *RefundService {
	return &RefundService{
		queries:       queries,
		ledgerService: ledgerService,
		logger:        logger,
	}
}

// Refund represents a refund record.
type Refund struct {
	ID                    uuid.UUID  `json:"id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	OriginalLedgerEntryID uuid.UUID  `json:"original_ledger_entry_id"`
	RefundLedgerEntryID   *uuid.UUID `json:"refund_ledger_entry_id,omitempty"`
	AmountKurus           int64      `json:"amount_kurus"`
	Reason                string     `json:"reason"`
	Status                string     `json:"status"`
	ReferenceType         string     `json:"reference_type"`
	ReferenceID           uuid.UUID  `json:"reference_id"`
	ApprovedBy            *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt            *time.Time `json:"approved_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// CreateRefundRequest represents a request to create a refund.
type CreateRefundRequest struct {
	OrganizationID uuid.UUID
	ShipmentID     uuid.UUID
	AmountKurus    int64
	Reason         string
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
}

// CreateRefundResult represents the result of creating a refund.
type CreateRefundResult struct {
	Refund      *Refund
	LedgerEntry *LedgerEntry
}

// Create creates a new refund and immediately processes it (credits the wallet).
// For shipment cancellations, this is called automatically.
func (s *RefundService) Create(ctx context.Context, req *CreateRefundRequest) (*CreateRefundResult, error) {
	if req.AmountKurus <= 0 {
		return nil, ErrInvalidAmount
	}

	// Find the original debit ledger entry for this shipment
	originalEntry, err := s.ledgerService.GetEntryByReference(ctx, domain.ReferenceTypeShipment, req.ShipmentID)
	if err != nil {
		if errors.Is(err, ErrLedgerEntryNotFound) {
			return nil, ErrOriginalEntryNotFound
		}
		return nil, fmt.Errorf("failed to get original ledger entry: %w", err)
	}

	// Verify the original entry was a debit
	if originalEntry.EntryType != domain.EntryTypeDebit.String() {
		return nil, fmt.Errorf("original entry is not a debit")
	}

	// Verify refund amount doesn't exceed original charge
	if req.AmountKurus > originalEntry.AmountKurus {
		return nil, ErrRefundAmountExceeded
	}

	// Check if a refund already exists for this shipment
	existingRefund, err := s.GetByShipmentID(ctx, req.ShipmentID)
	if err == nil && existingRefund != nil {
		// Return existing refund if already completed
		if existingRefund.Status == domain.RefundStatusCompleted.String() {
			ledgerEntry, _ := s.ledgerService.GetEntry(ctx, *existingRefund.RefundLedgerEntryID)
			return &CreateRefundResult{
				Refund:      existingRefund,
				LedgerEntry: ledgerEntry,
			}, nil
		}
		return nil, ErrRefundAlreadyExists
	}

	// Create the credit ledger entry for the refund
	creditEntry, err := s.ledgerService.Credit(ctx, &CreditRequest{
		OrganizationID: req.OrganizationID,
		AmountKurus:    req.AmountKurus,
		ReferenceType:  domain.ReferenceTypeRefund,
		ReferenceID:    req.ShipmentID, // Use shipment ID as reference
		Description:    fmt.Sprintf("Refund: %s", req.Reason),
		Metadata: map[string]interface{}{
			"shipment_id":              req.ShipmentID.String(),
			"original_ledger_entry_id": originalEntry.ID.String(),
			"reason":                   req.Reason,
		},
		ActorID:   req.ActorID,
		ActorType: req.ActorType,
	})
	if err != nil {
		// If duplicate reference error, the refund was already processed
		if errors.Is(err, ErrDuplicateReference) {
			return nil, ErrRefundAlreadyExists
		}
		return nil, fmt.Errorf("failed to create refund credit entry: %w", err)
	}

	// Create refund record
	now := time.Now().UTC()
	refund := &Refund{
		ID:                    uuid.New(),
		OrganizationID:        req.OrganizationID,
		OriginalLedgerEntryID: originalEntry.ID,
		RefundLedgerEntryID:   &creditEntry.ID,
		AmountKurus:           req.AmountKurus,
		Reason:                req.Reason,
		Status:                domain.RefundStatusCompleted.String(),
		ReferenceType:         domain.ReferenceTypeShipment.String(),
		ReferenceID:           req.ShipmentID,
		ApprovedBy:            req.ActorID,
		ApprovedAt:            &now,
		CompletedAt:           &now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	s.logger.Info("Refund processed successfully",
		"refund_id", refund.ID,
		"organization_id", req.OrganizationID,
		"shipment_id", req.ShipmentID,
		"amount_kurus", req.AmountKurus,
		"ledger_entry_id", creditEntry.ID,
	)

	return &CreateRefundResult{
		Refund:      refund,
		LedgerEntry: creditEntry,
	}, nil
}

// CreateManualRefundRequest represents a request to create a manual refund.
type CreateManualRefundRequest struct {
	OrganizationID        uuid.UUID
	OriginalLedgerEntryID uuid.UUID
	AmountKurus           int64
	Reason                string
	ActorID               *uuid.UUID
	ActorType             domain.ActorType
}

// CreateManual creates a manual refund for any ledger entry.
// This requires admin approval workflow (not auto-completed).
func (s *RefundService) CreateManual(ctx context.Context, req *CreateManualRefundRequest) (*Refund, error) {
	if req.AmountKurus <= 0 {
		return nil, ErrInvalidAmount
	}

	// Get the original ledger entry
	originalEntry, err := s.ledgerService.GetEntry(ctx, req.OriginalLedgerEntryID)
	if err != nil {
		if errors.Is(err, ErrLedgerEntryNotFound) {
			return nil, ErrOriginalEntryNotFound
		}
		return nil, fmt.Errorf("failed to get original ledger entry: %w", err)
	}

	// Verify organization ownership
	if originalEntry.OrganizationID != req.OrganizationID {
		return nil, ErrOriginalEntryNotFound
	}

	// Verify the original entry was a debit
	if originalEntry.EntryType != domain.EntryTypeDebit.String() {
		return nil, fmt.Errorf("can only refund debit entries")
	}

	// Verify refund amount doesn't exceed original charge
	if req.AmountKurus > originalEntry.AmountKurus {
		return nil, ErrRefundAmountExceeded
	}

	// Create refund record in pending state
	now := time.Now().UTC()
	refund := &Refund{
		ID:                    uuid.New(),
		OrganizationID:        req.OrganizationID,
		OriginalLedgerEntryID: req.OriginalLedgerEntryID,
		AmountKurus:           req.AmountKurus,
		Reason:                req.Reason,
		Status:                domain.RefundStatusPending.String(),
		ReferenceType:         originalEntry.ReferenceType,
		ReferenceID:           originalEntry.ReferenceID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	s.logger.Info("Manual refund request created",
		"refund_id", refund.ID,
		"organization_id", req.OrganizationID,
		"original_entry_id", req.OriginalLedgerEntryID,
		"amount_kurus", req.AmountKurus,
	)

	return refund, nil
}

// ApproveRefundRequest represents a request to approve a refund.
type ApproveRefundRequest struct {
	RefundID   uuid.UUID
	ApproverID uuid.UUID
}

// Approve approves a pending refund and processes the credit.
func (s *RefundService) Approve(ctx context.Context, req *ApproveRefundRequest) (*CreateRefundResult, error) {
	// In a full implementation, this would:
	// 1. Load the refund from database
	// 2. Verify it's in pending state
	// 3. Create the credit ledger entry
	// 4. Update the refund status to completed

	// For now, this is a placeholder as we're using auto-approved refunds for shipment cancellations
	return nil, fmt.Errorf("manual refund approval not yet implemented")
}

// Get retrieves a refund by ID.
func (s *RefundService) Get(ctx context.Context, refundID, organizationID uuid.UUID) (*Refund, error) {
	// In a full implementation, this would query the refunds table
	// For now, we can reconstruct refund info from ledger entries
	return nil, ErrRefundNotFound
}

// GetByShipmentID retrieves a refund by shipment ID.
func (s *RefundService) GetByShipmentID(ctx context.Context, shipmentID uuid.UUID) (*Refund, error) {
	// Try to find a refund ledger entry for this shipment
	entry, err := s.ledgerService.GetEntryByReference(ctx, domain.ReferenceTypeRefund, shipmentID)
	if err != nil {
		if errors.Is(err, ErrLedgerEntryNotFound) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}

	// Reconstruct refund from ledger entry
	refund := &Refund{
		ID:                  entry.ID, // Use ledger entry ID as refund ID
		OrganizationID:      entry.OrganizationID,
		RefundLedgerEntryID: &entry.ID,
		AmountKurus:         entry.AmountKurus,
		Reason:              entry.Description,
		Status:              domain.RefundStatusCompleted.String(),
		ReferenceType:       domain.ReferenceTypeShipment.String(),
		ReferenceID:         shipmentID,
		CompletedAt:         &entry.CreatedAt,
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.CreatedAt,
	}

	return refund, nil
}

// ListRefundsRequest represents a request to list refunds.
type ListRefundsRequest struct {
	OrganizationID uuid.UUID
	Status         *string
	ReferenceType  *string
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int
	Limit          int
}

// ListRefundsResult represents the result of listing refunds.
type ListRefundsResult struct {
	Refunds []*Refund
	Total   int64
}

// List lists refunds for an organization.
func (s *RefundService) List(ctx context.Context, req *ListRefundsRequest) (*ListRefundsResult, error) {
	// Get refund ledger entries
	refType := domain.ReferenceTypeRefund
	listReq := &ListLedgerRequest{
		OrganizationID: req.OrganizationID,
		ReferenceType:  &refType,
		CreatedAfter:   req.CreatedAfter,
		CreatedBefore:  req.CreatedBefore,
		Offset:         req.Offset,
		Limit:          req.Limit,
	}

	result, err := s.ledgerService.List(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("failed to list refund entries: %w", err)
	}

	// Convert ledger entries to refunds
	refunds := make([]*Refund, 0, len(result.Entries))
	for _, entry := range result.Entries {
		refund := &Refund{
			ID:                  entry.ID,
			OrganizationID:      entry.OrganizationID,
			RefundLedgerEntryID: &entry.ID,
			AmountKurus:         entry.AmountKurus,
			Reason:              entry.Description,
			Status:              domain.RefundStatusCompleted.String(),
			ReferenceType:       entry.ReferenceType,
			ReferenceID:         entry.ReferenceID,
			CompletedAt:         &entry.CreatedAt,
			CreatedAt:           entry.CreatedAt,
			UpdatedAt:           entry.CreatedAt,
		}
		refunds = append(refunds, refund)
	}

	return &ListRefundsResult{
		Refunds: refunds,
		Total:   result.Total,
	}, nil
}

// GetRefundSummary retrieves a summary of refunds for an organization.
type RefundSummary struct {
	OrganizationID   uuid.UUID `json:"organization_id"`
	TotalRefunds     int64     `json:"total_refunds"`
	TotalAmountKurus int64     `json:"total_amount_kurus"`
	PendingCount     int64     `json:"pending_count"`
	CompletedCount   int64     `json:"completed_count"`
}

// GetSummary retrieves a summary of refunds for an organization.
func (s *RefundService) GetSummary(ctx context.Context, organizationID uuid.UUID) (*RefundSummary, error) {
	// Get all refund entries for summary
	refType := domain.ReferenceTypeRefund
	result, err := s.ledgerService.List(ctx, &ListLedgerRequest{
		OrganizationID: organizationID,
		ReferenceType:  &refType,
		Offset:         0,
		Limit:          10000, // Get all for summary
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get refund entries: %w", err)
	}

	var totalAmount int64
	for _, entry := range result.Entries {
		totalAmount += entry.AmountKurus
	}

	return &RefundSummary{
		OrganizationID:   organizationID,
		TotalRefunds:     result.Total,
		TotalAmountKurus: totalAmount,
		PendingCount:     0, // All auto-approved refunds are completed
		CompletedCount:   result.Total,
	}, nil
}

// ProcessShipmentCancellationRefund processes a refund for a cancelled shipment.
// This is a convenience method that wraps Create with shipment-specific logic.
func (s *RefundService) ProcessShipmentCancellationRefund(
	ctx context.Context,
	organizationID uuid.UUID,
	shipmentID uuid.UUID,
	amountKurus int64,
	trackingNumber string,
	reason string,
	actorID *uuid.UUID,
	actorType domain.ActorType,
) (*CreateRefundResult, error) {
	if reason == "" {
		reason = "Shipment cancelled"
	}

	fullReason := fmt.Sprintf("Refund for cancelled shipment %s: %s", trackingNumber, reason)

	return s.Create(ctx, &CreateRefundRequest{
		OrganizationID: organizationID,
		ShipmentID:     shipmentID,
		AmountKurus:    amountKurus,
		Reason:         fullReason,
		ActorID:        actorID,
		ActorType:      actorType,
	})
}

// Helper to convert pgtype.UUID to uuid.UUID pointer
func pgtypeUUIDToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

// Helper to convert pgtype.Timestamptz to time.Time pointer
func pgtypeTimestamptzToPtr(p pgtype.Timestamptz) *time.Time {
	if !p.Valid {
		return nil
	}
	return &p.Time
}
