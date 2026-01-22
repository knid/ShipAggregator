// Package auth provides authentication and authorization services.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// APIKeyService handles API key validation and management.
type APIKeyService struct {
	queries *db.Queries
}

// NewAPIKeyService creates a new API key service.
func NewAPIKeyService(queries *db.Queries) *APIKeyService {
	return &APIKeyService{
		queries: queries,
	}
}

// ValidateAPIKeyResult contains the result of API key validation.
type ValidateAPIKeyResult struct {
	APIKey         *domain.APIKey
	OrganizationID uuid.UUID
	IsRotatedKey   bool // True if authenticated with the previous (rotated) key
}

// ValidateAPIKey validates an API key and returns the associated key details.
// It checks both the current key hash and the previous key hash (during rotation grace period).
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, apiKey string) (*ValidateAPIKeyResult, error) {
	keyHash := hashAPIKey(apiKey)

	// Try to find by current key hash
	dbKey, err := s.queries.GetAPIKeyByHash(ctx, keyHash)
	if err == nil {
		// Found by current key hash
		domainKey := s.dbToDomain(dbKey)
		if !domainKey.IsValid() {
			return nil, domain.ErrAPIKeyInactive
		}

		// Update last used timestamp asynchronously (fire and forget)
		go func() {
			_ = s.queries.UpdateAPIKeyLastUsed(context.Background(), dbKey.ID)
		}()

		return &ValidateAPIKeyResult{
			APIKey:         domainKey,
			OrganizationID: dbKey.OrganizationID,
			IsRotatedKey:   false,
		}, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Try to find by previous key hash (rotation grace period)
	dbKey, err = s.queries.GetAPIKeyByPreviousHash(ctx, &keyHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key by previous hash: %w", err)
	}

	// Found by previous key hash
	domainKey := s.dbToDomain(dbKey)
	if !domainKey.IsValid() {
		return nil, domain.ErrAPIKeyInactive
	}

	// Check if previous key is still valid
	if !domainKey.IsPreviousKeyValid() {
		return nil, domain.ErrAPIKeyExpired
	}

	// Update last used timestamp asynchronously
	go func() {
		_ = s.queries.UpdateAPIKeyLastUsed(context.Background(), dbKey.ID)
	}()

	return &ValidateAPIKeyResult{
		APIKey:         domainKey,
		OrganizationID: dbKey.OrganizationID,
		IsRotatedKey:   true,
	}, nil
}

// CheckScope verifies that an API key has the required scope.
func (s *APIKeyService) CheckScope(apiKey *domain.APIKey, requiredScope domain.APIKeyScope) error {
	if !apiKey.HasScope(requiredScope) {
		return domain.ErrAPIKeyInvalidScope
	}
	return nil
}

// CheckAnyScope verifies that an API key has at least one of the required scopes.
func (s *APIKeyService) CheckAnyScope(apiKey *domain.APIKey, requiredScopes ...domain.APIKeyScope) error {
	if !apiKey.HasAnyScope(requiredScopes...) {
		return domain.ErrAPIKeyInvalidScope
	}
	return nil
}

// CheckIPAllowed verifies that the client IP is allowed for the API key.
func (s *APIKeyService) CheckIPAllowed(apiKey *domain.APIKey, clientIP string) error {
	if !apiKey.IsIPAllowed(clientIP) {
		return domain.ErrAPIKeyIPNotAllowed
	}
	return nil
}

// CreateAPIKeyRequest contains the parameters for creating a new API key.
type CreateAPIKeyRequest struct {
	OrganizationID     uuid.UUID
	Name               string
	Scopes             []domain.APIKeyScope
	RateLimitPerMinute int
	IPAllowlist        []string
	ExpiresAt          *time.Time
}

// CreateAPIKeyResult contains the result of creating a new API key.
type CreateAPIKeyResult struct {
	APIKey    *domain.APIKey
	SecretKey string // The raw API key (only returned once)
}

// CreateAPIKey creates a new API key for an organization.
func (s *APIKeyService) CreateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResult, error) {
	// Generate a new API key
	secretKey, keyPrefix, keyHash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Validate scopes
	for _, scope := range req.Scopes {
		if !scope.IsValid() {
			return nil, fmt.Errorf("%w: invalid scope %s", domain.ErrValidation, scope)
		}
	}

	// Convert scopes to JSON
	scopesJSON, err := json.Marshal(scopesToStrings(req.Scopes))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal scopes: %w", err)
	}

	// Convert IP allowlist to JSON
	ipAllowlistJSON, err := json.Marshal(req.IPAllowlist)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal IP allowlist: %w", err)
	}

	// Set default rate limit if not provided
	rateLimit := req.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = 100
	}

	// Prepare expiration
	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{Time: *req.ExpiresAt, Valid: true}
	}

	// Create the API key in the database
	dbKey, err := s.queries.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		OrganizationID:     req.OrganizationID,
		Name:               req.Name,
		KeyPrefix:          keyPrefix,
		KeyHash:            keyHash,
		Scopes:             scopesJSON,
		RateLimitPerMinute: int32(rateLimit),
		IpAllowlist:        ipAllowlistJSON,
		ExpiresAt:          expiresAt,
		IsActive:           true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &CreateAPIKeyResult{
		APIKey:    s.dbToDomain(dbKey),
		SecretKey: secretKey,
	}, nil
}

