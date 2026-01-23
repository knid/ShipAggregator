// Package provider provides provider account management services.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/knid/kargonet/internal/crypto"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// Common errors for provider account operations.
var (
	ErrAccountNotFound      = errors.New("provider account not found")
	ErrAccountAlreadyExists = errors.New("provider account already exists")
	ErrInvalidProviderCode  = errors.New("invalid provider code")
	ErrInvalidCredentials   = errors.New("invalid credentials format")
	ErrEncryptionFailed     = errors.New("failed to encrypt credentials")
	ErrDecryptionFailed     = errors.New("failed to decrypt credentials")
	ErrAccountNotVerified   = errors.New("provider account not verified")
	ErrAccountSuspended     = errors.New("provider account is suspended")
	ErrCannotSetDefault     = errors.New("cannot set unverified account as default")
	ErrMissingEncryptionKey = errors.New("encryption key not configured")
)

// AccountService provides provider account management operations.
type AccountService struct {
	queries       *db.Queries
	encryptionKey []byte
	logger        *observability.Logger
}

// NewAccountService creates a new provider account service.
func NewAccountService(queries *db.Queries, encryptionKey []byte, logger *observability.Logger) (*AccountService, error) {
	if len(encryptionKey) != crypto.AESKeySize {
		return nil, ErrMissingEncryptionKey
	}

	return &AccountService{
		queries:       queries,
		encryptionKey: encryptionKey,
		logger:        logger,
	}, nil
}

// CreateAccountRequest represents a request to create a provider account.
type CreateAccountRequest struct {
	OrganizationID uuid.UUID
	ProviderCode   string
	Name           string
	Credentials    domain.ProviderCredentials
	IsDefault      bool
}

// CreateAccount creates a new provider account with encrypted credentials.
func (s *AccountService) CreateAccount(ctx context.Context, req *CreateAccountRequest) (*db.ProviderAccount, error) {
	// Validate provider code
	providerCode := domain.ProviderCode(req.ProviderCode)
	if !providerCode.IsValid() {
		return nil, ErrInvalidProviderCode
	}

	// Serialize credentials to JSON
	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	// Encrypt credentials
	ciphertext, iv, err := crypto.EncryptCredentialsWithIV(credentialsJSON, s.encryptionKey)
	if err != nil {
		s.logger.Error("Failed to encrypt credentials", "error", err, "provider_code", req.ProviderCode)
		return nil, ErrEncryptionFailed
	}

	// If setting as default, clear existing default first
	if req.IsDefault {
		err = s.queries.ClearDefaultProviderAccount(ctx, db.ClearDefaultProviderAccountParams{
			OrganizationID: req.OrganizationID,
			ProviderCode:   req.ProviderCode,
		})
		if err != nil {
			s.logger.Error("Failed to clear default provider account", "error", err)
			// Continue anyway - this is not critical
		}
	}

	// Create the account
	account, err := s.queries.CreateProviderAccount(ctx, db.CreateProviderAccountParams{
		OrganizationID:       req.OrganizationID,
		ProviderCode:         req.ProviderCode,
		Name:                 req.Name,
		CredentialsEncrypted: ciphertext,
		CredentialsIv:        iv,
		Status:               domain.ProviderAccountStatusPendingVerification.String(),
		IsDefault:            false, // Cannot be default until verified
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider account: %w", err)
	}

	s.logger.Info("Provider account created",
		"account_id", account.ID,
		"organization_id", req.OrganizationID,
		"provider_code", req.ProviderCode,
	)

	return account, nil
}

// GetAccount retrieves a provider account by ID.
func (s *AccountService) GetAccount(ctx context.Context, accountID, organizationID uuid.UUID) (*db.ProviderAccount, error) {
	account, err := s.queries.GetProviderAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get provider account: %w", err)
	}

	// Verify organization ownership
	if account.OrganizationID != organizationID {
		return nil, ErrAccountNotFound
	}

	return account, nil
}

// GetVerifiedAccount retrieves a verified provider account by ID.
func (s *AccountService) GetVerifiedAccount(ctx context.Context, accountID uuid.UUID) (*db.ProviderAccount, error) {
	account, err := s.queries.GetVerifiedProviderAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get verified provider account: %w", err)
	}

	return account, nil
}

