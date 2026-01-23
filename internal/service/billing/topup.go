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

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/payment/iyzico"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// Common errors for topup operations.
var (
	ErrTopupNotFound           = errors.New("topup not found")
	ErrTopupExpired            = errors.New("topup has expired")
	ErrTopupAlreadyCompleted   = errors.New("topup already completed")
	ErrTopupInvalidState       = errors.New("topup is in invalid state for this operation")
	ErrTopupAmountTooLow       = errors.New("topup amount is below minimum")
	ErrTopupAmountTooHigh      = errors.New("topup amount exceeds maximum")
	ErrTopupDailyLimitExceeded = errors.New("daily topup limit exceeded")
	ErrTopupRateLimitExceeded  = errors.New("topup rate limit exceeded")
	ErrPaymentInitFailed       = errors.New("payment initialization failed")
	ErrPaymentConfirmFailed    = errors.New("payment confirmation failed")
)

// TopupConfig holds configuration for the topup service.
type TopupConfig struct {
	// MinAmountKurus is the minimum topup amount in kuruş.
	MinAmountKurus int64
	// MaxAmountKurus is the maximum topup amount in kuruş.
	MaxAmountKurus int64
	// MaxDailyAmountKurus is the maximum daily topup amount in kuruş.
	MaxDailyAmountKurus int64
	// MaxTopupsPerHour is the maximum number of topups per hour.
	MaxTopupsPerHour int
	// ExpirationDuration is the duration before a topup expires.
	ExpirationDuration time.Duration
	// CallbackURL is the URL for Iyzico callbacks.
	CallbackURL string
}

// DefaultTopupConfig returns the default topup configuration.
func DefaultTopupConfig() TopupConfig {
	return TopupConfig{
		MinAmountKurus:      1000,     // 10 TRY
		MaxAmountKurus:      5000000,  // 50,000 TRY
		MaxDailyAmountKurus: 10000000, // 100,000 TRY
		MaxTopupsPerHour:    10,
		ExpirationDuration:  60 * time.Minute,
	}
}

// TopupService provides topup operations.
type TopupService struct {
	queries       *db.Queries
	iyzicoClient  *iyzico.Client
	ledgerService *LedgerService
	config        TopupConfig
	logger        *observability.Logger
}

// NewTopupService creates a new topup service.
func NewTopupService(
	queries *db.Queries,
	iyzicoClient *iyzico.Client,
	ledgerService *LedgerService,
	config TopupConfig,
	logger *observability.Logger,
) *TopupService {
	return &TopupService{
		queries:       queries,
		iyzicoClient:  iyzicoClient,
		ledgerService: ledgerService,
		config:        config,
		logger:        logger,
	}
}

// CreateTopupRequest represents a request to create a topup.
type CreateTopupRequest struct {
	OrganizationID uuid.UUID
	AmountKurus    int64
	ReturnURL      string
	IdempotencyKey string
	// Organization details for Iyzico
	OrgName    string
	OrgEmail   string
	OrgPhone   string
	OrgAddress string
	OrgCity    string
	ClientIP   string
}

// CreateTopupResult represents the result of creating a topup.
type CreateTopupResult struct {
	Topup      *Topup
	PaymentURL string
}

