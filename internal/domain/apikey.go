package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// APIKeyScope represents a permission scope for an API key.
type APIKeyScope string

const (
	// APIScopeShipmentsRead allows reading shipment data.
	APIScopeShipmentsRead APIKeyScope = "shipments:read"
	// APIScopeShipmentsWrite allows creating and modifying shipments.
	APIScopeShipmentsWrite APIKeyScope = "shipments:write"
	// APIScopeQuotesRead allows reading quotes.
	APIScopeQuotesRead APIKeyScope = "quotes:read"
	// APIScopeQuotesWrite allows creating quotes.
	APIScopeQuotesWrite APIKeyScope = "quotes:write"
	// APIScopeTrackingRead allows reading tracking information.
	APIScopeTrackingRead APIKeyScope = "tracking:read"
	// APIScopeWebhooksRead allows reading webhook configurations.
	APIScopeWebhooksRead APIKeyScope = "webhooks:read"
	// APIScopeWebhooksWrite allows managing webhook configurations.
	APIScopeWebhooksWrite APIKeyScope = "webhooks:write"
	// APIScopeBillingRead allows reading billing information.
	APIScopeBillingRead APIKeyScope = "billing:read"
	// APIScopeBillingWrite allows billing operations (topups).
	APIScopeBillingWrite APIKeyScope = "billing:write"
	// APIScopeAddressesRead allows reading addresses.
	APIScopeAddressesRead APIKeyScope = "addresses:read"
	// APIScopeAddressesWrite allows managing addresses.
	APIScopeAddressesWrite APIKeyScope = "addresses:write"
	// APIScopeProvidersRead allows reading provider information.
	APIScopeProvidersRead APIKeyScope = "providers:read"
	// APIScopeProviderAccountsRead allows reading provider accounts.
	APIScopeProviderAccountsRead APIKeyScope = "provider_accounts:read"
	// APIScopeProviderAccountsWrite allows managing provider accounts.
	APIScopeProviderAccountsWrite APIKeyScope = "provider_accounts:write"
	// APIScopeAll grants all permissions.
	APIScopeAll APIKeyScope = "*"
)

// IsValid checks if the API key scope is a valid value.
func (s APIKeyScope) IsValid() bool {
	switch s {
	case APIScopeShipmentsRead, APIScopeShipmentsWrite,
		APIScopeQuotesRead, APIScopeQuotesWrite,
		APIScopeTrackingRead,
		APIScopeWebhooksRead, APIScopeWebhooksWrite,
		APIScopeBillingRead, APIScopeBillingWrite,
		APIScopeAddressesRead, APIScopeAddressesWrite,
		APIScopeProvidersRead,
		APIScopeProviderAccountsRead, APIScopeProviderAccountsWrite,
		APIScopeAll:
		return true
	default:
		return false
	}
}

// String returns the string representation of the API key scope.
func (s APIKeyScope) String() string {
	return string(s)
}

// DefaultAPIKeyScopes returns the default scopes for a new API key.
func DefaultAPIKeyScopes() []APIKeyScope {
	return []APIKeyScope{
		APIScopeShipmentsRead, APIScopeShipmentsWrite,
		APIScopeQuotesRead, APIScopeQuotesWrite,
		APIScopeTrackingRead,
		APIScopeAddressesRead, APIScopeAddressesWrite,
		APIScopeProvidersRead,
	}
}

// AllAPIKeyScopes returns all available API key scopes.
func AllAPIKeyScopes() []APIKeyScope {
	return []APIKeyScope{
		APIScopeShipmentsRead, APIScopeShipmentsWrite,
		APIScopeQuotesRead, APIScopeQuotesWrite,
		APIScopeTrackingRead,
		APIScopeWebhooksRead, APIScopeWebhooksWrite,
		APIScopeBillingRead, APIScopeBillingWrite,
		APIScopeAddressesRead, APIScopeAddressesWrite,
		APIScopeProvidersRead,
		APIScopeProviderAccountsRead, APIScopeProviderAccountsWrite,
	}
}

