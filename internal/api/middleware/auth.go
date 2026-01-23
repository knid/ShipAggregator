// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
)

// Authentication context keys.
type authContextKey string

const (
	// AuthContextKey is the context key for authentication info.
	AuthContextKey authContextKey = "auth"
	// APIKeyContextKey is the context key for API key info.
	APIKeyContextKey authContextKey = "api_key"
	// SessionContextKey is the context key for session info.
	SessionContextKey authContextKey = "session"
	// MemberContextKey is the context key for member info.
	MemberContextKey authContextKey = "member"
)

// AuthType represents the type of authentication used.
type AuthType string

const (
	// AuthTypeAPIKey indicates API key authentication.
	AuthTypeAPIKey AuthType = "api_key"
	// AuthTypeSession indicates session-based authentication.
	AuthTypeSession AuthType = "session"
	// AuthTypeNone indicates no authentication.
	AuthTypeNone AuthType = "none"
)

// AuthInfo contains authentication information extracted from the request.
type AuthInfo struct {
	// Type is the authentication type used.
	Type AuthType
	// OrganizationID is the authenticated organization's ID.
	OrganizationID uuid.UUID
	// ActorID is the ID of the authenticated actor (API key ID or member ID).
	ActorID uuid.UUID
	// ActorType is the type of actor ("api_key" or "member").
	ActorType string
	// Scopes contains the allowed scopes for API key auth.
	Scopes []string
	// Permissions contains the allowed permissions for session auth.
	Permissions []string
	// APIKeyID is the API key ID (if API key auth).
	APIKeyID uuid.UUID
	// MemberID is the member ID (if session auth).
	MemberID uuid.UUID
}

// HasScope checks if the auth info has a specific scope.
func (a *AuthInfo) HasScope(scope string) bool {
	if len(a.Scopes) == 0 {
		// Empty scopes means all scopes are allowed
		return true
	}
	for _, s := range a.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

// HasPermission checks if the auth info has a specific permission.
func (a *AuthInfo) HasPermission(permission string) bool {
	for _, p := range a.Permissions {
		if p == permission || p == "*" {
			return true
		}
		// Check for wildcard permissions like "shipment:*"
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(permission, prefix) {
				return true
			}
		}
	}
	return false
}

// APIKeyInfo contains information about an authenticated API key.
type APIKeyInfo struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	KeyPrefix      string
	Scopes         []string
	IPAllowlist    []string
	RateLimit      int
	Status         string
	ExpiresAt      *time.Time
	LastUsedAt     *time.Time
}

// SessionInfo contains information about an authenticated session.
type SessionInfo struct {
	ID             uuid.UUID
	MemberID       uuid.UUID
	OrganizationID uuid.UUID
	IPAddress      string
	UserAgent      string
	ExpiresAt      time.Time
	LastActivityAt time.Time
}

// MemberInfo contains information about an authenticated member.
type MemberInfo struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Name           string
	RoleID         uuid.UUID
	Permissions    []string
	Status         string
}

// APIKeyValidator is the interface for validating API keys.
type APIKeyValidator interface {
	// ValidateAPIKey validates an API key and returns the key info.
	ValidateAPIKey(ctx context.Context, keyHash string) (*APIKeyInfo, error)
	// UpdateLastUsed updates the last used timestamp for an API key.
	UpdateLastUsed(ctx context.Context, keyID uuid.UUID) error
}

// SessionValidator is the interface for validating sessions.
type SessionValidator interface {
	// ValidateSession validates a session token and returns the session info.
	ValidateSession(ctx context.Context, tokenHash string) (*SessionInfo, error)
	// GetMember retrieves member information.
	GetMember(ctx context.Context, memberID uuid.UUID) (*MemberInfo, error)
	// UpdateSessionActivity updates the last activity timestamp for a session.
	UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error
}

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	// APIKeyValidator validates API keys.
	APIKeyValidator APIKeyValidator
	// SessionValidator validates sessions.
	SessionValidator SessionValidator
	// Logger is the logger for auth events.
	Logger *observability.Logger
	// SkipPaths are paths that don't require authentication.
	SkipPaths []string
	// RequireAuth determines if authentication is required.
	RequireAuth bool
}