// GetDefaultAccount retrieves the default provider account for a provider.
func (s *AccountService) GetDefaultAccount(ctx context.Context, organizationID uuid.UUID, providerCode string) (*db.ProviderAccount, error) {
	account, err := s.queries.GetDefaultProviderAccount(ctx, db.GetDefaultProviderAccountParams{
		OrganizationID: organizationID,
		ProviderCode:   providerCode,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to get default provider account: %w", err)
	}

	return account, nil
}

// ListAccountsRequest represents a request to list provider accounts.
type ListAccountsRequest struct {
	OrganizationID uuid.UUID
	ProviderCode   *string
	Status         *string
	Offset         int
	Limit          int
}

// ListAccountsResult represents the result of listing provider accounts.
type ListAccountsResult struct {
	Accounts []*db.ProviderAccount
	Total    int64
}

// ListAccounts lists provider accounts for an organization.
func (s *AccountService) ListAccounts(ctx context.Context, req *ListAccountsRequest) (*ListAccountsResult, error) {
	// Get count
	count, err := s.queries.CountProviderAccountsByOrganization(ctx, db.CountProviderAccountsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		ProviderCode:   req.ProviderCode,
		Status:         req.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count provider accounts: %w", err)
	}

	// Get accounts
	accounts, err := s.queries.ListProviderAccountsByOrganization(ctx, db.ListProviderAccountsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		ProviderCode:   req.ProviderCode,
		Status:         req.Status,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list provider accounts: %w", err)
	}

	return &ListAccountsResult{
		Accounts: accounts,
		Total:    count,
	}, nil
}

// ListVerifiedAccounts lists verified provider accounts for an organization.
func (s *AccountService) ListVerifiedAccounts(ctx context.Context, organizationID uuid.UUID) ([]*db.ProviderAccount, error) {
	return s.queries.ListVerifiedProviderAccountsByOrganization(ctx, organizationID)
}

// GetAccountsByIDs retrieves verified provider accounts by their IDs.
func (s *AccountService) GetAccountsByIDs(ctx context.Context, accountIDs []uuid.UUID) ([]*db.ProviderAccount, error) {
	return s.queries.GetProviderAccountsByIDs(ctx, accountIDs)
}

// UpdateAccountRequest represents a request to update a provider account.
type UpdateAccountRequest struct {
	AccountID      uuid.UUID
	OrganizationID uuid.UUID
	Name           *string
	Credentials    *domain.ProviderCredentials
}

// UpdateAccount updates a provider account.
func (s *AccountService) UpdateAccount(ctx context.Context, req *UpdateAccountRequest) (*db.ProviderAccount, error) {
	// Get existing account
	account, err := s.GetAccount(ctx, req.AccountID, req.OrganizationID)
	if err != nil {
		return nil, err
	}

	// Prepare update params
	updateParams := db.UpdateProviderAccountParams{
		ID:   account.ID,
		Name: req.Name,
	}

	// If credentials are being updated, encrypt them and reset verification status
	if req.Credentials != nil {
		credentialsJSON, err := json.Marshal(req.Credentials)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
		}

		ciphertext, iv, err := crypto.EncryptCredentialsWithIV(credentialsJSON, s.encryptionKey)
		if err != nil {
			s.logger.Error("Failed to encrypt credentials", "error", err)
			return nil, ErrEncryptionFailed
		}

		updateParams.CredentialsEncrypted = ciphertext
		updateParams.CredentialsIv = iv

		// Reset verification status when credentials change
		_, err = s.queries.UpdateProviderAccountStatus(ctx, db.UpdateProviderAccountStatusParams{
			ID:                account.ID,
			Status:            domain.ProviderAccountStatusPendingVerification.String(),
			VerificationError: nil,
		})
		if err != nil {
			s.logger.Error("Failed to reset verification status", "error", err)
		}
	}

	// Update the account
	updatedAccount, err := s.queries.UpdateProviderAccount(ctx, updateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update provider account: %w", err)
	}

	s.logger.Info("Provider account updated",
		"account_id", account.ID,
		"organization_id", req.OrganizationID,
	)

	return updatedAccount, nil
}

