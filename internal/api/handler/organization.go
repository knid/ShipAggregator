// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/service/organization"
)

// OrganizationHandler handles organization-related HTTP requests.
type OrganizationHandler struct {
	orgService *organization.Service
	logger     *observability.Logger
}

// NewOrganizationHandler creates a new organization handler.
func NewOrganizationHandler(orgService *organization.Service, logger *observability.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
		logger:     logger,
	}
}

// GetCurrentOrganization handles GET /orgs/current
// Returns the current organization based on the authenticated context.
func (h *OrganizationHandler) GetCurrentOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	org, err := h.orgService.GetOrganization(ctx, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrOrganizationNotFound) {
			WriteError(w, r, NewNotFoundError("Organization"))
			return
		}
		h.logger.Error("Failed to get organization", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get organization"))
		return
	}

	writeJSON(w, http.StatusOK, organizationToResponse(org))
}

// UpdateOrganizationRequest represents the request body for updating an organization.
type UpdateOrganizationRequest struct {
	Name         *string                      `json:"name,omitempty"`
	BillingEmail *string                      `json:"billing_email,omitempty"`
	Settings     *domain.OrganizationSettings `json:"settings,omitempty"`
}

// UpdateCurrentOrganization handles PATCH /orgs/current
// Updates the current organization.
func (h *OrganizationHandler) UpdateCurrentOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req UpdateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate name if provided
	if req.Name != nil && len(*req.Name) < 2 {
		WriteError(w, r, NewValidationError("Name must be at least 2 characters", nil))
		return
	}

	// Validate billing email if provided
	if req.BillingEmail != nil && !isValidEmail(*req.BillingEmail) {
		WriteError(w, r, NewValidationError("Invalid billing email", nil))
		return
	}

	org, err := h.orgService.UpdateOrganization(ctx, organization.UpdateOrganizationRequest{
		ID:           orgID,
		Name:         req.Name,
		BillingEmail: req.BillingEmail,
		Settings:     req.Settings,
	})
	if err != nil {
		if errors.Is(err, domain.ErrOrganizationNotFound) {
			WriteError(w, r, NewNotFoundError("Organization"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			WriteError(w, r, NewValidationError(err.Error(), nil))
			return
		}
		h.logger.Error("Failed to update organization", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to update organization"))
		return
	}

	writeJSON(w, http.StatusOK, organizationToResponse(org))
}

// ListMembers handles GET /orgs/current/members
// Lists members of the current organization.
func (h *OrganizationHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse query parameters
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	var status *domain.MemberStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := domain.MemberStatus(statusStr)
		if s.IsValid() {
			status = &s
		}
	}

	members, total, err := h.orgService.ListMembers(ctx, organization.ListMembersRequest{
		OrganizationID: orgID,
		Status:         status,
		Offset:         offset,
		Limit:          limit,
	})
	if err != nil {
		h.logger.Error("Failed to list members", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list members"))
		return
	}

	response := make([]MemberResponse, len(members))
	for i, m := range members {
		response[i] = memberToResponse(m)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: offset,
		Limit:  limit,
	})
}

// CreateMemberRequest represents the request body for creating a member.
type CreateMemberRequest struct {
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	RoleID   uuid.UUID `json:"role_id"`
	Password *string   `json:"password,omitempty"` // Optional - if provided, member is activated immediately
}

// CreateMember handles POST /orgs/current/members
// Creates a new member in the current organization.
func (h *OrganizationHandler) CreateMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	var req CreateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate required fields
	var fieldErrors []ErrorDetail
	if req.Email == "" {
		fieldErrors = append(fieldErrors, NewFieldError("email", "REQUIRED", "Email is required"))
	} else if !isValidEmail(req.Email) {
		fieldErrors = append(fieldErrors, NewFieldError("email", "INVALID", "Invalid email format"))
	}
	if req.Name == "" {
		fieldErrors = append(fieldErrors, NewFieldError("name", "REQUIRED", "Name is required"))
	}
	if req.RoleID == uuid.Nil {
		fieldErrors = append(fieldErrors, NewFieldError("role_id", "REQUIRED", "Role ID is required"))
	}
	if req.Password != nil && len(*req.Password) < 8 {
		fieldErrors = append(fieldErrors, NewFieldError("password", "TOO_SHORT", "Password must be at least 8 characters"))
	}

	if len(fieldErrors) > 0 {
		WriteError(w, r, NewValidationError("Validation failed", fieldErrors))
		return
	}

	member, err := h.orgService.CreateMember(ctx, organization.CreateMemberRequest{
		OrganizationID: orgID,
		Email:          req.Email,
		Name:           req.Name,
		RoleID:         req.RoleID,
		Password:       req.Password,
	})
	if err != nil {
		if errors.Is(err, domain.ErrMemberEmailTaken) {
			WriteError(w, r, NewConflictError("Email is already in use"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			WriteError(w, r, NewValidationError(err.Error(), nil))
			return
		}
		h.logger.Error("Failed to create member", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to create member"))
		return
	}

	writeJSON(w, http.StatusCreated, memberToResponse(member))
}

// GetMember handles GET /orgs/current/members/{id}
// Returns a specific member.
func (h *OrganizationHandler) GetMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get member ID from URL
	memberIDStr := chi.URLParam(r, "id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid member ID"))
		return
	}

	member, err := h.orgService.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to get member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to get member"))
		return
	}

	// Verify member belongs to the organization
	if member.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Member"))
		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(member))
}

// UpdateMemberRequest represents the request body for updating a member.
type UpdateMemberRequest struct {
	Name   *string    `json:"name,omitempty"`
	Email  *string    `json:"email,omitempty"`
	RoleID *uuid.UUID `json:"role_id,omitempty"`
}

