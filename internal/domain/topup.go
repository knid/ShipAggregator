package domain

import (
	"time"

	"github.com/google/uuid"
)

// TopupStatus represents the status of a wallet top-up.
type TopupStatus string

const (
	// TopupStatusPending indicates the top-up is pending payment.
	TopupStatusPending TopupStatus = "pending"
	// TopupStatusProcessing indicates the payment is being processed.
	TopupStatusProcessing TopupStatus = "processing"
	// TopupStatusCompleted indicates the top-up is complete.
	TopupStatusCompleted TopupStatus = "completed"
	// TopupStatusFailed indicates the top-up failed.
	TopupStatusFailed TopupStatus = "failed"
	// TopupStatusCancelled indicates the top-up was cancelled.
	TopupStatusCancelled TopupStatus = "cancelled"
)

// IsValid checks if the top-up status is valid.
func (s TopupStatus) IsValid() bool {
	switch s {
	case TopupStatusPending, TopupStatusProcessing, TopupStatusCompleted,
		TopupStatusFailed, TopupStatusCancelled:
		return true
	default:
		return false
	}
}

// String returns the string representation of the top-up status.
func (s TopupStatus) String() string {
	return string(s)
}

// IsFinal returns true if the status is a final state.
func (s TopupStatus) IsFinal() bool {
	switch s {
	case TopupStatusCompleted, TopupStatusFailed, TopupStatusCancelled:
		return true
	default:
		return false
	}
}

// PaymentProvider represents a payment provider.
type PaymentProvider string

const (
	// PaymentProviderIyzico is the Iyzico payment provider.
	PaymentProviderIyzico PaymentProvider = "iyzico"
)

// IsValid checks if the payment provider is valid.
func (p PaymentProvider) IsValid() bool {
	switch p {
	case PaymentProviderIyzico:
		return true
	default:
		return false
	}
}

// String returns the string representation of the payment provider.
func (p PaymentProvider) String() string {
	return string(p)
}

// Topup represents a wallet funding request.
type Topup struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// IdempotencyKey is the client-provided idempotency key.
	IdempotencyKey string `json:"idempotency_key"`
	// AmountKurus is the top-up amount in kuruş.
	AmountKurus int64 `json:"amount_kurus"`
	// Currency is the currency code.
	Currency string `json:"currency"`
	// Status is the top-up status.
	Status TopupStatus `json:"status"`
	// PaymentProvider is the payment provider.
	PaymentProvider PaymentProvider `json:"payment_provider"`
	// PaymentID is the provider's payment ID.
	PaymentID *string `json:"payment_id,omitempty"`
	// PaymentURL is the payment form URL for redirect.
	PaymentURL *string `json:"payment_url,omitempty"`
	// PaymentToken is the provider token for callback verification.
	PaymentToken *string `json:"-"`
	// LedgerEntryID is the created ledger entry ID (after completion).
	LedgerEntryID *uuid.UUID `json:"ledger_entry_id,omitempty"`
	// FailureReason is the failure reason.
	FailureReason *string `json:"failure_reason,omitempty"`
	// CompletedAt is the completion timestamp.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// ExpiresAt is the payment expiration timestamp.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is the timestamp when the top-up was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the top-up was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTopup creates a new Topup.