// APIKey represents an API authentication credential.
type APIKey struct {
	// ID is the unique identifier for the API key.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the ID of the parent organization.
	OrganizationID uuid.UUID `json:"organization_id"`
	// Name is the key name/description.
	Name string `json:"name"`
	// KeyPrefix is the first 8 characters of the key for identification.
	KeyPrefix string `json:"key_prefix"`
	// KeyHash is the SHA-256 hash of the full API key.
	KeyHash string `json:"-"`
	// PreviousKeyHash is the previous key hash during rotation grace period.
	PreviousKeyHash *string `json:"-"`
	// PreviousKeyExpiresAt is when the previous key becomes invalid.
	PreviousKeyExpiresAt *time.Time `json:"previous_key_expires_at,omitempty"`
	// Scopes is the list of permission scopes.
	Scopes []APIKeyScope `json:"scopes"`
	// RateLimitPerMinute is the maximum requests per minute.
	RateLimitPerMinute int `json:"rate_limit_per_minute"`
	// IPAllowlist is the list of allowed IP addresses/CIDRs.
	IPAllowlist []string `json:"ip_allowlist"`
	// ExpiresAt is the key expiration timestamp (nil = no expiry).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// LastUsedAt is the timestamp of the last successful use.
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// IsActive indicates if the key is active.
	IsActive bool `json:"is_active"`
	// CreatedAt is the timestamp when the key was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the key was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// RevokedAt is the timestamp when the key was revoked.
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// NewAPIKey creates a new API key with the given parameters.
func NewAPIKey(organizationID uuid.UUID, name, keyPrefix, keyHash string, scopes []APIKeyScope) *APIKey {
	now := time.Now().UTC()
	return &APIKey{
		ID:                 uuid.New(),
		OrganizationID:     organizationID,
		Name:               name,
		KeyPrefix:          keyPrefix,
		KeyHash:            keyHash,
		Scopes:             scopes,
		RateLimitPerMinute: 100, // Default rate limit
		IPAllowlist:        []string{},
		IsActive:           true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// IsValid returns true if the API key is valid for use.
func (k *APIKey) IsValid() bool {
	if !k.IsActive || k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && time.Now().UTC().After(*k.ExpiresAt) {
		return false
	}
	return true
}

// IsExpired returns true if the API key has expired.
func (k *APIKey) IsExpired() bool {
	return k.ExpiresAt != nil && time.Now().UTC().After(*k.ExpiresAt)
}

// IsRevoked returns true if the API key has been revoked.
func (k *APIKey) IsRevoked() bool {
	return k.RevokedAt != nil
}

// HasScope checks if the API key has a specific scope.
func (k *APIKey) HasScope(scope APIKeyScope) bool {
	for _, s := range k.Scopes {
		if s == APIScopeAll || s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the API key has any of the specified scopes.
func (k *APIKey) HasAnyScope(scopes ...APIKeyScope) bool {
	for _, scope := range scopes {
		if k.HasScope(scope) {
			return true
		}
	}
	return false
}

// IsIPAllowed checks if the given IP address is allowed.
func (k *APIKey) IsIPAllowed(ip string) bool {
	// If no allowlist is configured, all IPs are allowed
	if len(k.IPAllowlist) == 0 {
		return true
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, allowed := range k.IPAllowlist {
		// Check if it's a CIDR
		_, network, err := net.ParseCIDR(allowed)
		if err == nil {
			if network.Contains(parsedIP) {
				return true
			}
			continue
		}

		// Check if it's a single IP
		if net.ParseIP(allowed) != nil && allowed == ip {
			return true
		}
	}

	return false
}

// IsPreviousKeyValid checks if the previous key is still valid during rotation.
func (k *APIKey) IsPreviousKeyValid() bool {
	if k.PreviousKeyHash == nil || k.PreviousKeyExpiresAt == nil {
		return false
	}
	return time.Now().UTC().Before(*k.PreviousKeyExpiresAt)
}

// Rotate rotates the API key with a new hash and sets a grace period for the old key.
func (k *APIKey) Rotate(newKeyPrefix, newKeyHash string, gracePeriod time.Duration) {
	now := time.Now().UTC()
	expiresAt := now.Add(gracePeriod)

	k.PreviousKeyHash = &k.KeyHash
	k.PreviousKeyExpiresAt = &expiresAt
	k.KeyPrefix = newKeyPrefix
	k.KeyHash = newKeyHash
	k.UpdatedAt = now
}

// ClearRotation clears the rotation state (previous key).
func (k *APIKey) ClearRotation() {
	k.PreviousKeyHash = nil
	k.PreviousKeyExpiresAt = nil
	k.UpdatedAt = time.Now().UTC()
}

// Revoke revokes the API key.
func (k *APIKey) Revoke() {
	now := time.Now().UTC()
	k.RevokedAt = &now
	k.IsActive = false
	k.UpdatedAt = now
}

// Deactivate deactivates the API key without revoking it.
func (k *APIKey) Deactivate() {
	k.IsActive = false
	k.UpdatedAt = time.Now().UTC()
}

// Activate activates the API key.
func (k *APIKey) Activate() {
	k.IsActive = true
	k.UpdatedAt = time.Now().UTC()
}

// UpdateName updates the API key name.
func (k *APIKey) UpdateName(name string) {
	k.Name = name
	k.UpdatedAt = time.Now().UTC()
}

// UpdateScopes updates the API key scopes.
func (k *APIKey) UpdateScopes(scopes []APIKeyScope) {
	k.Scopes = scopes
	k.UpdatedAt = time.Now().UTC()
}

// UpdateRateLimit updates the rate limit.
func (k *APIKey) UpdateRateLimit(limit int) {
	k.RateLimitPerMinute = limit
	k.UpdatedAt = time.Now().UTC()
}

// UpdateIPAllowlist updates the IP allowlist.
func (k *APIKey) UpdateIPAllowlist(ips []string) {
	k.IPAllowlist = ips
	k.UpdatedAt = time.Now().UTC()
}

// SetExpiration sets the expiration time.
func (k *APIKey) SetExpiration(expiresAt time.Time) {
	k.ExpiresAt = &expiresAt
	k.UpdatedAt = time.Now().UTC()
}

// ClearExpiration removes the expiration time.
func (k *APIKey) ClearExpiration() {
	k.ExpiresAt = nil
	k.UpdatedAt = time.Now().UTC()
}

// RecordUsage records a successful API key usage.
func (k *APIKey) RecordUsage() {
	now := time.Now().UTC()
	k.LastUsedAt = &now
}

// ScopesToStrings converts scopes to string slice.
func (k *APIKey) ScopesToStrings() []string {
	result := make([]string, len(k.Scopes))
	for i, s := range k.Scopes {
		result[i] = string(s)
	}
	return result
}

// ScopesFromStrings converts string slice to scopes.
func ScopesFromStrings(strs []string) []APIKeyScope {
	result := make([]APIKeyScope, len(strs))
	for i, s := range strs {
		result[i] = APIKeyScope(s)
	}
	return result
}

// Session represents a dashboard authentication session.
type Session struct {
	// ID is the unique session identifier.
	ID uuid.UUID `json:"id"`
	// MemberID is the ID of the authenticated member.
	MemberID uuid.UUID `json:"member_id"`
	// TokenHash is the SHA-256 hash of the session token.
	TokenHash string `json:"-"`
	// IPAddress is the client IP address.
	IPAddress string `json:"ip_address"`
	// UserAgent is the client user agent.
	UserAgent *string `json:"user_agent,omitempty"`
	// ExpiresAt is the session expiration timestamp.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is the timestamp when the session was created.
	CreatedAt time.Time `json:"created_at"`
	// LastActivityAt is the timestamp of the last activity.
	LastActivityAt time.Time `json:"last_activity_at"`
}

// NewSession creates a new session.
func NewSession(memberID uuid.UUID, tokenHash, ipAddress string, userAgent *string, duration time.Duration) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:             uuid.New(),
		MemberID:       memberID,
		TokenHash:      tokenHash,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		ExpiresAt:      now.Add(duration),
		CreatedAt:      now,
		LastActivityAt: now,
	}
}

// IsValid returns true if the session is valid.
func (s *Session) IsValid() bool {
	return time.Now().UTC().Before(s.ExpiresAt)
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// Touch updates the last activity timestamp.
func (s *Session) Touch() {
	s.LastActivityAt = time.Now().UTC()
}

// Extend extends the session expiration.
func (s *Session) Extend(duration time.Duration) {
	s.ExpiresAt = time.Now().UTC().Add(duration)
	s.LastActivityAt = time.Now().UTC()
}
