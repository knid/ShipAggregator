package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EntryType represents the type of a ledger entry.
type EntryType string

const (
	// EntryTypeCredit indicates a credit (add funds) entry.
	EntryTypeCredit EntryType = "credit"
	// EntryTypeDebit indicates a debit (subtract funds) entry.
	EntryTypeDebit EntryType = "debit"
)

// IsValid checks if the entry type is valid.
func (t EntryType) IsValid() bool {
	switch t {
	case EntryTypeCredit, EntryTypeDebit:
		return true
	default:
		return false
	}
}

// String returns the string representation of the entry type.
func (t EntryType) String() string {
	return string(t)
}

// ReferenceType represents the type of reference for a ledger entry.
type ReferenceType string

const (
	// ReferenceTypeTopup indicates a wallet top-up.
	ReferenceTypeTopup ReferenceType = "topup"
	// ReferenceTypeShipment indicates a shipment charge.
	ReferenceTypeShipment ReferenceType = "shipment"
	// ReferenceTypeRefund indicates a refund.
	ReferenceTypeRefund ReferenceType = "refund"
	// ReferenceTypeCODSettlement indicates a COD settlement.
	ReferenceTypeCODSettlement ReferenceType = "cod_settlement"
	// ReferenceTypeAdjustment indicates a manual adjustment.
	ReferenceTypeAdjustment ReferenceType = "adjustment"
	// ReferenceTypePromotion indicates a promotional credit.
	ReferenceTypePromotion ReferenceType = "promotion"
)

// IsValid checks if the reference type is valid.
func (t ReferenceType) IsValid() bool {
	switch t {
	case ReferenceTypeTopup, ReferenceTypeShipment, ReferenceTypeRefund,
		ReferenceTypeCODSettlement, ReferenceTypeAdjustment, ReferenceTypePromotion:
		return true
	default:
		return false
	}
}

// String returns the string representation of the reference type.
func (t ReferenceType) String() string {
	return string(t)
}

// ActorType represents the type of actor that created a ledger entry.
type ActorType string

const (
	// ActorTypeMember indicates a member (user) created the entry.
	ActorTypeMember ActorType = "member"
	// ActorTypeAPIKey indicates an API key created the entry.
	ActorTypeAPIKey ActorType = "api_key"
	// ActorTypeSystem indicates the system created the entry.
	ActorTypeSystem ActorType = "system"
)

// IsValid checks if the actor type is valid.
func (t ActorType) IsValid() bool {
	switch t {
	case ActorTypeMember, ActorTypeAPIKey, ActorTypeSystem:
		return true
	default:
		return false
	}
}

// String returns the string representation of the actor type.
func (t ActorType) String() string {
	return string(t)
}

// LedgerEntry represents an immutable financial transaction log entry.
type LedgerEntry struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// EntryType is the entry type (credit or debit).
	EntryType EntryType `json:"entry_type"`
	// AmountKurus is the amount in kuruş (always positive).
	AmountKurus int64 `json:"amount_kurus"`
	// Currency is the currency code (default: TRY).
	Currency string `json:"currency"`
	// BalanceAfterKurus is the running balance after this entry.
	BalanceAfterKurus int64 `json:"balance_after_kurus"`
	// ReferenceType is the type of reference.
	ReferenceType ReferenceType `json:"reference_type"`
	// ReferenceID is the reference record ID.
	ReferenceID uuid.UUID `json:"reference_id"`
	// Description is the human-readable description.
	Description string `json:"description"`
	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// ActorID is the ID of the user/API key that created the entry.
	ActorID *uuid.UUID `json:"actor_id,omitempty"`
	// ActorType is the type of actor.
	ActorType ActorType `json:"actor_type"`
	// CreatedAt is the timestamp when the entry was created (immutable).
	CreatedAt time.Time `json:"created_at"`
}

// NewCreditEntry creates a new credit ledger entry.
func NewCreditEntry(
	organizationID uuid.UUID,
	amountKurus int64,
	balanceAfterKurus int64,
	referenceType ReferenceType,
	referenceID uuid.UUID,
	description string,
	actorID *uuid.UUID,
	actorType ActorType,
) *LedgerEntry {
	return &LedgerEntry{
		ID:                uuid.New(),
		OrganizationID:    organizationID,
		EntryType:         EntryTypeCredit,
		AmountKurus:       amountKurus,
		Currency:          "TRY",
		BalanceAfterKurus: balanceAfterKurus,
		ReferenceType:     referenceType,
		ReferenceID:       referenceID,
		Description:       description,
		Metadata:          make(map[string]interface{}),
		ActorID:           actorID,
		ActorType:         actorType,
		CreatedAt:         time.Now().UTC(),
	}
}

