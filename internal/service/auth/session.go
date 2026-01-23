package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

const (
	// DefaultSessionDuration is the default session duration (24 hours).
	DefaultSessionDuration = 24 * time.Hour
	// MaxSessionsPerMember is the maximum number of active sessions per member.
	MaxSessionsPerMember = 5
	// SessionTokenLength is the length of the session token in bytes.
	SessionTokenLength = 32
)

// SessionService handles session management for dashboard authentication.
type SessionService struct {
	queries         *db.Queries
	sessionDuration time.Duration
	maxSessions     int
}

// SessionServiceOption is a functional option for SessionService.
type SessionServiceOption func(*SessionService)

// WithSessionDuration sets the session duration.
func WithSessionDuration(d time.Duration) SessionServiceOption {
	return func(s *SessionService) {
		s.sessionDuration = d
	}
}

// WithMaxSessions sets the maximum sessions per member.
func WithMaxSessions(max int) SessionServiceOption {
	return func(s *SessionService) {
		s.maxSessions = max
	}
}

// NewSessionService creates a new session service.
func NewSessionService(queries *db.Queries, opts ...SessionServiceOption) *SessionService {
	s := &SessionService{
		queries:         queries,
		sessionDuration: DefaultSessionDuration,
		maxSessions:     MaxSessionsPerMember,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// RegisterRequest contains the parameters for registering a new organization and owner.
type RegisterRequest struct {
	// Organization details
	OrganizationName string
	// Owner member details
	Email    string
	Name     string
	Password string
	// Session details
	IPAddress string
	UserAgent *string
}

// RegisterResult contains the result of a successful registration.
type RegisterResult struct {
	SessionToken   string
	SessionID      uuid.UUID
	Member         *domain.Member
	Role           *domain.Role
	OrganizationID uuid.UUID
	ExpiresAt      time.Time
}

// LoginRequest contains the parameters for logging in.
type LoginRequest struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent *string
}

// LoginResult contains the result of a successful login.
type LoginResult struct {
	SessionToken   string
	SessionID      uuid.UUID
	Member         *domain.Member
	Role           *domain.Role
	OrganizationID uuid.UUID
	ExpiresAt      time.Time
}

// Login authenticates a member and creates a new session.
func (s *SessionService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	// Get member by email
	dbMember, err := s.queries.GetMemberByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	// Check member status
	if dbMember.Status != string(domain.MemberStatusActive) {
		switch dbMember.Status {
		case string(domain.MemberStatusInvited):
			return nil, domain.ErrMemberNotActivated
		case string(domain.MemberStatusSuspended):
			return nil, domain.ErrMemberSuspended
		case string(domain.MemberStatusRemoved):
			return nil, domain.ErrMemberRemoved
		default:
			return nil, domain.ErrInvalidCredentials
		}
	}

	// Check password
	if dbMember.PasswordHash == nil {
		return nil, domain.ErrMemberNotActivated
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*dbMember.PasswordHash), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Get member with role
	memberWithRole, err := s.queries.GetMemberWithRole(ctx, dbMember.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get member with role: %w", err)
	}

	// Enforce max sessions limit
	activeCount, err := s.queries.CountActiveSessionsByMember(ctx, dbMember.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to count active sessions: %w", err)
	}

	if activeCount >= int64(s.maxSessions) {
		// Delete oldest sessions to make room
		deleteCount := int(activeCount) - s.maxSessions + 1
		if err := s.queries.DeleteOldestSessionsByMember(ctx, db.DeleteOldestSessionsByMemberParams{
			MemberID: dbMember.ID,
			Limit:    int32(deleteCount),
		}); err != nil {
			return nil, fmt.Errorf("failed to delete oldest sessions: %w", err)
		}
	}

	// Generate session token
	sessionToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Parse IP address
	ipAddr, err := netip.ParseAddr(req.IPAddress)
	if err != nil {
		// Use a default IP if parsing fails
		ipAddr = netip.MustParseAddr("0.0.0.0")
	}

	// Create session
	expiresAt := time.Now().UTC().Add(s.sessionDuration)
	dbSession, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		MemberID:  dbMember.ID,
		TokenHash: tokenHash,
		IpAddress: ipAddr,
		UserAgent: req.UserAgent,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login
	if err := s.queries.UpdateMemberLastLogin(ctx, dbMember.ID); err != nil {
		// Log but don't fail
		_ = err
	}

	// Parse role permissions
	var permissions []string
	if err := json.Unmarshal(memberWithRole.RolePermissions, &permissions); err != nil {
		permissions = []string{}
	}

	return &LoginResult{
		SessionToken:   sessionToken,
		SessionID:      dbSession.ID,
		Member:         s.dbMemberToDomain(dbMember),
		Role:           s.dbRoleToDomain(memberWithRole),
		OrganizationID: dbMember.OrganizationID,
		ExpiresAt:      expiresAt,
	}, nil
}

// Register creates a new organization with an owner member and logs them in.
func (s *SessionService) Register(ctx context.Context, req RegisterRequest) (*RegisterResult, error) {
	// Check if email already exists globally
	emailExists, err := s.queries.CheckGlobalEmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if emailExists {
		return nil, domain.ErrMemberEmailTaken
	}

	// Generate organization slug from name
	slug := generateSlug(req.OrganizationName)

	// Check if slug already exists
	slugExists, err := s.queries.CheckOrganizationSlugExists(ctx, db.CheckOrganizationSlugExistsParams{
		Slug: slug,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check slug existence: %w", err)
	}
	if slugExists {
		// Append a random suffix to make it unique
		slug = slug + "-" + generateRandomSuffix()
	}

	// Hash password
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create organization
	dbOrg, err := s.queries.CreateOrganization(ctx, db.CreateOrganizationParams{
		Name:         req.OrganizationName,
		Slug:         slug,
		BillingEmail: req.Email,
		Settings:     json.RawMessage(`{}`),
		Status:       "active",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Get the system owner role
	ownerRole, err := s.queries.GetSystemRoleByName(ctx, "owner")
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("owner role not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get owner role: %w", err)
	}

	// Create owner member with active status
	dbMember, err := s.queries.CreateMember(ctx, db.CreateMemberParams{
		OrganizationID: dbOrg.ID,
		Email:          req.Email,
		Name:           req.Name,
		PasswordHash:   &passwordHash,
		RoleID:         ownerRole.ID,
		Status:         string(domain.MemberStatusActive),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create member: %w", err)
	}

	// Activate the member (set joined_at)
	now := time.Now().UTC()
	dbMember.JoinedAt = pgtype.Timestamptz{Time: now, Valid: true}

	// Generate session token
	sessionToken, tokenHash, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Parse IP address
	ipAddr, err := netip.ParseAddr(req.IPAddress)
	if err != nil {
		ipAddr = netip.MustParseAddr("0.0.0.0")
	}

	// Create session
	expiresAt := time.Now().UTC().Add(s.sessionDuration)
	dbSession, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		MemberID:  dbMember.ID,
		TokenHash: tokenHash,
		IpAddress: ipAddr,
		UserAgent: req.UserAgent,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Parse role permissions
	var permissions []string
	if err := json.Unmarshal(ownerRole.Permissions, &permissions); err != nil {
		permissions = []string{}
	}

	return &RegisterResult{
		SessionToken: sessionToken,
		SessionID:    dbSession.ID,
		Member:       s.dbMemberToDomain(dbMember),
		Role: &domain.Role{
			ID:          ownerRole.ID,
			Name:        ownerRole.Name,
			Permissions: permissions,
			IsSystem:    ownerRole.IsSystem,
		},
		OrganizationID: dbOrg.ID,
		ExpiresAt:      expiresAt,
	}, nil
}

// generateSlug generates a URL-friendly slug from a name.
func generateSlug(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := ""
	for _, r := range name {
		if r >= 'a' && r <= 'z' {
			slug += string(r)
		} else if r >= 'A' && r <= 'Z' {
			slug += string(r + 32) // Convert to lowercase
		} else if r >= '0' && r <= '9' {
			slug += string(r)
		} else if r == ' ' || r == '-' || r == '_' {
			if len(slug) > 0 && slug[len(slug)-1] != '-' {
				slug += "-"
			}
		}
	}
	// Trim trailing hyphens
	for len(slug) > 0 && slug[len(slug)-1] == '-' {
		slug = slug[:len(slug)-1]
	}
	if slug == "" {
		slug = "org"
	}
	return slug
}

// generateRandomSuffix generates a random 6-character suffix.
func generateRandomSuffix() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ValidateSessionResult contains the result of session validation.
type ValidateSessionResult struct {
	Session        *domain.Session
	Member         *domain.Member
	Role           *domain.Role
	OrganizationID uuid.UUID
}

// ValidateSession validates a session token and returns the associated member.
func (s *SessionService) ValidateSession(ctx context.Context, sessionToken string) (*ValidateSessionResult, error) {
	tokenHash := hashSessionToken(sessionToken)

	// Get session with member
	sessionWithMember, err := s.queries.GetSessionWithMember(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get full member with role
	memberWithRole, err := s.queries.GetMemberWithRole(ctx, sessionWithMember.MemberID)
	if err != nil {
		return nil, fmt.Errorf("failed to get member with role: %w", err)
	}

	// Update session activity asynchronously
	go func() {
		_ = s.queries.UpdateSessionActivity(context.Background(), sessionWithMember.ID)
	}()

	// Parse role permissions
	var permissions []string
	if err := json.Unmarshal(memberWithRole.RolePermissions, &permissions); err != nil {
		permissions = []string{}
	}

	return &ValidateSessionResult{
		Session: &domain.Session{
			ID:             sessionWithMember.ID,
			MemberID:       sessionWithMember.MemberID,
			TokenHash:      sessionWithMember.TokenHash,
			IPAddress:      sessionWithMember.IpAddress.String(),
			UserAgent:      sessionWithMember.UserAgent,
			ExpiresAt:      sessionWithMember.ExpiresAt.Time,
			CreatedAt:      sessionWithMember.CreatedAt.Time,
			LastActivityAt: sessionWithMember.LastActivityAt.Time,
		},
		Member:         s.dbMemberWithRoleToDomain(memberWithRole),
		Role:           s.dbRoleToDomain(memberWithRole),
		OrganizationID: sessionWithMember.OrganizationID,
	}, nil
}

// Logout invalidates a session.
func (s *SessionService) Logout(ctx context.Context, sessionToken string) error {
	tokenHash := hashSessionToken(sessionToken)

	if err := s.queries.DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// LogoutAll invalidates all sessions for a member.
func (s *SessionService) LogoutAll(ctx context.Context, memberID uuid.UUID) error {
	if err := s.queries.DeleteSessionsByMember(ctx, memberID); err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}

	return nil
}

// ExtendSession extends the session expiration.
func (s *SessionService) ExtendSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	newExpiresAt := time.Now().UTC().Add(s.sessionDuration)

	dbSession, err := s.queries.ExtendSession(ctx, db.ExtendSessionParams{
		ID:        sessionID,
		ExpiresAt: pgtype.Timestamptz{Time: newExpiresAt, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to extend session: %w", err)
	}

	return s.dbSessionToDomain(dbSession), nil
}

// ListSessions lists all active sessions for a member.
func (s *SessionService) ListSessions(ctx context.Context, memberID uuid.UUID) ([]*domain.Session, error) {
	dbSessions, err := s.queries.ListSessionsByMember(ctx, memberID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*domain.Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.dbSessionToDomain(dbSession)
	}

	return sessions, nil
}

// DeleteSession deletes a specific session.
func (s *SessionService) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	if err := s.queries.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all expired sessions.
func (s *SessionService) CleanupExpiredSessions(ctx context.Context) error {
	if err := s.queries.DeleteExpiredSessions(ctx); err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	return nil
}

// GetMemberByID retrieves a member by ID.
func (s *SessionService) GetMemberByID(ctx context.Context, memberID uuid.UUID) (*domain.Member, error) {
	dbMember, err := s.queries.GetMemberByID(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to get member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// GetMemberWithRole retrieves a member with their role.
func (s *SessionService) GetMemberWithRole(ctx context.Context, memberID uuid.UUID) (*domain.Member, *domain.Role, error) {
	memberWithRole, err := s.queries.GetMemberWithRole(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrMemberNotFound
		}
		return nil, nil, fmt.Errorf("failed to get member with role: %w", err)
	}

	return s.dbMemberWithRoleToDomain(memberWithRole), s.dbRoleToDomain(memberWithRole), nil
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// generateSessionToken generates a new session token and its hash.
func generateSessionToken() (string, string, error) {
	tokenBytes := make([]byte, SessionTokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	token := hex.EncodeToString(tokenBytes)
	hash := hashSessionToken(token)

	return token, hash, nil
}

// hashSessionToken creates a SHA-256 hash of a session token.
func hashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// dbSessionToDomain converts a database session to a domain session.
func (s *SessionService) dbSessionToDomain(dbSession *db.Session) *domain.Session {
	return &domain.Session{
		ID:             dbSession.ID,
		MemberID:       dbSession.MemberID,
		TokenHash:      dbSession.TokenHash,
		IPAddress:      dbSession.IpAddress.String(),
		UserAgent:      dbSession.UserAgent,
		ExpiresAt:      dbSession.ExpiresAt.Time,
		CreatedAt:      dbSession.CreatedAt.Time,
		LastActivityAt: dbSession.LastActivityAt.Time,
	}
}

// dbMemberToDomain converts a database member to a domain member.
func (s *SessionService) dbMemberToDomain(dbMember *db.Member) *domain.Member {
	member := &domain.Member{
		ID:             dbMember.ID,
		OrganizationID: dbMember.OrganizationID,
		Email:          dbMember.Email,
		Name:           dbMember.Name,
		PasswordHash:   dbMember.PasswordHash,
		RoleID:         dbMember.RoleID,
		Status:         domain.MemberStatus(dbMember.Status),
		InvitedAt:      dbMember.InvitedAt.Time,
		CreatedAt:      dbMember.CreatedAt.Time,
		UpdatedAt:      dbMember.UpdatedAt.Time,
	}

	if dbMember.JoinedAt.Valid {
		t := dbMember.JoinedAt.Time
		member.JoinedAt = &t
	}

	if dbMember.LastLoginAt.Valid {
		t := dbMember.LastLoginAt.Time
		member.LastLoginAt = &t
	}

	return member
}

// dbMemberWithRoleToDomain converts a database member with role to a domain member.
func (s *SessionService) dbMemberWithRoleToDomain(row *db.GetMemberWithRoleRow) *domain.Member {
	member := &domain.Member{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Email:          row.Email,
		Name:           row.Name,
		PasswordHash:   row.PasswordHash,
		RoleID:         row.RoleID,
		Status:         domain.MemberStatus(row.Status),
		InvitedAt:      row.InvitedAt.Time,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}

	if row.JoinedAt.Valid {
		t := row.JoinedAt.Time
		member.JoinedAt = &t
	}

	if row.LastLoginAt.Valid {
		t := row.LastLoginAt.Time
		member.LastLoginAt = &t
	}

	return member
}

// dbRoleToDomain converts a database role row to a domain role.
func (s *SessionService) dbRoleToDomain(row *db.GetMemberWithRoleRow) *domain.Role {
	var permissions []string
	if err := json.Unmarshal(row.RolePermissions, &permissions); err != nil {
		permissions = []string{}
	}

	return &domain.Role{
		ID:          row.RoleID,
		Name:        row.RoleName,
		Permissions: permissions,
		IsSystem:    row.RoleIsSystem,
	}
}