// Auth returns a middleware that handles API key and session authentication.
// It extracts the authentication token from the Authorization header and validates it.
// For API keys: Authorization: Bearer sk_live_xxx
// For sessions: Authorization: Bearer sess_xxx or via cookie
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should skip authentication
			if shouldSkipAuth(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			token := extractBearerToken(r)
			if token == "" {
				// Try to extract from cookie for session auth
				token = extractSessionCookie(r)
			}

			if token == "" {
				if cfg.RequireAuth {
					writeAuthError(w, r, "AUTH_INVALID", "Authentication required", http.StatusUnauthorized)
					return
				}
				// No auth required, continue without auth context
				next.ServeHTTP(w, r)
				return
			}

			// Determine auth type based on token prefix
			authInfo, err := authenticateToken(r.Context(), token, r, cfg)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Warn("authentication failed",
						"error", err.Error(),
						"path", r.URL.Path,
						"method", r.Method,
					)
				}
				handleAuthError(w, r, err)
				return
			}

			// Add auth info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, AuthContextKey, authInfo)
			ctx = observability.ContextWithOrganizationID(ctx, authInfo.OrganizationID.String())
			ctx = observability.ContextWithActor(ctx, authInfo.ActorID.String(), authInfo.ActorType)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateToken authenticates a token and returns the auth info.
func authenticateToken(ctx context.Context, token string, r *http.Request, cfg AuthConfig) (*AuthInfo, error) {
	// Determine auth type based on token prefix
	if strings.HasPrefix(token, "sk_") {
		return authenticateAPIKey(ctx, token, r, cfg)
	}
	if strings.HasPrefix(token, "sess_") {
		return authenticateSession(ctx, token, r, cfg)
	}

	// Unknown token format
	return nil, domain.ErrUnauthorized
}

// authenticateAPIKey authenticates an API key.
func authenticateAPIKey(ctx context.Context, token string, r *http.Request, cfg AuthConfig) (*AuthInfo, error) {
	if cfg.APIKeyValidator == nil {
		return nil, errors.New("API key validation not configured")
	}

	// Hash the token for lookup
	keyHash := hashToken(token)

	// Validate the API key
	keyInfo, err := cfg.APIKeyValidator.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		if errors.Is(err, domain.ErrAPIKeyNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	// Check if key is active
	if keyInfo.Status != "active" {
		if keyInfo.Status == "revoked" {
			return nil, domain.ErrAPIKeyRevoked
		}
		return nil, domain.ErrAPIKeyInactive
	}

	// Check if key is expired
	if keyInfo.ExpiresAt != nil && keyInfo.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrAPIKeyExpired
	}

	// Check IP allowlist
	if len(keyInfo.IPAllowlist) > 0 {
		clientIP := GetClientIP(r)
		if !isIPAllowed(clientIP, keyInfo.IPAllowlist) {
			return nil, domain.ErrAPIKeyIPNotAllowed
		}
	}

	// Update last used timestamp (async, don't block request)
	go func() {
		_ = cfg.APIKeyValidator.UpdateLastUsed(context.Background(), keyInfo.ID)
	}()

	return &AuthInfo{
		Type:           AuthTypeAPIKey,
		OrganizationID: keyInfo.OrganizationID,
		ActorID:        keyInfo.ID,
		ActorType:      "api_key",
		Scopes:         keyInfo.Scopes,
		APIKeyID:       keyInfo.ID,
	}, nil
}

// authenticateSession authenticates a session token.
func authenticateSession(ctx context.Context, token string, r *http.Request, cfg AuthConfig) (*AuthInfo, error) {
	if cfg.SessionValidator == nil {
		return nil, errors.New("session validation not configured")
	}

	// Hash the token for lookup
	tokenHash := hashToken(token)

	// Validate the session
	sessionInfo, err := cfg.SessionValidator.ValidateSession(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	// Check if session is expired
	if sessionInfo.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrSessionExpired
	}

	// Get member info
	memberInfo, err := cfg.SessionValidator.GetMember(ctx, sessionInfo.MemberID)
	if err != nil {
		return nil, err
	}

	// Check if member is active
	if memberInfo.Status != "active" {
		if memberInfo.Status == "suspended" {
			return nil, domain.ErrMemberSuspended
		}
		if memberInfo.Status == "removed" {
			return nil, domain.ErrMemberRemoved
		}
		return nil, domain.ErrUnauthorized
	}

	// Update session activity (async, don't block request)
	go func() {
		_ = cfg.SessionValidator.UpdateSessionActivity(context.Background(), sessionInfo.ID)
	}()

	return &AuthInfo{
		Type:           AuthTypeSession,
		OrganizationID: sessionInfo.OrganizationID,
		ActorID:        memberInfo.ID,
		ActorType:      "member",
		Permissions:    memberInfo.Permissions,
		MemberID:       memberInfo.ID,
	}, nil
}