// RotateAPIKeyResult contains the result of rotating an API key.
type RotateAPIKeyResult struct {
	APIKey       *domain.APIKey
	NewSecretKey string // The new API key (only returned once)
}

// RotateAPIKey rotates an API key, generating a new secret while keeping the old one valid for a grace period.
func (s *APIKeyService) RotateAPIKey(ctx context.Context, keyID uuid.UUID, gracePeriod time.Duration) (*RotateAPIKeyResult, error) {
	// Generate a new API key
	newSecretKey, newKeyPrefix, newKeyHash, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new API key: %w", err)
	}

	// Default grace period is 24 hours
	if gracePeriod <= 0 {
		gracePeriod = 24 * time.Hour
	}

	previousKeyExpiresAt := time.Now().UTC().Add(gracePeriod)

	// Rotate the key in the database
	dbKey, err := s.queries.RotateAPIKey(ctx, db.RotateAPIKeyParams{
		ID:                   keyID,
		KeyHash:              newKeyHash,
		PreviousKeyExpiresAt: pgtype.Timestamptz{Time: previousKeyExpiresAt, Valid: true},
		KeyPrefix:            newKeyPrefix,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to rotate API key: %w", err)
	}

	return &RotateAPIKeyResult{
		APIKey:       s.dbToDomain(dbKey),
		NewSecretKey: newSecretKey,
	}, nil
}

// GetAPIKey retrieves an API key by ID.
func (s *APIKeyService) GetAPIKey(ctx context.Context, keyID uuid.UUID) (*domain.APIKey, error) {
	dbKey, err := s.queries.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return s.dbToDomain(dbKey), nil
}

// ListAPIKeysRequest contains the parameters for listing API keys.
type ListAPIKeysRequest struct {
	OrganizationID uuid.UUID
	Offset         int
	Limit          int
}

