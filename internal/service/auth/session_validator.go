package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// SessionValidatorAdapter adapts SessionService to implement middleware.SessionValidator.
type SessionValidatorAdapter struct {
	queries *db.Queries
}

// NewSessionValidatorAdapter creates a new session validator adapter.
func NewSessionValidatorAdapter(queries *db.Queries) *SessionValidatorAdapter {
	return &SessionValidatorAdapter{
		queries: queries,
	}
}

// ValidateSession validates a session token hash and returns the session info.
// Note: The tokenHash is already hashed by the middleware.
func (a *SessionValidatorAdapter) ValidateSession(ctx context.Context, tokenHash string) (*middleware.SessionInfo, error) {
	// Get session with member using the token hash
	sessionWithMember, err := a.queries.GetSessionWithMember(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Update session activity asynchronously
	go func() {
		_ = a.queries.UpdateSessionActivity(context.Background(), sessionWithMember.ID)
	}()

	return &middleware.SessionInfo{
		ID:             sessionWithMember.ID,
		MemberID:       sessionWithMember.MemberID,
		OrganizationID: sessionWithMember.OrganizationID,
		IPAddress:      sessionWithMember.IpAddress.String(),
		UserAgent:      stringPtrToString(sessionWithMember.UserAgent),
		ExpiresAt:      sessionWithMember.ExpiresAt.Time,
		LastActivityAt: sessionWithMember.LastActivityAt.Time,
	}, nil
}

// GetMember retrieves member information.
func (a *SessionValidatorAdapter) GetMember(ctx context.Context, memberID uuid.UUID) (*middleware.MemberInfo, error) {
	memberWithRole, err := a.queries.GetMemberWithRole(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	// Parse role permissions
	var permissions []string
	if err := json.Unmarshal(memberWithRole.RolePermissions, &permissions); err != nil {
		permissions = []string{}
	}

	return &middleware.MemberInfo{
		ID:             memberWithRole.ID,
		OrganizationID: memberWithRole.OrganizationID,
		Email:          memberWithRole.Email,
		Name:           memberWithRole.Name,
		RoleID:         memberWithRole.RoleID,
		Permissions:    permissions,
		Status:         memberWithRole.Status,
	}, nil
}

// UpdateSessionActivity updates the last activity timestamp for a session.
func (a *SessionValidatorAdapter) UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error {
	return a.queries.UpdateSessionActivity(ctx, sessionID)
}

// stringPtrToString converts a string pointer to a string.
func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