func NewTopup(organizationID uuid.UUID, idempotencyKey string, amountKurus int64, expiresIn time.Duration) *Topup {
	now := time.Now().UTC()
	return &Topup{
		ID:              uuid.New(),
		OrganizationID:  organizationID,
		IdempotencyKey:  idempotencyKey,
		AmountKurus:     amountKurus,
		Currency:        "TRY",
		Status:          TopupStatusPending,
		PaymentProvider: PaymentProviderIyzico,
		ExpiresAt:       now.Add(expiresIn),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// IsPending returns true if the top-up is pending.
func (t *Topup) IsPending() bool {
	return t.Status == TopupStatusPending
}

// IsProcessing returns true if the top-up is processing.
func (t *Topup) IsProcessing() bool {
	return t.Status == TopupStatusProcessing
}

// IsCompleted returns true if the top-up is completed.
func (t *Topup) IsCompleted() bool {
	return t.Status == TopupStatusCompleted
}

// IsFailed returns true if the top-up failed.
func (t *Topup) IsFailed() bool {
	return t.Status == TopupStatusFailed
}

// IsCancelled returns true if the top-up was cancelled.
func (t *Topup) IsCancelled() bool {
	return t.Status == TopupStatusCancelled
}

// IsExpired returns true if the top-up has expired.
func (t *Topup) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

// IsFinal returns true if the top-up is in a final state.
func (t *Topup) IsFinal() bool {
	return t.Status.IsFinal()
}

// CanBeProcessed returns true if the top-up can be processed.
func (t *Topup) CanBeProcessed() bool {
	return t.IsPending() && !t.IsExpired()
}

// CanBeConfirmed returns true if the top-up can be confirmed.
func (t *Topup) CanBeConfirmed() bool {
	return (t.IsPending() || t.IsProcessing()) && !t.IsExpired()
}

// CanBeCancelled returns true if the top-up can be cancelled.
func (t *Topup) CanBeCancelled() bool {
	return t.IsPending() && !t.IsExpired()
}

// SetPaymentDetails sets the payment details from the provider.
func (t *Topup) SetPaymentDetails(paymentID, paymentURL, paymentToken string) {
	t.PaymentID = &paymentID
	t.PaymentURL = &paymentURL
	t.PaymentToken = &paymentToken
	t.Status = TopupStatusProcessing
	t.UpdatedAt = time.Now().UTC()
}

// Complete completes the top-up with the ledger entry.
func (t *Topup) Complete(ledgerEntryID uuid.UUID) {
	now := time.Now().UTC()
	t.Status = TopupStatusCompleted
	t.LedgerEntryID = &ledgerEntryID
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// Fail marks the top-up as failed.
func (t *Topup) Fail(reason string) {
	t.Status = TopupStatusFailed
	t.FailureReason = &reason
	t.UpdatedAt = time.Now().UTC()
}

// Cancel cancels the top-up.
func (t *Topup) Cancel() {
	t.Status = TopupStatusCancelled
	t.UpdatedAt = time.Now().UTC()
}

// AmountTRY returns the amount in TRY (lira).
func (t *Topup) AmountTRY() float64 {
	return float64(t.AmountKurus) / 100.0
}

// TopupRequest represents a request to create a top-up.
type TopupRequest struct {
	// OrganizationID is the organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// IdempotencyKey is the client-provided idempotency key.
	IdempotencyKey string `json:"idempotency_key"`
	// AmountKurus is the top-up amount in kuruş.
	AmountKurus int64 `json:"amount_kurus"`
	// ReturnURL is the URL to redirect after payment.
	ReturnURL string `json:"return_url"`
	// CallbackURL is the URL for payment callbacks.
	CallbackURL string `json:"callback_url"`
}

// Validate validates the top-up request.
func (r *TopupRequest) Validate() error {
	if r.OrganizationID == uuid.Nil {
		return ErrInvalidOrganizationID
	}
	if r.IdempotencyKey == "" {
		return ErrIdempotencyKeyRequired
	}
	if r.AmountKurus <= 0 {
		return ErrInvalidAmount
	}
	if r.AmountKurus < MinTopupAmountKurus {
		return ErrAmountBelowMinimum
	}
	if r.AmountKurus > MaxTopupAmountKurus {
		return ErrAmountAboveMaximum
	}
	if r.ReturnURL == "" {
		return ErrReturnURLRequired
	}
	return nil
}

// MinTopupAmountKurus is the minimum top-up amount in kuruş (10 TRY).
const MinTopupAmountKurus int64 = 1000

// MaxTopupAmountKurus is the maximum top-up amount in kuruş (100,000 TRY).
const MaxTopupAmountKurus int64 = 10000000

// DefaultTopupExpirationDuration is the default expiration duration for top-ups.
const DefaultTopupExpirationDuration = 30 * time.Minute

// TopupConfirmation represents a top-up confirmation from the payment provider.
type TopupConfirmation struct {
	// TopupID is the top-up ID.
	TopupID uuid.UUID `json:"topup_id"`
	// PaymentID is the provider's payment ID.
	PaymentID string `json:"payment_id"`
	// PaymentToken is the provider token for verification.
	PaymentToken string `json:"payment_token"`
	// Status is the payment status from the provider.
	Status string `json:"status"`
	// ErrorCode is the error code (if failed).
	ErrorCode *string `json:"error_code,omitempty"`
	// ErrorMessage is the error message (if failed).
	ErrorMessage *string `json:"error_message,omitempty"`
}

// IsSuccess returns true if the confirmation indicates success.
func (c *TopupConfirmation) IsSuccess() bool {
	return c.Status == "success" || c.Status == "completed"
}

// GetFailureReason returns the failure reason.
func (c *TopupConfirmation) GetFailureReason() string {
	if c.ErrorMessage != nil {
		return *c.ErrorMessage
	}
	if c.ErrorCode != nil {
		return "Payment failed with code: " + *c.ErrorCode
	}
	return "Payment failed"
}
