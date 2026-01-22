// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
)

// Tenant context keys.
type tenantContextKey string

const (
	// TenantContextKey is the context key for tenant info.
	TenantContextKey tenantContextKey = "tenant"
	// DBConnContextKey is the context key for the database connection with RLS set.
	DBConnContextKey tenantContextKey = "db_conn"
)

// TenantInfo contains tenant information for the current request.
type TenantInfo struct {
	// OrganizationID is the tenant's organization ID.
	OrganizationID uuid.UUID
	// OrganizationSlug is the tenant's organization slug.
	OrganizationSlug string
	// OrganizationStatus is the tenant's organization status.
	OrganizationStatus string
	// ActorID is the ID of the actor making the request.
	ActorID uuid.UUID
	// ActorType is the type of actor ("api_key", "member", or "system").
	ActorType string
}

// OrganizationLoader is the interface for loading organization information.
type OrganizationLoader interface {
	// GetOrganization retrieves organization information by ID.
	GetOrganization(ctx context.Context, orgID uuid.UUID) (*OrganizationInfo, error)
}

// OrganizationInfo contains organization information.
type OrganizationInfo struct {
	ID     uuid.UUID
	Slug   string
	Status string
}

// TenantConfig holds configuration for the tenant middleware.
type TenantConfig struct {
	// DBPool is the database connection pool.
	DBPool *pgxpool.Pool
	// OrganizationLoader loads organization information.
	OrganizationLoader OrganizationLoader
	// Logger is the logger for tenant events.
	Logger *observability.Logger
	// RequireTenant determines if tenant context is required.
	RequireTenant bool
	// SkipPaths are paths that don't require tenant context.
	SkipPaths []string
}

// Tenant returns a middleware that injects organization context and sets up RLS.
// It extracts the organization ID from the auth context and:
// 1. Validates the organization exists and is active
// 2. Sets PostgreSQL session variables for RLS
// 3. Adds tenant info to the request context
func Tenant(cfg TenantConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should skip tenant context
			if shouldSkipTenant(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Get auth info from context
			authInfo := GetAuthInfo(r.Context())
			if authInfo == nil {
				if cfg.RequireTenant {
					writeTenantError(w, r, "AUTH_INVALID", "Authentication required for tenant context", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Validate organization
			var orgInfo *OrganizationInfo
			var err error

			if cfg.OrganizationLoader != nil {
				orgInfo, err = cfg.OrganizationLoader.GetOrganization(r.Context(), authInfo.OrganizationID)
				if err != nil {
					if errors.Is(err, domain.ErrOrganizationNotFound) {
						writeTenantError(w, r, "NOT_FOUND", "Organization not found", http.StatusNotFound)
						return
					}
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to load organization",
							"error", err.Error(),
							"organization_id", authInfo.OrganizationID.String(),
						)
					}
					writeTenantError(w, r, "INTERNAL_ERROR", "Failed to load organization", http.StatusInternalServerError)
					return
				}

				// Check organization status
				if orgInfo.Status == "suspended" {
					writeTenantError(w, r, "FORBIDDEN", "Organization is suspended", http.StatusForbidden)
					return
				}
				if orgInfo.Status == "closed" {
					writeTenantError(w, r, "FORBIDDEN", "Organization is closed", http.StatusForbidden)
					return
				}
			}

			// Create tenant info
			tenantInfo := &TenantInfo{
				OrganizationID: authInfo.OrganizationID,
				ActorID:        authInfo.ActorID,
				ActorType:      authInfo.ActorType,
			}
			if orgInfo != nil {
				tenantInfo.OrganizationSlug = orgInfo.Slug
				tenantInfo.OrganizationStatus = orgInfo.Status
			}

			// Add tenant info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, TenantContextKey, tenantInfo)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetRLSContext sets the PostgreSQL session variables for Row-Level Security.
// This should be called at the beginning of each database transaction.
func SetRLSContext(ctx context.Context, conn interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}) error {
	tenantInfo := GetTenantInfo(ctx)
	if tenantInfo == nil {
		return errors.New("tenant context not found")
	}

	// Set PostgreSQL session variables for RLS
	// These variables are used by RLS policies to filter data
	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql:  "SET LOCAL app.current_organization_id = $1",
			args: []any{tenantInfo.OrganizationID.String()},
		},
		{
			sql:  "SET LOCAL app.current_actor_id = $1",
			args: []any{tenantInfo.ActorID.String()},
		},
		{
			sql:  "SET LOCAL app.current_actor_type = $1",
			args: []any{tenantInfo.ActorType},
		},
	}

	// Add trace ID if available
	traceID := GetTraceID(ctx)
	if traceID != "" {
		queries = append(queries, struct {
			sql  string
			args []any
		}{
			sql:  "SET LOCAL app.trace_id = $1",
			args: []any{traceID},
		})
	}

	for _, q := range queries {
		if _, err := conn.Exec(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}

	return nil
}

// SetRLSContextStdDB sets the PostgreSQL session variables for Row-Level Security using standard database/sql.
func SetRLSContextStdDB(ctx context.Context, conn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}) error {
	tenantInfo := GetTenantInfo(ctx)
	if tenantInfo == nil {
		return errors.New("tenant context not found")
	}

	// Set PostgreSQL session variables for RLS
	queries := []struct {
		sql  string
		args []any
	}{
		{
			sql:  "SET LOCAL app.current_organization_id = $1",
			args: []any{tenantInfo.OrganizationID.String()},
		},
		{
			sql:  "SET LOCAL app.current_actor_id = $1",
			args: []any{tenantInfo.ActorID.String()},
		},
		{
			sql:  "SET LOCAL app.current_actor_type = $1",
			args: []any{tenantInfo.ActorType},
		},
	}

	// Add trace ID if available
	traceID := GetTraceID(ctx)
	if traceID != "" {
		queries = append(queries, struct {
			sql  string
			args []any
		}{
			sql:  "SET LOCAL app.trace_id = $1",
			args: []any{traceID},
		})
	}

	for _, q := range queries {
		if _, err := conn.ExecContext(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}

	return nil
}

// WithRLSContext executes a function with RLS context set on the connection.
// This is a helper for executing queries with proper tenant isolation.
func WithRLSContext(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, conn *pgxpool.Conn) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Start a transaction to ensure SET LOCAL persists
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// Set RLS context
	if err = SetRLSContext(ctx, tx); err != nil {
		return err
	}

	// Execute the function
	if err = fn(ctx, conn); err != nil {
		return err
	}

	// Commit the transaction
	return tx.Commit(ctx)
}