// ListAPIKeys lists API keys for an organization.
func (s *APIKeyService) ListAPIKeys(ctx context.Context, req ListAPIKeysRequest) ([]*domain.APIKey, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	dbKeys, err := s.queries.ListAPIKeysByOrganization(ctx, db.ListAPIKeysByOrganizationParams{
		OrganizationID: req.OrganizationID,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	keys := make([]*domain.APIKey, len(dbKeys))
	for i, dbKey := range dbKeys {
		keys[i] = s.dbToDomain(dbKey)
	}

	return keys, nil
}

// UpdateAPIKeyRequest contains the parameters for updating an API key.
type UpdateAPIKeyRequest struct {
	KeyID              uuid.UUID
	Name               *string
	Scopes             []domain.APIKeyScope
	RateLimitPerMinute *int
	IPAllowlist        []string
	IsActive           *bool
}

// UpdateAPIKey updates an API key.
func (s *APIKeyService) UpdateAPIKey(ctx context.Context, req UpdateAPIKeyRequest) (*domain.APIKey, error) {
	params := db.UpdateAPIKeyParams{
		ID: req.KeyID,
	}

	if req.Name != nil {
		params.Name = req.Name
	}

	if req.Scopes != nil {
		// Validate scopes
		for _, scope := range req.Scopes {
			if !scope.IsValid() {
				return nil, fmt.Errorf("%w: invalid scope %s", domain.ErrValidation, scope)
			}
		}
		scopesJSON, err := json.Marshal(scopesToStrings(req.Scopes))
		if err != nil {
			return nil, fmt.Errorf("failed to marshal scopes: %w", err)
		}
		params.Scopes = scopesJSON
	}

	if req.RateLimitPerMinute != nil {
		rateLimit := int32(*req.RateLimitPerMinute)
		params.RateLimitPerMinute = &rateLimit
	}

	if req.IPAllowlist != nil {
		ipAllowlistJSON, err := json.Marshal(req.IPAllowlist)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal IP allowlist: %w", err)
		}
		params.IpAllowlist = ipAllowlistJSON
	}

	if req.IsActive != nil {
		params.IsActive = req.IsActive
	}

	dbKey, err := s.queries.UpdateAPIKey(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update API key: %w", err)
	}

	return s.dbToDomain(dbKey), nil
}

// RevokeAPIKey permanently revokes an API key.
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error {
	_, err := s.queries.RevokeAPIKey(ctx, keyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}

// CountAPIKeys returns the count of API keys for an organization.
func (s *APIKeyService) CountAPIKeys(ctx context.Context, organizationID uuid.UUID) (int64, error) {
	count, err := s.queries.CountAPIKeysByOrganization(ctx, organizationID)
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}
	return count, nil
}

// ClearExpiredPreviousKeys clears expired previous key hashes from rotated keys.
func (s *APIKeyService) ClearExpiredPreviousKeys(ctx context.Context) error {
	return s.queries.ClearExpiredPreviousKeys(ctx)
}

// dbToDomain converts a database API key to a domain API key.
func (s *APIKeyService) dbToDomain(dbKey *db.ApiKey) *domain.APIKey {
	key := &domain.APIKey{
		ID:                 dbKey.ID,
		OrganizationID:     dbKey.OrganizationID,
		Name:               dbKey.Name,
		KeyPrefix:          dbKey.KeyPrefix,
		KeyHash:            dbKey.KeyHash,
		RateLimitPerMinute: int(dbKey.RateLimitPerMinute),
		IsActive:           dbKey.IsActive,
		CreatedAt:          dbKey.CreatedAt.Time,
		UpdatedAt:          dbKey.UpdatedAt.Time,
	}

	// Parse scopes
	var scopeStrings []string
	if err := json.Unmarshal(dbKey.Scopes, &scopeStrings); err == nil {
		key.Scopes = domain.ScopesFromStrings(scopeStrings)
	}

	// Parse IP allowlist
	var ipAllowlist []string
	if err := json.Unmarshal(dbKey.IpAllowlist, &ipAllowlist); err == nil {
		key.IPAllowlist = ipAllowlist
	}

	// Handle nullable fields
	if dbKey.PreviousKeyHash != nil {
		key.PreviousKeyHash = dbKey.PreviousKeyHash
	}

	if dbKey.PreviousKeyExpiresAt.Valid {
		t := dbKey.PreviousKeyExpiresAt.Time
		key.PreviousKeyExpiresAt = &t
	}

	if dbKey.ExpiresAt.Valid {
		t := dbKey.ExpiresAt.Time
		key.ExpiresAt = &t
	}

	if dbKey.LastUsedAt.Valid {
		t := dbKey.LastUsedAt.Time
		key.LastUsedAt = &t
	}

	if dbKey.RevokedAt.Valid {
		t := dbKey.RevokedAt.Time
		key.RevokedAt = &t
	}

	return key
}

// generateAPIKey generates a new API key with prefix, hash, and the raw key.
// Returns: secretKey, keyPrefix, keyHash, error
func generateAPIKey() (string, string, string, error) {
	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create the secret key with prefix
	secretKey := "sk_live_" + hex.EncodeToString(randomBytes)

	// Extract prefix (first 8 chars after the prefix)
	keyPrefix := secretKey[8:16]

	// Hash the full key
	keyHash := hashAPIKey(secretKey)

	return secretKey, keyPrefix, keyHash, nil
}

// hashAPIKey creates a SHA-256 hash of an API key.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// scopesToStrings converts a slice of APIKeyScope to a slice of strings.
func scopesToStrings(scopes []domain.APIKeyScope) []string {
	result := make([]string, len(scopes))
	for i, s := range scopes {
		result[i] = string(s)
	}
	return result
}