// SetDefaultAccount sets a provider account as the default for its provider.
func (s *AccountService) SetDefaultAccount(ctx context.Context, accountID, organizationID uuid.UUID) (*db.ProviderAccount, error) {
	// Get the account
	account, err := s.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return nil, err
	}

	// Check if account is verified
	if account.Status != domain.ProviderAccountStatusVerified.String() {
		return nil, ErrCannotSetDefault
	}

	// Clear existing default for this provider
	err = s.queries.ClearDefaultProviderAccount(ctx, db.ClearDefaultProviderAccountParams{
		OrganizationID: organizationID,
		ProviderCode:   account.ProviderCode,
	})
	if err != nil {
		s.logger.Error("Failed to clear default provider account", "error", err)
	}

	// Set this account as default
	updatedAccount, err := s.queries.SetDefaultProviderAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to set default provider account: %w", err)
	}

	s.logger.Info("Provider account set as default",
		"account_id", accountID,
		"organization_id", organizationID,
		"provider_code", account.ProviderCode,
	)

	return updatedAccount, nil
}

// DeleteAccount soft-deletes a provider account.
func (s *AccountService) DeleteAccount(ctx context.Context, accountID, organizationID uuid.UUID) error {
	// Get the account to verify ownership
	_, err := s.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return err
	}

	// Soft delete the account
	_, err = s.queries.SoftDeleteProviderAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete provider account: %w", err)
	}

	s.logger.Info("Provider account deleted",
		"account_id", accountID,
		"organization_id", organizationID,
	)

	return nil
}

// SuspendAccount suspends a provider account.
func (s *AccountService) SuspendAccount(ctx context.Context, accountID, organizationID uuid.UUID) (*db.ProviderAccount, error) {
	// Get the account to verify ownership
	_, err := s.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return nil, err
	}

	// Suspend the account
	account, err := s.queries.SuspendProviderAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to suspend provider account: %w", err)
	}

	s.logger.Info("Provider account suspended",
		"account_id", accountID,
		"organization_id", organizationID,
	)

	return account, nil
}

// DecryptCredentials decrypts the credentials for a provider account.
func (s *AccountService) DecryptCredentials(ctx context.Context, account *db.ProviderAccount) (*domain.ProviderCredentials, error) {
	// Decrypt credentials
	plaintext, err := crypto.DecryptCredentialsWithIV(account.CredentialsEncrypted, account.CredentialsIv, s.encryptionKey)
	if err != nil {
		s.logger.Error("Failed to decrypt credentials",
			"error", err,
			"account_id", account.ID,
		)
		return nil, ErrDecryptionFailed
	}

	// Parse credentials
	var credentials domain.ProviderCredentials
	if err := json.Unmarshal(plaintext, &credentials); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	return &credentials, nil
}

// GetCredentialsForAccount retrieves and decrypts credentials for a provider account.
func (s *AccountService) GetCredentialsForAccount(ctx context.Context, accountID, organizationID uuid.UUID) (*domain.ProviderCredentials, error) {
	account, err := s.GetAccount(ctx, accountID, organizationID)
	if err != nil {
		return nil, err
	}

	return s.DecryptCredentials(ctx, account)
}

// RecordUsage records a successful usage of a provider account.
func (s *AccountService) RecordUsage(ctx context.Context, accountID uuid.UUID) error {
	return s.queries.UpdateProviderAccountLastUsed(ctx, accountID)
}

// UpdateStatus updates the status of a provider account.
func (s *AccountService) UpdateStatus(ctx context.Context, accountID uuid.UUID, status domain.ProviderAccountStatus, verificationError *string) (*db.ProviderAccount, error) {
	return s.queries.UpdateProviderAccountStatus(ctx, db.UpdateProviderAccountStatusParams{
		ID:                accountID,
		Status:            status.String(),
		VerificationError: verificationError,
	})
}

// MarkVerified marks a provider account as verified.
func (s *AccountService) MarkVerified(ctx context.Context, accountID uuid.UUID) (*db.ProviderAccount, error) {
	return s.UpdateStatus(ctx, accountID, domain.ProviderAccountStatusVerified, nil)
}