// NewDebitEntry creates a new debit ledger entry.
func NewDebitEntry(
	organizationID uuid.UUID,
	amountKurus int64,
	balanceAfterKurus int64,
	referenceType ReferenceType,
	referenceID uuid.UUID,
	description string,
	actorID *uuid.UUID,
	actorType ActorType,
) *LedgerEntry {
	return &LedgerEntry{
		ID:                uuid.New(),
		OrganizationID:    organizationID,
		EntryType:         EntryTypeDebit,
		AmountKurus:       amountKurus,
		Currency:          "TRY",
		BalanceAfterKurus: balanceAfterKurus,
		ReferenceType:     referenceType,
		ReferenceID:       referenceID,
		Description:       description,
		Metadata:          make(map[string]interface{}),
		ActorID:           actorID,
		ActorType:         actorType,
		CreatedAt:         time.Now().UTC(),
	}
}

// IsCredit returns true if this is a credit entry.
func (e *LedgerEntry) IsCredit() bool {
	return e.EntryType == EntryTypeCredit
}

// IsDebit returns true if this is a debit entry.
func (e *LedgerEntry) IsDebit() bool {
	return e.EntryType == EntryTypeDebit
}

// SetMetadata sets a metadata value.
func (e *LedgerEntry) SetMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
}

// GetMetadata gets a metadata value.
func (e *LedgerEntry) GetMetadata(key string) (interface{}, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	v, ok := e.Metadata[key]
	return v, ok
}

// MarshalMetadataJSON marshals the metadata to JSON.
func (e *LedgerEntry) MarshalMetadataJSON() (json.RawMessage, error) {
	if e.Metadata == nil {
		return json.RawMessage("{}"), nil
	}
	return json.Marshal(e.Metadata)
}

// UnmarshalMetadataJSON unmarshals the metadata from JSON.
func (e *LedgerEntry) UnmarshalMetadataJSON(data json.RawMessage) error {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		e.Metadata = make(map[string]interface{})
		return nil
	}
	return json.Unmarshal(data, &e.Metadata)
}

// WalletBalance represents the current wallet balance for an organization.
type WalletBalance struct {
	// OrganizationID is the organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// BalanceKurus is the current balance in kuruş.
	BalanceKurus int64 `json:"balance_kurus"`
	// Currency is the currency code.
	Currency string `json:"currency"`
	// LastTransactionAt is the timestamp of the last transaction.
	LastTransactionAt *time.Time `json:"last_transaction_at,omitempty"`
}

// NewWalletBalance creates a new WalletBalance.
func NewWalletBalance(organizationID uuid.UUID, balanceKurus int64) *WalletBalance {
	return &WalletBalance{
		OrganizationID: organizationID,
		BalanceKurus:   balanceKurus,
		Currency:       "TRY",
	}
}

// HasSufficientBalance checks if the wallet has sufficient balance.
func (w *WalletBalance) HasSufficientBalance(amountKurus int64) bool {
	return w.BalanceKurus >= amountKurus
}

// BalanceTRY returns the balance in TRY (lira).
func (w *WalletBalance) BalanceTRY() float64 {
	return float64(w.BalanceKurus) / 100.0
}

// LedgerSummary represents a summary of ledger entries for a period.
type LedgerSummary struct {
	// OrganizationID is the organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// PeriodStart is the start of the period.
	PeriodStart time.Time `json:"period_start"`
	// PeriodEnd is the end of the period.
	PeriodEnd time.Time `json:"period_end"`
	// TotalCreditsKurus is the total credits in kuruş.
	TotalCreditsKurus int64 `json:"total_credits_kurus"`
	// TotalDebitsKurus is the total debits in kuruş.
	TotalDebitsKurus int64 `json:"total_debits_kurus"`
	// NetChangeKurus is the net change (credits - debits) in kuruş.
	NetChangeKurus int64 `json:"net_change_kurus"`
	// OpeningBalanceKurus is the opening balance in kuruş.
	OpeningBalanceKurus int64 `json:"opening_balance_kurus"`
	// ClosingBalanceKurus is the closing balance in kuruş.
	ClosingBalanceKurus int64 `json:"closing_balance_kurus"`
	// TransactionCount is the number of transactions.
	TransactionCount int `json:"transaction_count"`
}

// CalculateNetChange calculates the net change.
func (s *LedgerSummary) CalculateNetChange() {
	s.NetChangeKurus = s.TotalCreditsKurus - s.TotalDebitsKurus
}

// CalculateClosingBalance calculates the closing balance.
func (s *LedgerSummary) CalculateClosingBalance() {
	s.ClosingBalanceKurus = s.OpeningBalanceKurus + s.NetChangeKurus
}

