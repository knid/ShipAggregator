// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// RoleHandler handles role-related HTTP requests.
type RoleHandler struct {
	queries *db.Queries
	logger  *observability.Logger
}

// NewRoleHandler creates a new role handler.
func NewRoleHandler(queries *db.Queries, logger *observability.Logger) *RoleHandler {
	return &RoleHandler{
		queries: queries,
		logger:  logger,
	}
}

// ListRoles handles GET /roles
// Returns all roles for the organization.
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Convert uuid.UUID to pgtype.UUID
	var pgOrgID pgtype.UUID
	pgOrgID.Bytes = orgID
	pgOrgID.Valid = true

	// Get roles (includes both org-specific and system roles)
	roles, err := h.queries.ListRolesByOrganization(ctx, pgOrgID)
	if err != nil {
		h.logger.Error("Failed to list roles", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to list roles"))
		return
	}

	response := make([]RoleResponse, len(roles))
	for i, role := range roles {
		response[i] = roleToResponse(role)
	}

	writeJSON(w, http.StatusOK, response)
}

// GetRole handles GET /roles/{id}
// Returns a specific role.
func (h *RoleHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get role ID from URL
	roleIDStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid role ID"))
		return
	}

	// Get role
	role, err := h.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Role"))
			return
		}
		h.logger.Error("Failed to get role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to get role"))
		return
	}

	// Verify role belongs to organization or is a system role
	if role.OrganizationID.Valid && role.OrganizationID.Bytes != orgID {
		WriteError(w, r, NewNotFoundError("Role"))
		return
	}

	writeJSON(w, http.StatusOK, roleToResponse(role))
}

// CreateRole handles POST /roles
// Creates a new role.
func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse request body
	var req CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if req.Name == "" {
		WriteError(w, r, NewBadRequestError("name is required"))
		return
	}
	if len(req.Permissions) == 0 {
		WriteError(w, r, NewBadRequestError("permissions is required"))
		return
	}

	// Convert uuid.UUID to pgtype.UUID
	var pgOrgID pgtype.UUID
	pgOrgID.Bytes = orgID
	pgOrgID.Valid = true

	// Check if role name already exists
	exists, err := h.queries.CheckRoleNameExists(ctx, db.CheckRoleNameExistsParams{
		Name:           req.Name,
		OrganizationID: pgOrgID,
	})
	if err != nil {
		h.logger.Error("Failed to check role name", "error", err, "name", req.Name)
		WriteError(w, r, NewInternalError("Failed to create role"))
		return
	}
	if exists {
		WriteError(w, r, NewBadRequestError("Role with this name already exists"))
		return
	}

	// Convert permissions to JSON
	permissionsJSON, err := json.Marshal(req.Permissions)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid permissions format"))
		return
	}

	// Create role
	role, err := h.queries.CreateRole(ctx, db.CreateRoleParams{
		OrganizationID: pgOrgID,
		Name:           req.Name,
		Description:    req.Description,
		Permissions:    permissionsJSON,
		IsSystem:       false, // User-created roles are not system roles
	})
	if err != nil {
		h.logger.Error("Failed to create role", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to create role"))
		return
	}

	writeJSON(w, http.StatusCreated, roleToResponse(role))
}

// UpdateRole handles PUT /roles/{id}
// Updates an existing role.
func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get role ID from URL
	roleIDStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid role ID"))
		return
	}

	// Check if role exists and belongs to organization
	existingRole, err := h.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Role"))
			return
		}
		h.logger.Error("Failed to get role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to get role"))
		return
	}

	// Verify role belongs to organization
	if !existingRole.OrganizationID.Valid || existingRole.OrganizationID.Bytes != orgID {
		WriteError(w, r, NewNotFoundError("Role"))
		return
	}

	// Check if role is a system role
	if existingRole.IsSystem {
		WriteError(w, r, NewBadRequestError("Cannot modify system roles"))
		return
	}

	// Parse request body
	var req UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Build update params
	var name *string
	if req.Name != "" {
		name = &req.Name
	}

	var permissionsJSON []byte
	if len(req.Permissions) > 0 {
		permissionsJSON, err = json.Marshal(req.Permissions)
		if err != nil {
			WriteError(w, r, NewBadRequestError("Invalid permissions format"))
			return
		}
	}

	// Update role
	role, err := h.queries.UpdateRole(ctx, db.UpdateRoleParams{
		ID:          roleID,
		Name:        name,
		Description: req.Description,
		Permissions: permissionsJSON,
	})
	if err != nil {
		h.logger.Error("Failed to update role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to update role"))
		return
	}

	writeJSON(w, http.StatusOK, roleToResponse(role))
}

// DeleteRole handles DELETE /roles/{id}
// Deletes a role.
func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Get role ID from URL
	roleIDStr := chi.URLParam(r, "id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid role ID"))
		return
	}

	// Check if role exists and belongs to organization
	existingRole, err := h.queries.GetRoleByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(w, r, NewNotFoundError("Role"))
			return
		}
		h.logger.Error("Failed to get role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to get role"))
		return
	}

	// Verify role belongs to organization
	if !existingRole.OrganizationID.Valid || existingRole.OrganizationID.Bytes != orgID {
		WriteError(w, r, NewNotFoundError("Role"))
		return
	}

	// Check if role is a system role
	if existingRole.IsSystem {
		WriteError(w, r, NewBadRequestError("Cannot delete system roles"))
		return
	}

	// Check if role is in use by any members
	memberCount, err := h.queries.CountMembersWithRole(ctx, roleID)
	if err != nil {
		h.logger.Error("Failed to count members with role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to check role usage"))
		return
	}
	if memberCount > 0 {
		WriteError(w, r, NewBadRequestError("Cannot delete role that is assigned to members"))
		return
	}

	// Delete role
	err = h.queries.DeleteRole(ctx, roleID)
	if err != nil {
		h.logger.Error("Failed to delete role", "error", err, "role_id", roleID)
		WriteError(w, r, NewInternalError("Failed to delete role"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Request types

// CreateRoleRequest represents a request to create a role.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions"`
}

// UpdateRoleRequest represents a request to update a role.
type UpdateRoleRequest struct {
	Name        string   `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// Response types

// RoleResponse represents a role in API responses.
type RoleResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID *string   `json:"organization_id,omitempty"`
	Name           string    `json:"name"`
	Description    *string   `json:"description,omitempty"`
	Permissions    []string  `json:"permissions"`
	IsSystem       bool      `json:"is_system"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

// Helper functions

func roleToResponse(role *db.Role) RoleResponse {
	resp := RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedAt:   role.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Time.Format(time.RFC3339),
	}

	// Convert organization ID
	if role.OrganizationID.Valid {
		orgID := uuid.UUID(role.OrganizationID.Bytes)
		orgIDStr := orgID.String()
		resp.OrganizationID = &orgIDStr
	}

	// Parse permissions from JSON
	if len(role.Permissions) > 0 {
		var permissions []string
		if err := json.Unmarshal(role.Permissions, &permissions); err == nil {
			resp.Permissions = permissions
		} else {
			resp.Permissions = []string{}
		}
	} else {
		resp.Permissions = []string{}
	}

	return resp
}
