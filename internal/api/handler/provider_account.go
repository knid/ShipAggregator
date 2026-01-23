// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	providerservice "github.com/knid/kargonet/internal/service/provider"
)

// ProviderAccountHandler handles provider account HTTP requests.
type ProviderAccountHandler struct {
	accountService      *providerservice.AccountService
	verificationService *providerservice.VerificationService
	logger              *observability.Logger
}

// NewProviderAccountHandler creates a new provider account handler.
func NewProviderAccountHandler(
	accountService *providerservice.AccountService,
	verificationService *providerservice.VerificationService,
	logger *observability.Logger,
) *ProviderAccountHandler {
	return &ProviderAccountHandler{
		accountService:      accountService,
		verificationService: verificationService,
		logger:              logger,
	}
}

// CreateProviderAccountRequest represents the request body for creating a provider account.
type CreateProviderAccountRequest struct {
	ProviderCode string                     `json:"provider_code"`
	Name         string                     `json:"name"`
	Credentials  domain.ProviderCredentials `json:"credentials"`
	IsDefault    bool                       `json:"is_default"`
}

// UpdateProviderAccountRequest represents the request body for updating a provider account.
type UpdateProviderAccountRequest struct {
	Name        *string                     `json:"name,omitempty"`
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// ProviderAccountResponse represents a provider account in API responses.
type ProviderAccountResponse struct {
	ID                uuid.UUID `json:"id"`
	OrganizationID    uuid.UUID `json:"organization_id"`
	ProviderCode      string    `json:"provider_code"`
	Name              string    `json:"name"`
	Status            string    `json:"status"`
	VerificationError *string   `json:"verification_error,omitempty"`
	VerifiedAt        *string   `json:"verified_at,omitempty"`
	LastUsedAt        *string   `json:"last_used_at,omitempty"`
	IsDefault         bool      `json:"is_default"`
	CreatedAt         string    `json:"created_at"`
	UpdatedAt         string    `json:"updated_at"`
}

// ListProviderAccountsResponse represents the response for listing provider accounts.
type ListProviderAccountsResponse struct {
	Accounts []ProviderAccountResponse `json:"accounts"`
	Total    int64                     `json:"total"`
	Offset   int                       `json:"offset"`
	Limit    int                       `json:"limit"`
}

// VerifyProviderAccountResponse represents the response for verifying a provider account.
type VerifyProviderAccountResponse struct {
	Success     bool                     `json:"success"`
	Error       string                   `json:"error,omitempty"`
	Account     *ProviderAccountResponse `json:"account,omitempty"`
	AccountInfo *AccountInfoResponse     `json:"account_info,omitempty"`
}

// AccountInfoResponse represents account info from the provider.
type AccountInfoResponse struct {
	AccountNumber string  `json:"account_number,omitempty"`
	CompanyName   string  `json:"company_name,omitempty"`
	ContractType  string  `json:"contract_type,omitempty"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
}

// CredentialSchemaResponse represents the credential schema for a provider.
type CredentialSchemaResponse struct {
	ProviderCode string                            `json:"provider_code"`
	Fields       []providerservice.CredentialField `json:"fields"`
}

// toProviderAccountResponse converts a db.ProviderAccount to ProviderAccountResponse.
func toProviderAccountResponse(account *db.ProviderAccount) ProviderAccountResponse {
	resp := ProviderAccountResponse{
		ID:             account.ID,
		OrganizationID: account.OrganizationID,
		ProviderCode:   account.ProviderCode,
		Name:           account.Name,
		Status:         account.Status,
		IsDefault:      account.IsDefault,
		CreatedAt:      account.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      account.UpdatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}

	if account.VerificationError != nil {
		resp.VerificationError = account.VerificationError
	}

	if account.VerifiedAt.Valid {
		verifiedAt := account.VerifiedAt.Time.Format("2006-01-02T15:04:05Z")
		resp.VerifiedAt = &verifiedAt
	}

	if account.LastUsedAt.Valid {
		lastUsedAt := account.LastUsedAt.Time.Format("2006-01-02T15:04:05Z")
		resp.LastUsedAt = &lastUsedAt
	}

	return resp
}

// CreateProviderAccount handles POST /provider-accounts
func (h *ProviderAccountHandler) CreateProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse request body
	var req CreateProviderAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate provider code
	providerCode := domain.ProviderCode(req.ProviderCode)
	if !providerCode.IsValid() {
		WriteError(w, r, NewBadRequestError("Invalid provider code"))
		return
	}

	// Validate credentials against schema
	if err := providerservice.ValidateCredentials(providerCode, req.Credentials); err != nil {
		WriteError(w, r, NewValidationError("Invalid credentials", []ErrorDetail{
			NewFieldError("credentials", "INVALID_CREDENTIALS", err.Error()),
		}))
		return
	}

	// Create the account
	account, err := h.accountService.CreateAccount(ctx, &providerservice.CreateAccountRequest{
		OrganizationID: orgID,
		ProviderCode:   req.ProviderCode,
		Name:           req.Name,
		Credentials:    req.Credentials,
		IsDefault:      req.IsDefault,
	})
	if err != nil {
		h.logger.Error("Failed to create provider account", "error", err)
		if errors.Is(err, providerservice.ErrInvalidProviderCode) {
			WriteError(w, r, NewBadRequestError(err.Error()))
			return
		}
		if errors.Is(err, providerservice.ErrInvalidCredentials) {
			WriteError(w, r, NewBadRequestError(err.Error()))
			return
		}
		WriteError(w, r, NewInternalError("Failed to create provider account"))
		return
	}

	writeJSON(w, http.StatusCreated, toProviderAccountResponse(account))
}

// GetProviderAccount handles GET /provider-accounts/{id}
func (h *ProviderAccountHandler) GetProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Get the account
	account, err := h.accountService.GetAccount(ctx, accountID, orgID)
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		h.logger.Error("Failed to get provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to get provider account"))
		return
	}

	writeJSON(w, http.StatusOK, toProviderAccountResponse(account))
}

// ListProviderAccounts handles GET /provider-accounts
func (h *ProviderAccountHandler) ListProviderAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	var providerCode *string
	if pc := query.Get("provider_code"); pc != "" {
		providerCode = &pc
	}

	var status *string
	if s := query.Get("status"); s != "" {
		status = &s
	}

	offset := 0
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	limit := 20
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// List accounts
	result, err := h.accountService.ListAccounts(ctx, &providerservice.ListAccountsRequest{
		OrganizationID: orgID,
		ProviderCode:   providerCode,
		Status:         status,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list provider accounts", "error", err)
		WriteError(w, r, NewInternalError("Failed to list provider accounts"))
		return
	}

	// Convert to response
	accounts := make([]ProviderAccountResponse, len(result.Accounts))
	for i, account := range result.Accounts {
		accounts[i] = toProviderAccountResponse(account)
	}

	writeJSON(w, http.StatusOK, ListProviderAccountsResponse{
		Accounts: accounts,
		Total:    result.Total,
		Offset:   offset,
		Limit:    limit,
	})
}

// UpdateProviderAccount handles PUT /provider-accounts/{id}
func (h *ProviderAccountHandler) UpdateProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Parse request body
	var req UpdateProviderAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Get existing account to validate credentials if provided
	if req.Credentials != nil {
		account, err := h.accountService.GetAccount(ctx, accountID, orgID)
		if err != nil {
			if errors.Is(err, providerservice.ErrAccountNotFound) {
				WriteError(w, r, NewNotFoundError("Provider account"))
				return
			}
			h.logger.Error("Failed to get provider account", "error", err)
			WriteError(w, r, NewInternalError("Failed to get provider account"))
			return
		}

		providerCode := domain.ProviderCode(account.ProviderCode)
		if err := providerservice.ValidateCredentials(providerCode, *req.Credentials); err != nil {
			WriteError(w, r, NewValidationError("Invalid credentials", []ErrorDetail{
				NewFieldError("credentials", "INVALID_CREDENTIALS", err.Error()),
			}))
			return
		}
	}

	// Update the account
	account, err := h.accountService.UpdateAccount(ctx, &providerservice.UpdateAccountRequest{
		AccountID:      accountID,
		OrganizationID: orgID,
		Name:           req.Name,
		Credentials:    req.Credentials,
	})
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		h.logger.Error("Failed to update provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to update provider account"))
		return
	}

	writeJSON(w, http.StatusOK, toProviderAccountResponse(account))
}

// DeleteProviderAccount handles DELETE /provider-accounts/{id}
func (h *ProviderAccountHandler) DeleteProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Delete the account
	err = h.accountService.DeleteAccount(ctx, accountID, orgID)
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		h.logger.Error("Failed to delete provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to delete provider account"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetDefaultProviderAccount handles POST /provider-accounts/{id}/set-default
func (h *ProviderAccountHandler) SetDefaultProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Set as default
	account, err := h.accountService.SetDefaultAccount(ctx, accountID, orgID)
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		if errors.Is(err, providerservice.ErrCannotSetDefault) {
			WriteError(w, r, NewBadRequestError("Cannot set unverified account as default"))
			return
		}
		h.logger.Error("Failed to set default provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to set default provider account"))
		return
	}

	writeJSON(w, http.StatusOK, toProviderAccountResponse(account))
}

// VerifyProviderAccount handles POST /provider-accounts/{id}/verify
func (h *ProviderAccountHandler) VerifyProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Verify the account
	result, err := h.verificationService.VerifyAccount(ctx, &providerservice.VerifyAccountRequest{
		AccountID:      accountID,
		OrganizationID: orgID,
	})
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		if errors.Is(err, providerservice.ErrProviderNotSupported) {
			WriteError(w, r, NewBadRequestError("Provider does not support credential verification"))
			return
		}
		h.logger.Error("Failed to verify provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to verify provider account"))
		return
	}

	// Build response
	resp := VerifyProviderAccountResponse{
		Success: result.Success,
		Error:   result.Error,
	}

	if result.Account != nil {
		accountResp := toProviderAccountResponse(result.Account)
		resp.Account = &accountResp
	}

	if result.AccountInfo != nil {
		accountInfo := AccountInfoResponse{
			AccountNumber: result.AccountInfo.AccountNumber,
			CompanyName:   result.AccountInfo.CompanyName,
			ContractType:  result.AccountInfo.ContractType,
		}
		if result.AccountInfo.ExpiresAt != nil {
			expiresAt := result.AccountInfo.ExpiresAt.Format("2006-01-02T15:04:05Z")
			accountInfo.ExpiresAt = &expiresAt
		}
		resp.AccountInfo = &accountInfo
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetCredentialSchema handles GET /providers/{provider_code}/credential-schema
func (h *ProviderAccountHandler) GetCredentialSchema(w http.ResponseWriter, r *http.Request) {
	// Parse provider code from URL
	providerCodeStr := chi.URLParam(r, "provider_code")
	providerCode := domain.ProviderCode(providerCodeStr)

	if !providerCode.IsValid() {
		WriteError(w, r, NewBadRequestError("Invalid provider code"))
		return
	}

	// Get credential schema
	schema := providerservice.GetCredentialSchema(providerCode)
	if schema == nil {
		WriteError(w, r, NewNotFoundError("Credential schema"))
		return
	}

	writeJSON(w, http.StatusOK, CredentialSchemaResponse{
		ProviderCode: providerCodeStr,
		Fields:       schema,
	})
}

// SuspendProviderAccount handles POST /provider-accounts/{id}/suspend
func (h *ProviderAccountHandler) SuspendProviderAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse account ID from URL
	accountIDStr := chi.URLParam(r, "id")
	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid account ID"))
		return
	}

	// Suspend the account
	account, err := h.accountService.SuspendAccount(ctx, accountID, orgID)
	if err != nil {
		if errors.Is(err, providerservice.ErrAccountNotFound) {
			WriteError(w, r, NewNotFoundError("Provider account"))
			return
		}
		h.logger.Error("Failed to suspend provider account", "error", err)
		WriteError(w, r, NewInternalError("Failed to suspend provider account"))
		return
	}

	writeJSON(w, http.StatusOK, toProviderAccountResponse(account))
}