// Refund represents a refund record.
type Refund struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// OriginalLedgerEntryID is the original charge ledger entry ID.
	OriginalLedgerEntryID uuid.UUID `json:"original_ledger_entry_id"`
	// RefundLedgerEntryID is the refund credit ledger entry ID (after completion).
	RefundLedgerEntryID *uuid.UUID `json:"refund_ledger_entry_id,omitempty"`
	// AmountKurus is the refund amount in kuruş.
	AmountKurus int64 `json:"amount_kurus"`
	// Reason is the refund reason.
	Reason string `json:"reason"`
	// Status is the refund status.
	Status RefundStatus `json:"status"`
	// ApprovedBy is the member who approved the refund.
	ApprovedBy *uuid.UUID `json:"approved_by,omitempty"`
	// ApprovedAt is the approval timestamp.
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	// CompletedAt is the completion timestamp.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// CreatedAt is the timestamp when the refund was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the refund was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// RefundStatus represents the status of a refund.
type RefundStatus string

const (
	// RefundStatusPending indicates the refund is pending approval.
	RefundStatusPending RefundStatus = "pending"
	// RefundStatusApproved indicates the refund has been approved.
	RefundStatusApproved RefundStatus = "approved"
	// RefundStatusCompleted indicates the refund has been completed.
	RefundStatusCompleted RefundStatus = "completed"
	// RefundStatusRejected indicates the refund was rejected.
	RefundStatusRejected RefundStatus = "rejected"
)

// IsValid checks if the refund status is valid.
func (s RefundStatus) IsValid() bool {
	switch s {
	case RefundStatusPending, RefundStatusApproved, RefundStatusCompleted, RefundStatusRejected:
		return true
	default:
		return false
	}
}

// String returns the string representation of the refund status.
func (s RefundStatus) String() string {
	return string(s)
}

