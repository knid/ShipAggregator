// Package provider provides provider account management services.
package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/provider"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// Verification errors.
var (
	ErrVerificationFailed     = errors.New("credential verification failed")
	ErrProviderNotSupported   = errors.New("provider does not support credential verification")
	ErrVerificationInProgress = errors.New("verification already in progress")
)

// VerificationService provides credential verification operations.
type VerificationService struct {
	accountService *AccountService
	adapterPool    *provider.AdapterPool
	queries        *db.Queries
	logger         *observability.Logger
}

// NewVerificationService creates a new verification service.
func NewVerificationService(
	accountService *AccountService,
	adapterPool *provider.AdapterPool,
	queries *db.Queries,
	logger *observability.Logger,
) *VerificationService {
	return &VerificationService{
		accountService: accountService,
		adapterPool:    adapterPool,
		queries:        queries,
		logger:         logger,
	}
}

// VerifyAccountRequest represents a request to verify a provider account.
type VerifyAccountRequest struct {
	AccountID      uuid.UUID
	OrganizationID uuid.UUID
}

// VerifyAccountResult represents the result of verifying a provider account.
type VerifyAccountResult struct {
	Account     *db.ProviderAccount
	Success     bool
	Error       string
	AccountInfo *provider.AccountInfo
}

// VerifyAccount verifies the credentials of a provider account by calling the carrier API.
func (s *VerificationService) VerifyAccount(ctx context.Context, req *VerifyAccountRequest) (*VerifyAccountResult, error) {
	// Get the account
	account, err := s.accountService.GetAccount(ctx, req.AccountID, req.OrganizationID)
	if err != nil {
		return nil, err
	}

	// Decrypt credentials
	credentials, err := s.accountService.DecryptCredentials(ctx, account)
	if err != nil {
		return nil, err
	}

	// Get the provider adapter
	providerCode := domain.ProviderCode(account.ProviderCode)
	adapter, err := s.adapterPool.Get(providerCode)
	if err != nil {
		s.logger.Error("Failed to get provider adapter",
			"error", err,
			"provider_code", account.ProviderCode,
		)
		return nil, fmt.Errorf("%w: %v", ErrProviderNotSupported, err)
	}

	// Call the provider to verify credentials
	s.logger.Info("Verifying provider account credentials",
		"account_id", account.ID,
		"provider_code", account.ProviderCode,
	)

	verificationResult, err := adapter.VerifyCredentials(ctx, *credentials)
	if err != nil {
		// Handle provider errors
		var providerErr *provider.ProviderError
		if errors.As(err, &providerErr) {
			// Mark as verification failed
			errorMsg := providerErr.Message
			if providerErr.ProviderMessage != "" {
				errorMsg = fmt.Sprintf("%s: %s", providerErr.Message, providerErr.ProviderMessage)
			}

			updatedAccount, updateErr := s.accountService.MarkVerificationFailed(ctx, account.ID, errorMsg)
			if updateErr != nil {
				s.logger.Error("Failed to update account status", "error", updateErr)
			}

			s.logger.Warn("Provider account verification failed",
				"account_id", account.ID,
				"provider_code", account.ProviderCode,
				"error", errorMsg,
			)

			return &VerifyAccountResult{
				Account: updatedAccount,
				Success: false,
				Error:   errorMsg,
			}, nil
		}

		// Other errors
		s.logger.Error("Verification request failed",
			"error", err,
			"account_id", account.ID,
			"provider_code", account.ProviderCode,
		)
		return nil, fmt.Errorf("%w: %v", ErrVerificationFailed, err)
	}

	// Process verification result
	if !verificationResult.Success {
		// Mark as verification failed
		errorMsg := verificationResult.Error
		if errorMsg == "" {
			errorMsg = "Verification failed"
		}

		updatedAccount, updateErr := s.accountService.MarkVerificationFailed(ctx, account.ID, errorMsg)
		if updateErr != nil {
			s.logger.Error("Failed to update account status", "error", updateErr)
		}

		s.logger.Warn("Provider account verification failed",
			"account_id", account.ID,
			"provider_code", account.ProviderCode,
			"error", errorMsg,
		)

		return &VerifyAccountResult{
			Account: updatedAccount,
			Success: false,
			Error:   errorMsg,
		}, nil
	}

	// Mark as verified
	updatedAccount, err := s.accountService.MarkVerified(ctx, account.ID)
	if err != nil {
		s.logger.Error("Failed to mark account as verified", "error", err)
		return nil, fmt.Errorf("failed to update account status: %w", err)
	}

	s.logger.Info("Provider account verified successfully",
		"account_id", account.ID,
		"provider_code", account.ProviderCode,
		"account_number", verificationResult.AccountInfo.AccountNumber,
	)

	return &VerifyAccountResult{
		Account:     updatedAccount,
		Success:     true,
		AccountInfo: verificationResult.AccountInfo,
	}, nil
}