// MarkVerificationFailed marks a provider account as verification failed.
func (s *AccountService) MarkVerificationFailed(ctx context.Context, accountID uuid.UUID, errorMsg string) (*db.ProviderAccount, error) {
	return s.UpdateStatus(ctx, accountID, domain.ProviderAccountStatusVerificationFailed, &errorMsg)
}

// GetCredentialSchema returns the credential schema for a provider.
func GetCredentialSchema(providerCode domain.ProviderCode) []CredentialField {
	schemas := map[domain.ProviderCode][]CredentialField{
		domain.ProviderCodeAras: {
			{Name: "username", Type: "string", Required: true, Sensitive: false, Label: "Username"},
			{Name: "password", Type: "string", Required: true, Sensitive: true, Label: "Password"},
			{Name: "customer_code", Type: "string", Required: true, Sensitive: false, Label: "Customer Code"},
		},
		domain.ProviderCodeYurtici: {
			{Name: "username", Type: "string", Required: true, Sensitive: false, Label: "Username"},
			{Name: "password", Type: "string", Required: true, Sensitive: true, Label: "Password"},
		},
		domain.ProviderCodeHepsijet: {
			{Name: "api_key", Type: "string", Required: true, Sensitive: true, Label: "API Key"},
			{Name: "customer_code", Type: "string", Required: true, Sensitive: false, Label: "Merchant ID"},
		},
		domain.ProviderCodeMNG: {
			{Name: "username", Type: "string", Required: true, Sensitive: false, Label: "Username"},
			{Name: "password", Type: "string", Required: true, Sensitive: true, Label: "Password"},
			{Name: "customer_code", Type: "string", Required: true, Sensitive: false, Label: "Customer Number"},
		},
		domain.ProviderCodeSurat: {
			{Name: "username", Type: "string", Required: true, Sensitive: false, Label: "Username"},
			{Name: "password", Type: "string", Required: true, Sensitive: true, Label: "Password"},
		},
		domain.ProviderCodePTT: {
			{Name: "username", Type: "string", Required: true, Sensitive: false, Label: "Username"},
			{Name: "password", Type: "string", Required: true, Sensitive: true, Label: "Password"},
			{Name: "customer_code", Type: "string", Required: true, Sensitive: false, Label: "Sender Code"},
		},
		domain.ProviderCodeSendeo: {
			{Name: "api_key", Type: "string", Required: true, Sensitive: true, Label: "API Key"},
			{Name: "customer_code", Type: "string", Required: true, Sensitive: false, Label: "Customer Code"},
		},
		domain.ProviderCodeKolaygelsin: {
			{Name: "api_key", Type: "string", Required: true, Sensitive: true, Label: "API Key"},
		},
		domain.ProviderCodePakettaxi: {
			{Name: "api_key", Type: "string", Required: true, Sensitive: true, Label: "API Key"},
			{Name: "api_secret", Type: "string", Required: true, Sensitive: true, Label: "Secret Key"},
		},
	}

	if schema, ok := schemas[providerCode]; ok {
		return schema
	}
	return nil
}

// CredentialField represents a field in the credential schema.
type CredentialField struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Required  bool   `json:"required"`
	Sensitive bool   `json:"sensitive"`
	Label     string `json:"label"`
}

// ValidateCredentials validates credentials against the schema for a provider.
func ValidateCredentials(providerCode domain.ProviderCode, credentials domain.ProviderCredentials) error {
	schema := GetCredentialSchema(providerCode)
	if schema == nil {
		return ErrInvalidProviderCode
	}

	for _, field := range schema {
		if !field.Required {
			continue
		}

		var value string
		switch field.Name {
		case "username":
			value = credentials.Username
		case "password":
			value = credentials.Password
		case "api_key":
			value = credentials.APIKey
		case "api_secret":
			value = credentials.APISecret
		case "customer_code":
			value = credentials.CustomerCode
		case "account_number":
			value = credentials.AccountNumber
		case "contract_id":
			value = credentials.ContractID
		default:
			if credentials.Extra != nil {
				value = credentials.Extra[field.Name]
			}
		}

		if value == "" {
			return fmt.Errorf("%w: missing required field %s", ErrInvalidCredentials, field.Name)
		}
	}

	return nil
}