// NewRefund creates a new Refund.
func NewRefund(organizationID, originalLedgerEntryID uuid.UUID, amountKurus int64, reason string) *Refund {
	now := time.Now().UTC()
	return &Refund{
		ID:                    uuid.New(),
		OrganizationID:        organizationID,
		OriginalLedgerEntryID: originalLedgerEntryID,
		AmountKurus:           amountKurus,
		Reason:                reason,
		Status:                RefundStatusPending,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

// IsPending returns true if the refund is pending.
func (r *Refund) IsPending() bool {
	return r.Status == RefundStatusPending
}

// IsApproved returns true if the refund is approved.
func (r *Refund) IsApproved() bool {
	return r.Status == RefundStatusApproved
}

// IsCompleted returns true if the refund is completed.
func (r *Refund) IsCompleted() bool {
	return r.Status == RefundStatusCompleted
}

// IsRejected returns true if the refund was rejected.
func (r *Refund) IsRejected() bool {
	return r.Status == RefundStatusRejected
}

// Approve approves the refund.
func (r *Refund) Approve(approvedBy uuid.UUID) {
	now := time.Now().UTC()
	r.Status = RefundStatusApproved
	r.ApprovedBy = &approvedBy
	r.ApprovedAt = &now
	r.UpdatedAt = now
}

// Complete completes the refund with the ledger entry.
func (r *Refund) Complete(ledgerEntryID uuid.UUID) {
	now := time.Now().UTC()
	r.Status = RefundStatusCompleted
	r.RefundLedgerEntryID = &ledgerEntryID
	r.CompletedAt = &now
	r.UpdatedAt = now
}

// Reject rejects the refund.
func (r *Refund) Reject() {
	r.Status = RefundStatusRejected
	r.UpdatedAt = time.Now().UTC()
}

// CODSettlement represents a COD collection tracking record.
type CODSettlement struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// ShipmentID is the related shipment ID.
	ShipmentID uuid.UUID `json:"shipment_id"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// ExpectedAmountKurus is the expected COD amount in kuruş.
	ExpectedAmountKurus int64 `json:"expected_amount_kurus"`
	// ActualAmountKurus is the actual collected amount in kuruş.
	ActualAmountKurus *int64 `json:"actual_amount_kurus,omitempty"`
	// CarrierSettlementID is the carrier's settlement reference.
	CarrierSettlementID *string `json:"carrier_settlement_id,omitempty"`
	// CarrierSettlementDate is the carrier settlement date.
	CarrierSettlementDate *time.Time `json:"carrier_settlement_date,omitempty"`
	// Status is the settlement status.
	Status CODSettlementStatus `json:"status"`
	// LedgerEntryID is the credit ledger entry ID (after settlement).
	LedgerEntryID *uuid.UUID `json:"ledger_entry_id,omitempty"`
	// DiscrepancyKurus is the amount discrepancy (actual - expected).
	DiscrepancyKurus *int64 `json:"discrepancy_kurus,omitempty"`
	// DiscrepancyReason is the explanation for discrepancy.
	DiscrepancyReason *string `json:"discrepancy_reason,omitempty"`
	// SettledAt is the settlement timestamp.
	SettledAt *time.Time `json:"settled_at,omitempty"`
	// CreatedAt is the timestamp when the record was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the record was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// CODSettlementStatus represents the status of a COD settlement.
type CODSettlementStatus string

const (
	// CODSettlementStatusPending indicates the settlement is pending.
	CODSettlementStatusPending CODSettlementStatus = "pending"
	// CODSettlementStatusCollected indicates the COD has been collected.
	CODSettlementStatusCollected CODSettlementStatus = "collected"
	// CODSettlementStatusSettled indicates the settlement is complete.
	CODSettlementStatusSettled CODSettlementStatus = "settled"
	// CODSettlementStatusDisputed indicates there is a dispute.
	CODSettlementStatusDisputed CODSettlementStatus = "disputed"
	// CODSettlementStatusWrittenOff indicates the settlement was written off.
	CODSettlementStatusWrittenOff CODSettlementStatus = "written_off"
)

// IsValid checks if the COD settlement status is valid.
func (s CODSettlementStatus) IsValid() bool {
	switch s {
	case CODSettlementStatusPending, CODSettlementStatusCollected,
		CODSettlementStatusSettled, CODSettlementStatusDisputed,
		CODSettlementStatusWrittenOff:
		return true
	default:
		return false
	}
}

// String returns the string representation of the COD settlement status.
func (s CODSettlementStatus) String() string {
	return string(s)
}

// NewCODSettlement creates a new CODSettlement.
func NewCODSettlement(organizationID, shipmentID uuid.UUID, providerCode ProviderCode, expectedAmountKurus int64) *CODSettlement {
	now := time.Now().UTC()
	return &CODSettlement{
		ID:                  uuid.New(),
		OrganizationID:      organizationID,
		ShipmentID:          shipmentID,
		ProviderCode:        providerCode,
		ExpectedAmountKurus: expectedAmountKurus,
		Status:              CODSettlementStatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// IsPending returns true if the settlement is pending.
func (s *CODSettlement) IsPending() bool {
	return s.Status == CODSettlementStatusPending
}

// IsCollected returns true if the COD has been collected.
func (s *CODSettlement) IsCollected() bool {
	return s.Status == CODSettlementStatusCollected
}

// IsSettled returns true if the settlement is complete.
func (s *CODSettlement) IsSettled() bool {
	return s.Status == CODSettlementStatusSettled
}

// IsDisputed returns true if there is a dispute.
func (s *CODSettlement) IsDisputed() bool {
	return s.Status == CODSettlementStatusDisputed
}

// IsWrittenOff returns true if the settlement was written off.
func (s *CODSettlement) IsWrittenOff() bool {
	return s.Status == CODSettlementStatusWrittenOff
}

// MarkCollected marks the COD as collected.
func (s *CODSettlement) MarkCollected(actualAmountKurus int64) {
	s.ActualAmountKurus = &actualAmountKurus
	s.Status = CODSettlementStatusCollected
	s.UpdatedAt = time.Now().UTC()

	// Calculate discrepancy
	discrepancy := actualAmountKurus - s.ExpectedAmountKurus
	if discrepancy != 0 {
		s.DiscrepancyKurus = &discrepancy
	}
}

// Settle settles the COD with the ledger entry.
func (s *CODSettlement) Settle(ledgerEntryID uuid.UUID, carrierSettlementID string, carrierSettlementDate time.Time) {
	now := time.Now().UTC()
	s.Status = CODSettlementStatusSettled
	s.LedgerEntryID = &ledgerEntryID
	s.CarrierSettlementID = &carrierSettlementID
	s.CarrierSettlementDate = &carrierSettlementDate
	s.SettledAt = &now
	s.UpdatedAt = now
}

// Dispute marks the settlement as disputed.
func (s *CODSettlement) Dispute(reason string) {
	s.Status = CODSettlementStatusDisputed
	s.DiscrepancyReason = &reason
	s.UpdatedAt = time.Now().UTC()
}

// WriteOff writes off the settlement.
func (s *CODSettlement) WriteOff(reason string) {
	s.Status = CODSettlementStatusWrittenOff
	s.DiscrepancyReason = &reason
	s.UpdatedAt = time.Now().UTC()
}

// HasDiscrepancy returns true if there is a discrepancy.
func (s *CODSettlement) HasDiscrepancy() bool {
	return s.DiscrepancyKurus != nil && *s.DiscrepancyKurus != 0
}

// GetSettlementAmount returns the amount to settle (actual or expected).
func (s *CODSettlement) GetSettlementAmount() int64 {
	if s.ActualAmountKurus != nil {
		return *s.ActualAmountKurus
	}
	return s.ExpectedAmountKurus
}
