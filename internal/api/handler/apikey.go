// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/service/auth"
)

// APIKeyHandler handles API key HTTP requests.
type APIKeyHandler struct {
	apiKeyService *auth.APIKeyService
	logger        *observability.Logger
}

// NewAPIKeyHandler creates a new API key handler.
func NewAPIKeyHandler(apiKeyService *auth.APIKeyService, logger *observability.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		logger:        logger,
	}
}

// CreateAPIKeyRequest represents the request body for creating an API key.
type CreateAPIKeyRequest struct {
	Name               string     `json:"name"`
	Scopes             []string   `json:"scopes"`
	RateLimitPerMinute *int       `json:"rate_limit_per_minute,omitempty"`
	IPAllowlist        []string   `json:"ip_allowlist,omitempty"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
}

// UpdateAPIKeyRequest represents the request body for updating an API key.
type UpdateAPIKeyRequest struct {
	Name               *string  `json:"name,omitempty"`
	Scopes             []string `json:"scopes,omitempty"`
	RateLimitPerMinute *int     `json:"rate_limit_per_minute,omitempty"`
	IPAllowlist        []string `json:"ip_allowlist,omitempty"`
	IsActive           *bool    `json:"is_active,omitempty"`
}

// APIKeyResponse represents an API key in API responses.
type APIKeyResponse struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	KeyPrefix          string    `json:"key_prefix"`
	Scopes             []string  `json:"scopes"`
	RateLimitPerMinute int       `json:"rate_limit_per_minute"`
	IPAllowlist        []string  `json:"ip_allowlist,omitempty"`
	ExpiresAt          *string   `json:"expires_at,omitempty"`
	LastUsedAt         *string   `json:"last_used_at,omitempty"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

// APIKeyWithSecretResponse represents an API key with its secret (only returned on creation/rotation).
type APIKeyWithSecretResponse struct {
	APIKeyResponse
	Secret string `json:"secret"`
}

// ListAPIKeysResponse represents the response for listing API keys.
type ListAPIKeysResponse struct {
	Items  []APIKeyResponse `json:"items"`
	Total  int64            `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
}

// toAPIKeyResponse converts a domain.APIKey to APIKeyResponse.
func toAPIKeyResponse(key *domain.APIKey) APIKeyResponse {
	resp := APIKeyResponse{
		ID:                 key.ID,
		Name:               key.Name,
		KeyPrefix:          key.KeyPrefix,
		Scopes:             key.ScopesToStrings(),
		RateLimitPerMinute: key.RateLimitPerMinute,
		IPAllowlist:        key.IPAllowlist,
		IsActive:           key.IsActive,
		CreatedAt:          key.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          key.UpdatedAt.Format(time.RFC3339),
	}

	if key.ExpiresAt != nil {
		expiresAt := key.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAt
	}

	if key.LastUsedAt != nil {
		lastUsedAt := key.LastUsedAt.Format(time.RFC3339)
		resp.LastUsedAt = &lastUsedAt
	}

	return resp
}

// stringsToScopes converts string slice to APIKeyScope slice.
func stringsToScopes(strs []string) []domain.APIKeyScope {
	result := make([]domain.APIKeyScope, len(strs))
	for i, s := range strs {
		result[i] = domain.APIKeyScope(s)
	}
	return result
}

// CreateAPIKey handles POST /api-keys
// Creates a new API key for the organization.
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse request body
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.Name == "" {
		fieldErrors = append(fieldErrors, NewFieldError("name", "REQUIRED", "Name is required"))
	}
	if len(req.Scopes) == 0 {
		fieldErrors = append(fieldErrors, NewFieldError("scopes", "REQUIRED", "At least one scope is required"))
	}
	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	// Validate scopes
	scopes := stringsToScopes(req.Scopes)
	for _, scope := range scopes {
		if !scope.IsValid() {
			fieldErrors = append(fieldErrors, NewFieldErrorWithValue("scopes", "INVALID_SCOPE", "Invalid scope", string(scope)))
		}
	}
	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Invalid scopes", fieldErrors))
		return
	}

	// Set default rate limit if not provided
	rateLimitPerMinute := 100
	if req.RateLimitPerMinute != nil {
		rateLimitPerMinute = *req.RateLimitPerMinute
	}

	// Create the API key
	result, err := h.apiKeyService.CreateAPIKey(ctx, auth.CreateAPIKeyRequest{
		OrganizationID:     orgID,
		Name:               req.Name,
		Scopes:             scopes,
		RateLimitPerMinute: rateLimitPerMinute,
		IPAllowlist:        req.IPAllowlist,
		ExpiresAt:          req.ExpiresAt,
	})
	if err != nil {
		h.logger.Error("Failed to create API key", "error", err, "org_id", orgID)
		if errors.Is(err, domain.ErrValidation) {
			WriteError(w, r, NewValidationError(err.Error(), nil))
			return
		}
		WriteError(w, r, NewInternalError("Failed to create API key"))
		return
	}

	// Return the API key with secret (only time secret is returned)
	response := APIKeyWithSecretResponse{
		APIKeyResponse: toAPIKeyResponse(result.APIKey),
		Secret:         result.SecretKey,
	}

	h.logger.Info("API key created",
		"key_id", result.APIKey.ID,
		"org_id", orgID,
		"name", req.Name,
	)

	writeJSON(w, http.StatusCreated, response)
}

// GetAPIKey handles GET /api-keys/{id}
// Returns a specific API key.
func (h *APIKeyHandler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse key ID from URL
	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid API key ID"))
		return
	}

	// Get the API key
	key, err := h.apiKeyService.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to get API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to get API key"))
		return
	}

	// Verify the key belongs to the organization
	if key.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("API key"))
		return
	}

	writeJSON(w, http.StatusOK, toAPIKeyResponse(key))
}

// ListAPIKeys handles GET /api-keys
// Lists all API keys for the organization.
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse query parameters
	query := r.URL.Query()

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

	// List API keys
	keys, err := h.apiKeyService.ListAPIKeys(ctx, auth.ListAPIKeysRequest{
		OrganizationID: orgID,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list API keys", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list API keys"))
		return
	}

	// Get total count
	total, err := h.apiKeyService.CountAPIKeys(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to count API keys", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list API keys"))
		return
	}

	// Convert to response
	items := make([]APIKeyResponse, len(keys))
	for i, key := range keys {
		items[i] = toAPIKeyResponse(key)
	}

	response := ListAPIKeysResponse{
		Items:  items,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}

	writeJSON(w, http.StatusOK, response)
}

// UpdateAPIKey handles PATCH /api-keys/{id}
// Updates an API key.
func (h *APIKeyHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse key ID from URL
	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid API key ID"))
		return
	}

	// Verify the key belongs to the organization
	existingKey, err := h.apiKeyService.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to get API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to get API key"))
		return
	}
	if existingKey.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("API key"))
		return
	}

	// Parse request body
	var req UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate scopes if provided
	var scopes []domain.APIKeyScope
	if len(req.Scopes) > 0 {
		scopes = stringsToScopes(req.Scopes)
		var fieldErrors []ErrorDetail
		for _, scope := range scopes {
			if !scope.IsValid() {
				fieldErrors = append(fieldErrors, NewFieldErrorWithValue("scopes", "INVALID_SCOPE", "Invalid scope", string(scope)))
			}
		}
		if len(fieldErrors) > 0 {
			WriteError(w, r, NewValidationError("Invalid scopes", fieldErrors))
			return
		}
	}

	// Update the API key
	key, err := h.apiKeyService.UpdateAPIKey(ctx, auth.UpdateAPIKeyRequest{
		KeyID:              keyID,
		Name:               req.Name,
		Scopes:             scopes,
		RateLimitPerMinute: req.RateLimitPerMinute,
		IPAllowlist:        req.IPAllowlist,
		IsActive:           req.IsActive,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			WriteError(w, r, NewValidationError(err.Error(), nil))
			return
		}
		h.logger.Error("Failed to update API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to update API key"))
		return
	}

	h.logger.Info("API key updated", "key_id", keyID, "org_id", orgID)

	writeJSON(w, http.StatusOK, toAPIKeyResponse(key))
}

// RevokeAPIKey handles DELETE /api-keys/{id}
// Permanently revokes an API key.
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse key ID from URL
	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid API key ID"))
		return
	}

	// Verify the key belongs to the organization
	existingKey, err := h.apiKeyService.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to get API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to get API key"))
		return
	}
	if existingKey.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("API key"))
		return
	}

	// Revoke the API key
	err = h.apiKeyService.RevokeAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to revoke API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to revoke API key"))
		return
	}

	h.logger.Info("API key revoked", "key_id", keyID, "org_id", orgID)

	w.WriteHeader(http.StatusNoContent)
}

// RotateAPIKeyResponse represents the response for rotating an API key.
type RotateAPIKeyResponse struct {
	APIKeyWithSecretResponse
	GracePeriodEndsAt string `json:"grace_period_ends_at"`
}

// RotateAPIKey handles POST /api-keys/{id}/rotate
// Rotates an API key, generating a new secret.
// The old secret remains valid for a 24-hour grace period.
func (h *APIKeyHandler) RotateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context required"))
		return
	}

	// Parse key ID from URL
	keyIDStr := chi.URLParam(r, "id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid API key ID"))
		return
	}

	// Verify the key belongs to the organization
	existingKey, err := h.apiKeyService.GetAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to get API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to get API key"))
		return
	}
	if existingKey.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("API key"))
		return
	}

	// Check if key is active
	if !existingKey.IsActive {
		WriteError(w, r, NewConflictError("Cannot rotate an inactive API key"))
		return
	}

	// Check if key is revoked
	if existingKey.IsRevoked() {
		WriteError(w, r, NewConflictError("Cannot rotate a revoked API key"))
		return
	}

	// Default grace period is 24 hours
	gracePeriod := 24 * time.Hour

	// Rotate the API key
	result, err := h.apiKeyService.RotateAPIKey(ctx, keyID, gracePeriod)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteError(w, r, NewNotFoundError("API key"))
			return
		}
		h.logger.Error("Failed to rotate API key", "error", err, "key_id", keyID)
		WriteError(w, r, NewInternalError("Failed to rotate API key"))
		return
	}

	// Calculate grace period end time
	gracePeriodEndsAt := time.Now().UTC().Add(gracePeriod)

	h.logger.Info("API key rotated",
		"key_id", keyID,
		"org_id", orgID,
		"grace_period_ends", gracePeriodEndsAt,
	)

	// Return the API key with new secret
	response := RotateAPIKeyResponse{
		APIKeyWithSecretResponse: APIKeyWithSecretResponse{
			APIKeyResponse: toAPIKeyResponse(result.APIKey),
			Secret:         result.NewSecretKey,
		},
		GracePeriodEndsAt: gracePeriodEndsAt.Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}