// GetTenantInfo extracts the tenant info from the context.
func GetTenantInfo(ctx context.Context) *TenantInfo {
	if tenantInfo, ok := ctx.Value(TenantContextKey).(*TenantInfo); ok {
		return tenantInfo
	}
	return nil
}

// GetTenantOrganizationID extracts the organization ID from the tenant context.
func GetTenantOrganizationID(ctx context.Context) uuid.UUID {
	tenantInfo := GetTenantInfo(ctx)
	if tenantInfo != nil {
		return tenantInfo.OrganizationID
	}
	return uuid.Nil
}

// RequireTenant returns a middleware that requires tenant context.
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantInfo := GetTenantInfo(r.Context())
			if tenantInfo == nil {
				writeTenantError(w, r, "AUTH_INVALID", "Tenant context required", http.StatusUnauthorized)
				return
			}

			if tenantInfo.OrganizationID == uuid.Nil {
				writeTenantError(w, r, "AUTH_INVALID", "Valid organization required", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOrganizationStatus returns a middleware that requires a specific organization status.
func RequireOrganizationStatus(allowedStatuses ...string) func(http.Handler) http.Handler {
	statusSet := make(map[string]struct{})
	for _, s := range allowedStatuses {
		statusSet[s] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantInfo := GetTenantInfo(r.Context())
			if tenantInfo == nil {
				writeTenantError(w, r, "AUTH_INVALID", "Tenant context required", http.StatusUnauthorized)
				return
			}

			if _, ok := statusSet[tenantInfo.OrganizationStatus]; !ok {
				writeTenantError(w, r, "FORBIDDEN", "Organization status does not allow this action", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// shouldSkipTenant checks if the path should skip tenant context.
func shouldSkipTenant(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		if path == skip || (len(skip) > 0 && skip[len(skip)-1] == '*' && len(path) >= len(skip)-1 && path[:len(skip)-1] == skip[:len(skip)-1]) {
			return true
		}
	}
	return false
}

// writeTenantError writes a tenant error response.
func writeTenantError(w http.ResponseWriter, r *http.Request, code, detail string, status int) {
	writeAuthError(w, r, code, detail, status)
}

// TenantAwareHandler is a helper type for handlers that need tenant context.
type TenantAwareHandler struct {
	pool   *pgxpool.Pool
	logger *observability.Logger
}

// NewTenantAwareHandler creates a new tenant-aware handler.
func NewTenantAwareHandler(pool *pgxpool.Pool, logger *observability.Logger) *TenantAwareHandler {
	return &TenantAwareHandler{
		pool:   pool,
		logger: logger,
	}
}

// WithTx executes a function within a transaction with RLS context set.
func (h *TenantAwareHandler) WithTx(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}

	// Set RLS context
	if err := SetRLSContext(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	// Execute the function
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

// WithConn executes a function with a connection that has RLS context set.
func (h *TenantAwareHandler) WithConn(ctx context.Context, fn func(ctx context.Context, conn *pgxpool.Conn) error) error {
	return WithRLSContext(ctx, h.pool, fn)
}
