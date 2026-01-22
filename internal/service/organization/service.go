// Package organization provides organization management services.
package organization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/auth"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Service handles organization management operations.
type Service struct {
	queries *db.Queries
}

// NewService creates a new organization service.
func NewService(queries *db.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

// GetOrganization retrieves an organization by ID.
func (s *Service) GetOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	dbOrg, err := s.queries.GetOrganizationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return s.dbToDomain(dbOrg), nil
}

// GetOrganizationBySlug retrieves an organization by slug.
func (s *Service) GetOrganizationBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	dbOrg, err := s.queries.GetOrganizationBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to get organization by slug: %w", err)
	}

	return s.dbToDomain(dbOrg), nil
}

// CreateOrganizationRequest contains the parameters for creating an organization.
type CreateOrganizationRequest struct {
	Name         string
	Slug         string
	BillingEmail string
	Settings     *domain.OrganizationSettings
}

// CreateOrganization creates a new organization.
func (s *Service) CreateOrganization(ctx context.Context, req CreateOrganizationRequest) (*domain.Organization, error) {
	// Validate slug format
	if !isValidSlug(req.Slug) {
		return nil, fmt.Errorf("%w: invalid slug format", domain.ErrValidation)
	}

	// Check if slug is already taken
	exists, err := s.queries.CheckOrganizationSlugExists(ctx, db.CheckOrganizationSlugExistsParams{
		Slug:      req.Slug,
		ExcludeID: pgtype.UUID{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check slug existence: %w", err)
	}
	if exists {
		return nil, domain.ErrOrganizationSlugTaken
	}

	// Prepare settings
	settings := domain.OrganizationSettings{}
	if req.Settings != nil {
		settings = *req.Settings
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Create organization
	dbOrg, err := s.queries.CreateOrganization(ctx, db.CreateOrganizationParams{
		Name:         req.Name,
		Slug:         req.Slug,
		BillingEmail: req.BillingEmail,
		Settings:     settingsJSON,
		Status:       string(domain.OrganizationStatusActive),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return s.dbToDomain(dbOrg), nil
}

// UpdateOrganizationRequest contains the parameters for updating an organization.
type UpdateOrganizationRequest struct {
	ID           uuid.UUID
	Name         *string
	Slug         *string
	BillingEmail *string
	Settings     *domain.OrganizationSettings
}

// UpdateOrganization updates an organization.
func (s *Service) UpdateOrganization(ctx context.Context, req UpdateOrganizationRequest) (*domain.Organization, error) {
	// If slug is being updated, validate and check uniqueness
	if req.Slug != nil {
		if !isValidSlug(*req.Slug) {
			return nil, fmt.Errorf("%w: invalid slug format", domain.ErrValidation)
		}

		exists, err := s.queries.CheckOrganizationSlugExists(ctx, db.CheckOrganizationSlugExistsParams{
			Slug:      *req.Slug,
			ExcludeID: pgtype.UUID{Bytes: req.ID, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to check slug existence: %w", err)
		}
		if exists {
			return nil, domain.ErrOrganizationSlugTaken
		}
	}

	// Prepare settings
	var settingsJSON []byte
	if req.Settings != nil {
		var err error
		settingsJSON, err = json.Marshal(req.Settings)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal settings: %w", err)
		}
	}

	// Update organization
	dbOrg, err := s.queries.UpdateOrganization(ctx, db.UpdateOrganizationParams{
		ID:           req.ID,
		Name:         req.Name,
		Slug:         req.Slug,
		BillingEmail: req.BillingEmail,
		Settings:     settingsJSON,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return s.dbToDomain(dbOrg), nil
}

// UpdateOrganizationStatus updates the status of an organization.
func (s *Service) UpdateOrganizationStatus(ctx context.Context, id uuid.UUID, status domain.OrganizationStatus) (*domain.Organization, error) {
	if !status.IsValid() {
		return nil, fmt.Errorf("%w: invalid status", domain.ErrValidation)
	}

	dbOrg, err := s.queries.UpdateOrganizationStatus(ctx, db.UpdateOrganizationStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to update organization status: %w", err)
	}

	return s.dbToDomain(dbOrg), nil
}

// SuspendOrganization suspends an organization.
func (s *Service) SuspendOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.UpdateOrganizationStatus(ctx, id, domain.OrganizationStatusSuspended)
}

// ActivateOrganization activates an organization.
func (s *Service) ActivateOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.UpdateOrganizationStatus(ctx, id, domain.OrganizationStatusActive)
}

// CloseOrganization closes an organization.
func (s *Service) CloseOrganization(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.UpdateOrganizationStatus(ctx, id, domain.OrganizationStatusClosed)
}

// DeleteOrganization soft deletes an organization.
func (s *Service) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.SoftDeleteOrganization(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrOrganizationNotFound
		}
		return fmt.Errorf("failed to delete organization: %w", err)
	}

	return nil
}

// ListOrganizationsRequest contains the parameters for listing organizations.
type ListOrganizationsRequest struct {
	Status *domain.OrganizationStatus
	Offset int
	Limit  int
}

// ListOrganizations lists organizations with optional filtering.
func (s *Service) ListOrganizations(ctx context.Context, req ListOrganizationsRequest) ([]*domain.Organization, int64, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	var status *string
	if req.Status != nil {
		s := string(*req.Status)
		status = &s
	}

	// Get count
	count, err := s.queries.CountOrganizations(ctx, status)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count organizations: %w", err)
	}

	// Get organizations
	dbOrgs, err := s.queries.ListOrganizations(ctx, db.ListOrganizationsParams{
		Status:      status,
		OffsetCount: int32(req.Offset),
		LimitCount:  int32(req.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list organizations: %w", err)
	}

	orgs := make([]*domain.Organization, len(dbOrgs))
	for i, dbOrg := range dbOrgs {
		orgs[i] = s.dbToDomain(dbOrg)
	}

	return orgs, count, nil
}

// Member management

// GetMember retrieves a member by ID.
func (s *Service) GetMember(ctx context.Context, memberID uuid.UUID) (*domain.Member, error) {
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
func (s *Service) GetMemberWithRole(ctx context.Context, memberID uuid.UUID) (*domain.Member, *domain.Role, error) {
	row, err := s.queries.GetMemberWithRole(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.ErrMemberNotFound
		}
		return nil, nil, fmt.Errorf("failed to get member with role: %w", err)
	}

	member := s.dbMemberWithRoleToDomain(row)
	role := s.dbRoleToDomain(row)

	return member, role, nil
}

// ListMembersRequest contains the parameters for listing members.
type ListMembersRequest struct {
	OrganizationID uuid.UUID
	Status         *domain.MemberStatus
	Offset         int
	Limit          int
}

// ListMembers lists members of an organization.
func (s *Service) ListMembers(ctx context.Context, req ListMembersRequest) ([]*domain.Member, int64, error) {
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	var status *string
	if req.Status != nil {
		st := string(*req.Status)
		status = &st
	}

	// Get count
	count, err := s.queries.CountMembersByOrganization(ctx, db.CountMembersByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         status,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count members: %w", err)
	}

	// Get members
	dbMembers, err := s.queries.ListMembersByOrganization(ctx, db.ListMembersByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         status,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list members: %w", err)
	}

	members := make([]*domain.Member, len(dbMembers))
	for i, dbMember := range dbMembers {
		members[i] = s.dbMemberToDomain(dbMember)
	}

	return members, count, nil
}

// CreateMemberRequest contains the parameters for creating a member.
type CreateMemberRequest struct {
	OrganizationID uuid.UUID
	Email          string
	Name           string
	RoleID         uuid.UUID
	Password       *string // Optional - if provided, member is activated immediately
}

// CreateMember creates a new member (invited status by default).
func (s *Service) CreateMember(ctx context.Context, req CreateMemberRequest) (*domain.Member, error) {
	// Check if email is already taken in this organization
	exists, err := s.queries.CheckMemberEmailExists(ctx, db.CheckMemberEmailExistsParams{
		OrganizationID: req.OrganizationID,
		Email:          req.Email,
		ExcludeID:      pgtype.UUID{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		return nil, domain.ErrMemberEmailTaken
	}

	// Determine status and password hash
	status := string(domain.MemberStatusInvited)
	var passwordHash *string

	if req.Password != nil {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		passwordHash = &hash
		status = string(domain.MemberStatusActive)
	}

	// Create member
	dbMember, err := s.queries.CreateMember(ctx, db.CreateMemberParams{
		OrganizationID: req.OrganizationID,
		Email:          req.Email,
		Name:           req.Name,
		PasswordHash:   passwordHash,
		RoleID:         req.RoleID,
		Status:         status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// UpdateMemberRequest contains the parameters for updating a member.
type UpdateMemberRequest struct {
	MemberID uuid.UUID
	Name     *string
	Email    *string
	RoleID   *uuid.UUID
}

// UpdateMember updates a member.
func (s *Service) UpdateMember(ctx context.Context, req UpdateMemberRequest) (*domain.Member, error) {
	// If email is being updated, check uniqueness
	if req.Email != nil {
		// Get current member to get organization ID
		currentMember, err := s.queries.GetMemberByID(ctx, req.MemberID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, domain.ErrMemberNotFound
			}
			return nil, fmt.Errorf("failed to get member: %w", err)
		}

		exists, err := s.queries.CheckMemberEmailExists(ctx, db.CheckMemberEmailExistsParams{
			OrganizationID: currentMember.OrganizationID,
			Email:          *req.Email,
			ExcludeID:      pgtype.UUID{Bytes: req.MemberID, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to check email existence: %w", err)
		}
		if exists {
			return nil, domain.ErrMemberEmailTaken
		}
	}

	// Prepare role ID
	var roleID pgtype.UUID
	if req.RoleID != nil {
		roleID = pgtype.UUID{Bytes: *req.RoleID, Valid: true}
	}

	// Update member
	dbMember, err := s.queries.UpdateMember(ctx, db.UpdateMemberParams{
		ID:     req.MemberID,
		Name:   req.Name,
		Email:  req.Email,
		RoleID: roleID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to update member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// ActivateMember activates an invited member with a password.
func (s *Service) ActivateMember(ctx context.Context, memberID uuid.UUID, password string) (*domain.Member, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	dbMember, err := s.queries.ActivateMember(ctx, db.ActivateMemberParams{
		ID:           memberID,
		PasswordHash: &hash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to activate member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// SuspendMember suspends a member.
func (s *Service) SuspendMember(ctx context.Context, memberID uuid.UUID) (*domain.Member, error) {
	dbMember, err := s.queries.SuspendMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to suspend member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// ReactivateMember reactivates a suspended member.
func (s *Service) ReactivateMember(ctx context.Context, memberID uuid.UUID) (*domain.Member, error) {
	dbMember, err := s.queries.ReactivateMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, fmt.Errorf("failed to reactivate member: %w", err)
	}

	return s.dbMemberToDomain(dbMember), nil
}

// RemoveMember removes a member from the organization.
func (s *Service) RemoveMember(ctx context.Context, memberID uuid.UUID) error {
	_, err := s.queries.RemoveMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrMemberNotFound
		}
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return nil
}

// UpdateMemberPassword updates a member's password.
func (s *Service) UpdateMemberPassword(ctx context.Context, memberID uuid.UUID, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = s.queries.UpdateMemberPassword(ctx, db.UpdateMemberPasswordParams{
		ID:           memberID,
		PasswordHash: &hash,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrMemberNotFound
		}
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// Helper functions

func isValidSlug(slug string) bool {
	if len(slug) < 3 || len(slug) > 50 {
		return false
	}
	return slugRegex.MatchString(slug)
}

func (s *Service) dbToDomain(dbOrg *db.Organization) *domain.Organization {
	org := &domain.Organization{
		ID:           dbOrg.ID,
		Name:         dbOrg.Name,
		Slug:         dbOrg.Slug,
		BillingEmail: dbOrg.BillingEmail,
		Status:       domain.OrganizationStatus(dbOrg.Status),
		CreatedAt:    dbOrg.CreatedAt.Time,
		UpdatedAt:    dbOrg.UpdatedAt.Time,
	}

	// Parse settings
	if len(dbOrg.Settings) > 0 {
		_ = json.Unmarshal(dbOrg.Settings, &org.Settings)
	}

	// Handle deleted_at
	if dbOrg.DeletedAt.Valid {
		t := dbOrg.DeletedAt.Time
		org.DeletedAt = &t
	}

	return org
}

func (s *Service) dbMemberToDomain(dbMember *db.Member) *domain.Member {
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

func (s *Service) dbMemberWithRoleToDomain(row *db.GetMemberWithRoleRow) *domain.Member {
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

func (s *Service) dbRoleToDomain(row *db.GetMemberWithRoleRow) *domain.Role {
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

// GenerateSlug generates a URL-safe slug from a name.
func GenerateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()

	// Remove consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
		// Remove trailing hyphen if we cut in the middle of a word
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}