// Topup represents a topup record.
type Topup struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	IdempotencyKey  string     `json:"idempotency_key"`
	AmountKurus     int64      `json:"amount_kurus"`
	Currency        string     `json:"currency"`
	Status          string     `json:"status"`
	PaymentProvider string     `json:"payment_provider"`
	PaymentID       *string    `json:"payment_id,omitempty"`
	PaymentURL      *string    `json:"payment_url,omitempty"`
	PaymentToken    *string    `json:"-"`
	LedgerEntryID   *uuid.UUID `json:"ledger_entry_id,omitempty"`
	FailureReason   *string    `json:"failure_reason,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Create creates a new topup and initializes payment with Iyzico.
func (s *TopupService) Create(ctx context.Context, req *CreateTopupRequest) (*CreateTopupResult, error) {
	// Validate amount
	if req.AmountKurus < s.config.MinAmountKurus {
		return nil, ErrTopupAmountTooLow
	}
	if req.AmountKurus > s.config.MaxAmountKurus {
		return nil, ErrTopupAmountTooHigh
	}

	// Check for existing topup with same idempotency key
	existing, err := s.queries.GetTopupByIdempotencyKey(ctx, db.GetTopupByIdempotencyKeyParams{
		OrganizationID: req.OrganizationID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err == nil {
		// Return existing topup
		topup := dbTopupToTopup(existing)
		return &CreateTopupResult{
			Topup:      topup,
			PaymentURL: stringPtrValue(topup.PaymentURL),
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing topup: %w", err)
	}

	// Create topup record
	expiresAt := time.Now().UTC().Add(s.config.ExpirationDuration)
	dbTopup, err := s.queries.CreateTopup(ctx, db.CreateTopupParams{
		OrganizationID:  req.OrganizationID,
		IdempotencyKey:  req.IdempotencyKey,
		AmountKurus:     int32(req.AmountKurus),
		Currency:        "TRY",
		Status:          string(domain.TopupStatusPending),
		PaymentProvider: string(domain.PaymentProviderIyzico),
		ExpiresAt:       pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create topup: %w", err)
	}

	// Initialize Iyzico checkout form
	priceStr := fmt.Sprintf("%.2f", float64(req.AmountKurus)/100.0)

	buyer := iyzico.NewBuyerFromOrganization(
		req.OrganizationID.String(),
		req.OrgName,
		req.OrgEmail,
		req.OrgPhone,
		req.OrgAddress,
		req.OrgCity,
		req.ClientIP,
	)

	billingAddress := iyzico.NewAddressFromOrganization(
		req.OrgName,
		req.OrgAddress,
		req.OrgCity,
	)

	basketItem := iyzico.NewWalletTopupBasketItem(priceStr)

	iyzicoReq := iyzico.NewCheckoutFormRequest(
		dbTopup.ID.String(),
		priceStr,
		iyzico.CurrencyTRY,
		dbTopup.ID.String(),
		s.config.CallbackURL,
		buyer,
		billingAddress,
		[]iyzico.BasketItem{basketItem},
	)

	iyzicoResp, err := s.iyzicoClient.InitializeCheckoutForm(ctx, iyzicoReq)
	if err != nil {
		// Mark topup as failed
		failureReason := "Payment initialization failed"
		if iyzicoErr := iyzico.GetIyzicoError(err); iyzicoErr != nil {
			failureReason = iyzicoErr.Message
		}
		_, _ = s.queries.UpdateTopupFailed(ctx, db.UpdateTopupFailedParams{
			ID:            dbTopup.ID,
			FailureReason: &failureReason,
		})
		return nil, fmt.Errorf("%w: %v", ErrPaymentInitFailed, err)
	}

	// Update topup with payment details
	paymentURL := iyzicoResp.GetPaymentURL()
	token := iyzicoResp.GetToken()

	updatedTopup, err := s.queries.UpdateTopupPaymentInitiated(ctx, db.UpdateTopupPaymentInitiatedParams{
		ID:           dbTopup.ID,
		PaymentID:    nil, // Will be set after payment completion
		PaymentUrl:   &paymentURL,
		PaymentToken: &token,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update topup with payment details: %w", err)
	}

	s.logger.Info("Topup created and payment initialized",
		"topup_id", dbTopup.ID,
		"organization_id", req.OrganizationID,
		"amount_kurus", req.AmountKurus,
	)

	topup := dbTopupToTopup(updatedTopup)
	return &CreateTopupResult{
		Topup:      topup,
		PaymentURL: paymentURL,
	}, nil
}

// ConfirmTopupRequest represents a request to confirm a topup.
type ConfirmTopupRequest struct {
	TopupID      uuid.UUID
	PaymentToken string
}

// Confirm confirms a topup after payment completion.
func (s *TopupService) Confirm(ctx context.Context, req *ConfirmTopupRequest) (*Topup, error) {
	// Get topup by ID or token
	var dbTopup *db.Topup
	var err error

	if req.TopupID != uuid.Nil {
		dbTopup, err = s.queries.GetTopupByID(ctx, req.TopupID)
	} else if req.PaymentToken != "" {
		dbTopup, err = s.queries.GetTopupByPaymentToken(ctx, &req.PaymentToken)
	} else {
		return nil, ErrTopupNotFound
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTopupNotFound
		}
		return nil, fmt.Errorf("failed to get topup: %w", err)
	}

	// Check if already completed
	if dbTopup.Status == string(domain.TopupStatusCompleted) {
		return dbTopupToTopup(dbTopup), nil
	}

	// Check if in valid state
	if dbTopup.Status != string(domain.TopupStatusProcessing) {
		return nil, ErrTopupInvalidState
	}

	// Check if expired
	if dbTopup.ExpiresAt.Valid && time.Now().UTC().After(dbTopup.ExpiresAt.Time) {
		_, _ = s.queries.UpdateTopupFailed(ctx, db.UpdateTopupFailedParams{
			ID:            dbTopup.ID,
			FailureReason: stringPtr("Payment expired"),
		})
		return nil, ErrTopupExpired
	}

	// Retrieve payment result from Iyzico
	token := stringPtrValue(dbTopup.PaymentToken)
	if token == "" {
		return nil, fmt.Errorf("topup has no payment token")
	}

	iyzicoResp, err := s.iyzicoClient.RetrieveCheckoutFormResult(ctx, &iyzico.CheckoutFormRetrieveRequest{
		BaseRequest: iyzico.BaseRequest{
			Locale:         iyzico.LocaleTR,
			ConversationID: dbTopup.ID.String(),
		},
		Token: token,
	})
	if err != nil {
		// Check if it's an Iyzico error
		if iyzicoErr := iyzico.GetIyzicoError(err); iyzicoErr != nil {
			failureReason := iyzico.MapErrorToUserMessage(err)
			_, _ = s.queries.UpdateTopupFailed(ctx, db.UpdateTopupFailedParams{
				ID:            dbTopup.ID,
				FailureReason: &failureReason,
			})
			return nil, fmt.Errorf("%w: %s", ErrPaymentConfirmFailed, failureReason)
		}
		return nil, fmt.Errorf("failed to retrieve payment result: %w", err)
	}

	// Check payment status
	if !iyzicoResp.IsPaymentSuccess() {
		failureReason := "Payment was not successful"
		if iyzicoResp.ErrorMessage != "" {
			failureReason = iyzicoResp.ErrorMessage
		}
		_, _ = s.queries.UpdateTopupFailed(ctx, db.UpdateTopupFailedParams{
			ID:            dbTopup.ID,
			FailureReason: &failureReason,
		})
		return nil, fmt.Errorf("%w: %s", ErrPaymentConfirmFailed, failureReason)
	}

	// Check for fraud
	if iyzicoResp.IsFraudSuspected() {
		failureReason := "Payment flagged for fraud review"
		_, _ = s.queries.UpdateTopupFailed(ctx, db.UpdateTopupFailedParams{
			ID:            dbTopup.ID,
			FailureReason: &failureReason,
		})
		s.logger.Warn("Topup flagged for fraud",
			"topup_id", dbTopup.ID,
			"payment_id", iyzicoResp.PaymentID,
		)
		return nil, fmt.Errorf("%w: %s", ErrPaymentConfirmFailed, failureReason)
	}

	// Create ledger entry
	ledgerEntry, err := s.ledgerService.Credit(ctx, &CreditRequest{
		OrganizationID: dbTopup.OrganizationID,
		AmountKurus:    int64(dbTopup.AmountKurus),
		ReferenceType:  domain.ReferenceTypeTopup,
		ReferenceID:    dbTopup.ID,
		Description:    "Wallet top-up via Iyzico",
		Metadata: map[string]interface{}{
			"iyzico_payment_id": iyzicoResp.PaymentID,
			"card_last_four":    iyzicoResp.LastFourDigits,
			"card_type":         iyzicoResp.CardAssociation,
		},
		ActorType: domain.ActorTypeSystem,
	})
	if err != nil {
		s.logger.Error("Failed to create ledger entry for topup",
			"topup_id", dbTopup.ID,
			"error", err,
		)
		// Don't fail the topup - payment was successful
		// This should be handled by reconciliation
	}

	// Update topup as completed
	var ledgerEntryID pgtype.UUID
	if ledgerEntry != nil {
		ledgerEntryID = pgtype.UUID{Bytes: ledgerEntry.ID, Valid: true}
	}

	completedTopup, err := s.queries.UpdateTopupCompleted(ctx, db.UpdateTopupCompletedParams{
		ID:            dbTopup.ID,
		LedgerEntryID: ledgerEntryID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update topup as completed: %w", err)
	}

	s.logger.Info("Topup completed successfully",
		"topup_id", dbTopup.ID,
		"organization_id", dbTopup.OrganizationID,
		"amount_kurus", dbTopup.AmountKurus,
		"payment_id", iyzicoResp.PaymentID,
	)

	return dbTopupToTopup(completedTopup), nil
}

// ConfirmByToken confirms a topup using the payment token (for callbacks).
func (s *TopupService) ConfirmByToken(ctx context.Context, token string) (*Topup, error) {
	return s.Confirm(ctx, &ConfirmTopupRequest{
		PaymentToken: token,
	})
}

// Get retrieves a topup by ID.
func (s *TopupService) Get(ctx context.Context, topupID uuid.UUID) (*Topup, error) {
	dbTopup, err := s.queries.GetTopupByID(ctx, topupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTopupNotFound
		}
		return nil, fmt.Errorf("failed to get topup: %w", err)
	}
	return dbTopupToTopup(dbTopup), nil
}

// GetByOrganization retrieves a topup by ID and verifies organization ownership.
func (s *TopupService) GetByOrganization(ctx context.Context, topupID, organizationID uuid.UUID) (*Topup, error) {
	topup, err := s.Get(ctx, topupID)
	if err != nil {
		return nil, err
	}
	if topup.OrganizationID != organizationID {
		return nil, ErrTopupNotFound
	}
	return topup, nil
}

// ListTopupsRequest represents a request to list topups.
type ListTopupsRequest struct {
	OrganizationID uuid.UUID
	Status         *string
	Offset         int
	Limit          int
}

// ListTopupsResult represents the result of listing topups.
type ListTopupsResult struct {
	Topups []*Topup
	Total  int64
}

// List lists topups for an organization.
func (s *TopupService) List(ctx context.Context, req *ListTopupsRequest) (*ListTopupsResult, error) {
	// Get count
	total, err := s.queries.CountTopupsByOrganization(ctx, db.CountTopupsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count topups: %w", err)
	}

	// Get topups
	dbTopups, err := s.queries.ListTopupsByOrganization(ctx, db.ListTopupsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list topups: %w", err)
	}

	topups := make([]*Topup, len(dbTopups))
	for i, dbTopup := range dbTopups {
		topups[i] = dbTopupToTopup(dbTopup)
	}

	return &ListTopupsResult{
		Topups: topups,
		Total:  total,
	}, nil
}

// Cancel cancels a pending topup.
func (s *TopupService) Cancel(ctx context.Context, topupID uuid.UUID) (*Topup, error) {
	dbTopup, err := s.queries.GetTopupByID(ctx, topupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTopupNotFound
		}
		return nil, fmt.Errorf("failed to get topup: %w", err)
	}

	// Check if can be cancelled
	if dbTopup.Status != string(domain.TopupStatusPending) &&
		dbTopup.Status != string(domain.TopupStatusProcessing) {
		return nil, ErrTopupInvalidState
	}

	cancelledTopup, err := s.queries.UpdateTopupCancelled(ctx, dbTopup.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel topup: %w", err)
	}

	s.logger.Info("Topup cancelled",
		"topup_id", topupID,
		"organization_id", dbTopup.OrganizationID,
	)

	return dbTopupToTopup(cancelledTopup), nil
}

// ExpirePending expires all pending topups that have passed their expiration time.
func (s *TopupService) ExpirePending(ctx context.Context) error {
	if err := s.queries.ExpirePendingTopups(ctx); err != nil {
		return fmt.Errorf("failed to expire pending topups: %w", err)
	}
	return nil
}

// GetSummary retrieves a summary of topups for an organization.
func (s *TopupService) GetSummary(ctx context.Context, organizationID uuid.UUID) (*TopupSummary, error) {
	summary, err := s.queries.GetTopupSummaryByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topup summary: %w", err)
	}

	return &TopupSummary{
		TotalTopups:         summary.TotalTopups,
		TotalCompletedKurus: interfaceToInt64(summary.TotalCompletedKurus),
		TotalPendingKurus:   interfaceToInt64(summary.TotalPendingKurus),
	}, nil
}

// TopupSummary represents a summary of topups.
type TopupSummary struct {
	TotalTopups         int64 `json:"total_topups"`
	TotalCompletedKurus int64 `json:"total_completed_kurus"`
	TotalPendingKurus   int64 `json:"total_pending_kurus"`
}

// Helper functions

func dbTopupToTopup(dbTopup *db.Topup) *Topup {
	topup := &Topup{
		ID:              dbTopup.ID,
		OrganizationID:  dbTopup.OrganizationID,
		IdempotencyKey:  dbTopup.IdempotencyKey,
		AmountKurus:     int64(dbTopup.AmountKurus),
		Currency:        dbTopup.Currency,
		Status:          dbTopup.Status,
		PaymentProvider: dbTopup.PaymentProvider,
		PaymentID:       dbTopup.PaymentID,
		PaymentURL:      dbTopup.PaymentUrl,
		PaymentToken:    dbTopup.PaymentToken,
		FailureReason:   dbTopup.FailureReason,
		CreatedAt:       dbTopup.CreatedAt.Time,
		UpdatedAt:       dbTopup.UpdatedAt.Time,
	}

	if dbTopup.LedgerEntryID.Valid {
		id := uuid.UUID(dbTopup.LedgerEntryID.Bytes)
		topup.LedgerEntryID = &id
	}

	if dbTopup.CompletedAt.Valid {
		topup.CompletedAt = &dbTopup.CompletedAt.Time
	}

	if dbTopup.ExpiresAt.Valid {
		topup.ExpiresAt = dbTopup.ExpiresAt.Time
	}

	return topup
}

func stringPtr(s string) *string {
	return &s
}

func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