// UpdateMember handles PATCH /orgs/current/members/{id}
// Updates a specific member.
func (h *OrganizationHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get member ID from URL
	memberIDStr := chi.URLParam(r, "id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid member ID"))
		return
	}

	// Verify member belongs to the organization
	existingMember, err := h.orgService.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to get member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to get member"))
		return
	}
	if existingMember.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Member"))
		return
	}

	var req UpdateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate email if provided
	if req.Email != nil && !isValidEmail(*req.Email) {
		WriteError(w, r, NewValidationError("Invalid email format", nil))
		return
	}

	member, err := h.orgService.UpdateMember(ctx, organization.UpdateMemberRequest{
		MemberID: memberID,
		Name:     req.Name,
		Email:    req.Email,
		RoleID:   req.RoleID,
	})
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		if errors.Is(err, domain.ErrMemberEmailTaken) {
			WriteError(w, r, NewConflictError("Email is already in use"))
			return
		}
		if errors.Is(err, domain.ErrValidation) {
			WriteError(w, r, NewValidationError(err.Error(), nil))
			return
		}
		h.logger.Error("Failed to update member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to update member"))
		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(member))
}

// SuspendMember handles POST /orgs/current/members/{id}/suspend
// Suspends a member.
func (h *OrganizationHandler) SuspendMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get member ID from URL
	memberIDStr := chi.URLParam(r, "id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid member ID"))
		return
	}

	// Verify member belongs to the organization
	existingMember, err := h.orgService.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to get member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to get member"))
		return
	}
	if existingMember.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Member"))
		return
	}

	member, err := h.orgService.SuspendMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to suspend member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to suspend member"))
		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(member))
}

// ReactivateMember handles POST /orgs/current/members/{id}/reactivate
// Reactivates a suspended member.
func (h *OrganizationHandler) ReactivateMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get member ID from URL
	memberIDStr := chi.URLParam(r, "id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid member ID"))
		return
	}

	// Verify member belongs to the organization
	existingMember, err := h.orgService.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to get member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to get member"))
		return
	}
	if existingMember.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Member"))
		return
	}

	member, err := h.orgService.ReactivateMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to reactivate member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to reactivate member"))
		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(member))
}

// RemoveMember handles DELETE /orgs/current/members/{id}
// Removes a member from the organization.
func (h *OrganizationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get member ID from URL
	memberIDStr := chi.URLParam(r, "id")
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid member ID"))
		return
	}

	// Verify member belongs to the organization
	existingMember, err := h.orgService.GetMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to get member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to get member"))
		return
	}
	if existingMember.OrganizationID != orgID {
		WriteError(w, r, NewNotFoundError("Member"))
		return
	}

	// Prevent self-removal
	actorID := middleware.GetActorID(ctx)
	if actorID == existingMember.ID {
		WriteError(w, r, NewBadRequestError("Cannot remove yourself"))
		return
	}

	err = h.orgService.RemoveMember(ctx, memberID)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			WriteError(w, r, NewNotFoundError("Member"))
			return
		}
		h.logger.Error("Failed to remove member", "error", err, "member_id", memberID)
		WriteError(w, r, NewInternalError("Failed to remove member"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Response types

// OrganizationResponse represents an organization in API responses.
type OrganizationResponse struct {
	ID           uuid.UUID                   `json:"id"`
	Name         string                      `json:"name"`
	Slug         string                      `json:"slug"`
	BillingEmail string                      `json:"billing_email"`
	Settings     domain.OrganizationSettings `json:"settings"`
	Status       domain.OrganizationStatus   `json:"status"`
	CreatedAt    string                      `json:"created_at"`
	UpdatedAt    string                      `json:"updated_at"`
}

// MemberResponse represents a member in API responses.
type MemberResponse struct {
	ID             uuid.UUID           `json:"id"`
	OrganizationID uuid.UUID           `json:"organization_id"`
	Email          string              `json:"email"`
	Name           string              `json:"name"`
	RoleID         uuid.UUID           `json:"role_id"`
	Status         domain.MemberStatus `json:"status"`
	InvitedAt      string              `json:"invited_at"`
	JoinedAt       *string             `json:"joined_at,omitempty"`
	LastLoginAt    *string             `json:"last_login_at,omitempty"`
	CreatedAt      string              `json:"created_at"`
	UpdatedAt      string              `json:"updated_at"`
}

// PaginatedResponse represents a paginated list response.
type PaginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int64       `json:"total"`
	Offset int         `json:"offset"`
	Limit  int         `json:"limit"`
}

// Helper functions

func organizationToResponse(org *domain.Organization) OrganizationResponse {
	return OrganizationResponse{
		ID:           org.ID,
		Name:         org.Name,
		Slug:         org.Slug,
		BillingEmail: org.BillingEmail,
		Settings:     org.Settings,
		Status:       org.Status,
		CreatedAt:    org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func memberToResponse(m *domain.Member) MemberResponse {
	resp := MemberResponse{
		ID:             m.ID,
		OrganizationID: m.OrganizationID,
		Email:          m.Email,
		Name:           m.Name,
		RoleID:         m.RoleID,
		Status:         m.Status,
		InvitedAt:      m.InvitedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:      m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if m.JoinedAt != nil {
		t := m.JoinedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.JoinedAt = &t
	}

	if m.LastLoginAt != nil {
		t := m.LastLoginAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastLoginAt = &t
	}

	return resp
}

func isValidEmail(email string) bool {
	// Simple email validation
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			if atIndex != -1 {
				return false // Multiple @
			}
			atIndex = i
		}
	}
	if atIndex < 1 || atIndex >= len(email)-1 {
		return false
	}
	return true
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