// RequireScope returns a middleware that requires a specific scope.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authInfo := GetAuthInfo(r.Context())
			if authInfo == nil {
				writeAuthError(w, r, "AUTH_INVALID", "Authentication required", http.StatusUnauthorized)
				return
			}

			// Session auth uses permissions, not scopes
			if authInfo.Type == AuthTypeSession {
				if !authInfo.HasPermission(scope) {
					writeAuthError(w, r, "FORBIDDEN", "Permission denied: "+scope, http.StatusForbidden)
					return
				}
			} else if authInfo.Type == AuthTypeAPIKey {
				if !authInfo.HasScope(scope) {
					writeAuthError(w, r, "FORBIDDEN", "API key missing scope: "+scope, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a middleware that requires a specific permission.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return RequireScope(permission)
}

// GetAuthInfo extracts the auth info from the context.
func GetAuthInfo(ctx context.Context) *AuthInfo {
	if authInfo, ok := ctx.Value(AuthContextKey).(*AuthInfo); ok {
		return authInfo
	}
	return nil
}

// GetOrganizationID extracts the organization ID from the context.
func GetOrganizationID(ctx context.Context) uuid.UUID {
	authInfo := GetAuthInfo(ctx)
	if authInfo != nil {
		return authInfo.OrganizationID
	}
	return uuid.Nil
}

// GetActorID extracts the actor ID from the context.
func GetActorID(ctx context.Context) uuid.UUID {
	authInfo := GetAuthInfo(ctx)
	if authInfo != nil {
		return authInfo.ActorID
	}
	return uuid.Nil
}

// GetActorType extracts the actor type from the context.
func GetActorType(ctx context.Context) string {
	authInfo := GetAuthInfo(ctx)
	if authInfo != nil {
		return authInfo.ActorType
	}
	return ""
}

// Helper functions

// extractBearerToken extracts the bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Check for Bearer prefix
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}

	return strings.TrimPrefix(auth, prefix)
}

// extractSessionCookie extracts the session token from a cookie.
func extractSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// hashToken computes the SHA-256 hash of a token.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetClientIP extracts the client IP from the request.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// isIPAllowed checks if an IP is in the allowlist.
func isIPAllowed(clientIP string, allowlist []string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, entry := range allowlist {
		// Check if entry is a CIDR
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			if network.Contains(ip) {
				return true
			}
		} else {
			// Single IP
			if entry == clientIP {
				return true
			}
		}
	}

	return false
}

// shouldSkipAuth checks if the path should skip authentication.
func shouldSkipAuth(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		if path == skip || strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// handleAuthError handles authentication errors and writes the appropriate response.
func handleAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		writeAuthError(w, r, "AUTH_INVALID", "Invalid authentication credentials", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrAPIKeyExpired):
		writeAuthError(w, r, "AUTH_INVALID", "API key has expired", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrAPIKeyRevoked):
		writeAuthError(w, r, "AUTH_INVALID", "API key has been revoked", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrAPIKeyInactive):
		writeAuthError(w, r, "AUTH_INVALID", "API key is inactive", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrAPIKeyIPNotAllowed):
		writeAuthError(w, r, "FORBIDDEN", "IP address not allowed", http.StatusForbidden)
	case errors.Is(err, domain.ErrSessionExpired):
		writeAuthError(w, r, "AUTH_INVALID", "Session has expired", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrMemberSuspended):
		writeAuthError(w, r, "FORBIDDEN", "Account is suspended", http.StatusForbidden)
	case errors.Is(err, domain.ErrMemberRemoved):
		writeAuthError(w, r, "FORBIDDEN", "Account has been removed", http.StatusForbidden)
	default:
		writeAuthError(w, r, "AUTH_INVALID", "Authentication failed", http.StatusUnauthorized)
	}
}

// writeAuthError writes an authentication error response.
// This is a simple implementation; the full RFC 7807 response is in handler/errors.go
func writeAuthError(w http.ResponseWriter, r *http.Request, code, detail string, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	traceID := GetTraceID(r.Context())

	// Simple JSON response - full RFC 7807 implementation is in handler/errors.go
	response := `{"type":"https://api.shipaggregator.com/errors/` + strings.ToLower(strings.ReplaceAll(code, "_", "-")) + `","title":"` + getErrorTitle(code) + `","status":` + strconv.Itoa(status) + `,"detail":"` + detail + `","code":"` + code + `","trace_id":"` + traceID + `"}`
	_, _ = w.Write([]byte(response))
}

// getErrorTitle returns the title for an error code.
func getErrorTitle(code string) string {
	switch code {
	case "AUTH_INVALID":
		return "Authentication Required"
	case "FORBIDDEN":
		return "Forbidden"
	default:
		return "Error"
	}
}