// VerifyCredentialsDirectly verifies credentials without storing them.
// This is useful for testing credentials before creating an account.
func (s *VerificationService) VerifyCredentialsDirectly(ctx context.Context, providerCode domain.ProviderCode, credentials domain.ProviderCredentials) (*provider.VerificationResult, error) {
	// Validate credentials against schema
	if err := ValidateCredentials(providerCode, credentials); err != nil {
		return nil, err
	}

	// Get the provider adapter
	adapter, err := s.adapterPool.Get(providerCode)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderNotSupported, err)
	}

	// Call the provider to verify credentials
	result, err := adapter.VerifyCredentials(ctx, credentials)
	if err != nil {
		var providerErr *provider.ProviderError
		if errors.As(err, &providerErr) {
			return &provider.VerificationResult{
				Success: false,
				Error:   providerErr.Message,
			}, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrVerificationFailed, err)
	}

	return result, nil
}

// ResetVerification resets the verification status of an account to pending.
func (s *VerificationService) ResetVerification(ctx context.Context, accountID, organizationID uuid.UUID) (*db.ProviderAccount, error) {
	// Get the account to verify ownership
	_, err := s.accountService.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return nil, err
	}

	// Reset to pending verification
	return s.accountService.UpdateStatus(ctx, accountID, domain.ProviderAccountStatusPendingVerification, nil)
}

// GetVerificationStatus returns the verification status of an account.
func (s *VerificationService) GetVerificationStatus(ctx context.Context, accountID, organizationID uuid.UUID) (*VerificationStatus, error) {
	account, err := s.accountService.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return nil, err
	}

	status := &VerificationStatus{
		AccountID:    account.ID,
		ProviderCode: account.ProviderCode,
		Status:       domain.ProviderAccountStatus(account.Status),
		IsVerified:   account.Status == domain.ProviderAccountStatusVerified.String(),
	}

	if account.VerificationError != nil {
		status.Error = *account.VerificationError
	}

	if account.VerifiedAt.Valid {
		status.VerifiedAt = &account.VerifiedAt.Time
	}

	return status, nil
}

// VerificationStatus represents the verification status of a provider account.
type VerificationStatus struct {
	AccountID    uuid.UUID                    `json:"account_id"`
	ProviderCode string                       `json:"provider_code"`
	Status       domain.ProviderAccountStatus `json:"status"`
	IsVerified   bool                         `json:"is_verified"`
	Error        string                       `json:"error,omitempty"`
	VerifiedAt   *time.Time                   `json:"verified_at,omitempty"`
}

// BatchVerifyAccounts verifies multiple accounts in sequence.
// This is useful for re-verifying all accounts for an organization.
func (s *VerificationService) BatchVerifyAccounts(ctx context.Context, organizationID uuid.UUID, accountIDs []uuid.UUID) ([]*VerifyAccountResult, error) {
	results := make([]*VerifyAccountResult, 0, len(accountIDs))

	for _, accountID := range accountIDs {
		result, err := s.VerifyAccount(ctx, &VerifyAccountRequest{
			AccountID:      accountID,
			OrganizationID: organizationID,
		})
		if err != nil {
			s.logger.Error("Batch verification failed for account",
				"error", err,
				"account_id", accountID,
			)
			// Continue with other accounts
			results = append(results, &VerifyAccountResult{
				Success: false,
				Error:   err.Error(),
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// VerifyAllPendingAccounts verifies all pending accounts for an organization.
func (s *VerificationService) VerifyAllPendingAccounts(ctx context.Context, organizationID uuid.UUID) ([]*VerifyAccountResult, error) {
	// Get all pending accounts
	status := domain.ProviderAccountStatusPendingVerification.String()
	listResult, err := s.accountService.ListAccounts(ctx, &ListAccountsRequest{
		OrganizationID: organizationID,
		Status:         &status,
		Offset:         0,
		Limit:          100, // Reasonable limit
	})
	if err != nil {
		return nil, err
	}

	// Extract account IDs
	accountIDs := make([]uuid.UUID, len(listResult.Accounts))
	for i, account := range listResult.Accounts {
		accountIDs[i] = account.ID
	}

	// Verify all accounts
	return s.BatchVerifyAccounts(ctx, organizationID, accountIDs)
}
