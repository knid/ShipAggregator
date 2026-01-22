package domain

import (
	"time"

	"github.com/google/uuid"
)

// MemberStatus represents the status of a member within an organization.
type MemberStatus string

const (
	// MemberStatusInvited indicates the member has been invited but hasn't joined yet.
	MemberStatusInvited MemberStatus = "invited"
	// MemberStatusActive indicates the member is active and can access the platform.
	MemberStatusActive MemberStatus = "active"
	// MemberStatusSuspended indicates the member is temporarily suspended.
	MemberStatusSuspended MemberStatus = "suspended"
	// MemberStatusRemoved indicates the member has been removed from the organization.
	MemberStatusRemoved MemberStatus = "removed"
)

// IsValid checks if the member status is a valid value.
func (s MemberStatus) IsValid() bool {
	switch s {
	case MemberStatusInvited, MemberStatusActive, MemberStatusSuspended, MemberStatusRemoved:
		return true
	default:
		return false
	}
}

// String returns the string representation of the member status.
func (s MemberStatus) String() string {
	return string(s)
}

// Member represents a user belonging to an organization with role-based access.
type Member struct {
	// ID is the unique identifier for the member.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the ID of the parent organization.
	OrganizationID uuid.UUID `json:"organization_id"`
	// Email is the member's email address (login identifier).
	Email string `json:"email"`
	// Name is the member's display name.
	Name string `json:"name"`
	// PasswordHash is the bcrypt hash of the password (nil for invited members).
	PasswordHash *string `json:"-"`
	// RoleID is the ID of the assigned role.
	RoleID uuid.UUID `json:"role_id"`
	// Status is the current member status.
	Status MemberStatus `json:"status"`
	// InvitedAt is the timestamp when the member was invited.
	InvitedAt time.Time `json:"invited_at"`
	// JoinedAt is the timestamp when the member activated their account.
	JoinedAt *time.Time `json:"joined_at,omitempty"`
	// LastLoginAt is the timestamp of the last successful login.
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	// CreatedAt is the timestamp when the member record was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the member record was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewMember creates a new Member with invited status.
func NewMember(organizationID uuid.UUID, email, name string, roleID uuid.UUID) *Member {
	now := time.Now().UTC()
	return &Member{
		ID:             uuid.New(),
		OrganizationID: organizationID,
		Email:          email,
		Name:           name,
		RoleID:         roleID,
		Status:         MemberStatusInvited,
		InvitedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// IsActive returns true if the member is in active status.
func (m *Member) IsActive() bool {
	return m.Status == MemberStatusActive
}

// IsInvited returns true if the member is in invited status.
func (m *Member) IsInvited() bool {
	return m.Status == MemberStatusInvited
}

// IsSuspended returns true if the member is suspended.
func (m *Member) IsSuspended() bool {
	return m.Status == MemberStatusSuspended
}

// IsRemoved returns true if the member has been removed.
func (m *Member) IsRemoved() bool {
	return m.Status == MemberStatusRemoved
}

// CanLogin returns true if the member can log in.
func (m *Member) CanLogin() bool {
	return m.Status == MemberStatusActive && m.PasswordHash != nil
}

// Activate activates the member account with a password hash.
func (m *Member) Activate(passwordHash string) {
	now := time.Now().UTC()
	m.PasswordHash = &passwordHash
	m.Status = MemberStatusActive
	m.JoinedAt = &now
	m.UpdatedAt = now
}

// Suspend suspends the member account.
func (m *Member) Suspend() {
	m.Status = MemberStatusSuspended
	m.UpdatedAt = time.Now().UTC()
}

// Unsuspend reactivates a suspended member account.
func (m *Member) Unsuspend() {
	m.Status = MemberStatusActive
	m.UpdatedAt = time.Now().UTC()
}

// Remove removes the member from the organization.
func (m *Member) Remove() {
	m.Status = MemberStatusRemoved
	m.UpdatedAt = time.Now().UTC()
}

// UpdateName updates the member's display name.
func (m *Member) UpdateName(name string) {
	m.Name = name
	m.UpdatedAt = time.Now().UTC()
}

// UpdateEmail updates the member's email address.
func (m *Member) UpdateEmail(email string) {
	m.Email = email
	m.UpdatedAt = time.Now().UTC()
}

// UpdateRole updates the member's role.
func (m *Member) UpdateRole(roleID uuid.UUID) {
	m.RoleID = roleID
	m.UpdatedAt = time.Now().UTC()
}

// UpdatePassword updates the member's password hash.
func (m *Member) UpdatePassword(passwordHash string) {
	m.PasswordHash = &passwordHash
	m.UpdatedAt = time.Now().UTC()
}

// RecordLogin records a successful login.
func (m *Member) RecordLogin() {
	now := time.Now().UTC()
	m.LastLoginAt = &now
	m.UpdatedAt = now
}

// Role represents a permission set assignable to members.
type Role struct {
	// ID is the unique identifier for the role.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the ID of the parent organization (nil for system roles).
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	// Name is the role name.
	Name string `json:"name"`
	// Description is the role description.
	Description *string `json:"description,omitempty"`
	// Permissions is the list of permission strings.
	Permissions []string `json:"permissions"`
	// IsSystem indicates if this is a system role (non-editable).
	IsSystem bool `json:"is_system"`
	// CreatedAt is the timestamp when the role was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the role was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewRole creates a new custom role for an organization.
func NewRole(organizationID uuid.UUID, name string, permissions []string) *Role {
	now := time.Now().UTC()
	return &Role{
		ID:             uuid.New(),
		OrganizationID: &organizationID,
		Name:           name,
		Permissions:    permissions,
		IsSystem:       false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewSystemRole creates a new system role (no organization).
func NewSystemRole(name string, permissions []string) *Role {
	now := time.Now().UTC()
	return &Role{
		ID:          uuid.New(),
		Name:        name,
		Permissions: permissions,
		IsSystem:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// HasPermission checks if the role has a specific permission.
func (r *Role) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission || p == "*" {
			return true
		}
	}
	return false
}

// UpdateName updates the role name.
func (r *Role) UpdateName(name string) {
	r.Name = name
	r.UpdatedAt = time.Now().UTC()
}

// UpdateDescription updates the role description.
func (r *Role) UpdateDescription(description string) {
	r.Description = &description
	r.UpdatedAt = time.Now().UTC()
}

// UpdatePermissions updates the role permissions.
func (r *Role) UpdatePermissions(permissions []string) {
	r.Permissions = permissions
	r.UpdatedAt = time.Now().UTC()
}

// Permission constants for the platform.
const (
	// Organization permissions
	PermissionOrgRead   = "org:read"
	PermissionOrgUpdate = "org:update"
	PermissionOrgDelete = "org:delete"

	// Member permissions
	PermissionMemberRead   = "member:read"
	PermissionMemberCreate = "member:create"
	PermissionMemberUpdate = "member:update"
	PermissionMemberDelete = "member:delete"

	// Role permissions
	PermissionRoleRead   = "role:read"
	PermissionRoleCreate = "role:create"
	PermissionRoleUpdate = "role:update"
	PermissionRoleDelete = "role:delete"

	// API Key permissions
	PermissionAPIKeyRead   = "apikey:read"
	PermissionAPIKeyCreate = "apikey:create"
	PermissionAPIKeyUpdate = "apikey:update"
	PermissionAPIKeyDelete = "apikey:delete"
	PermissionAPIKeyRotate = "apikey:rotate"

	// Address permissions
	PermissionAddressRead   = "address:read"
	PermissionAddressCreate = "address:create"
	PermissionAddressUpdate = "address:update"
	PermissionAddressDelete = "address:delete"

	// Provider permissions
	PermissionProviderRead = "provider:read"

	// Provider Account permissions
	PermissionProviderAccountRead   = "provider_account:read"
	PermissionProviderAccountCreate = "provider_account:create"
	PermissionProviderAccountUpdate = "provider_account:update"
	PermissionProviderAccountDelete = "provider_account:delete"
	PermissionProviderAccountVerify = "provider_account:verify"

	// Quote permissions
	PermissionQuoteRead   = "quote:read"
	PermissionQuoteCreate = "quote:create"

	// Shipment permissions
	PermissionShipmentRead   = "shipment:read"
	PermissionShipmentCreate = "shipment:create"
	PermissionShipmentUpdate = "shipment:update"
	PermissionShipmentCancel = "shipment:cancel"
	PermissionShipmentLabel  = "shipment:label"
	PermissionShipmentPickup = "shipment:pickup"
	PermissionShipmentReturn = "shipment:return"

	// Tracking permissions
	PermissionTrackingRead = "tracking:read"

	// Webhook permissions
	PermissionWebhookRead   = "webhook:read"
	PermissionWebhookCreate = "webhook:create"
	PermissionWebhookUpdate = "webhook:update"
	PermissionWebhookDelete = "webhook:delete"
	PermissionWebhookTest   = "webhook:test"
	PermissionWebhookReplay = "webhook:replay"

	// Billing permissions
	PermissionBillingRead   = "billing:read"
	PermissionBillingTopup  = "billing:topup"
	PermissionBillingRefund = "billing:refund"

	// Admin permissions
	PermissionAdminAuditLog        = "admin:audit_log"
	PermissionAdminProviderLog     = "admin:provider_log"
	PermissionAdminProviderHealth  = "admin:provider_health"
	PermissionAdminWebhookDLQ      = "admin:webhook_dlq"
	PermissionAdminRebuildTracking = "admin:rebuild_tracking"

	// Wildcard permission (full access)
	PermissionAll = "*"
)

// SystemRoleOwner returns the permissions for the owner role.
func SystemRoleOwner() []string {
	return []string{PermissionAll}
}

// SystemRoleAdmin returns the permissions for the admin role.
func SystemRoleAdmin() []string {
	return []string{
		PermissionOrgRead, PermissionOrgUpdate,
		PermissionMemberRead, PermissionMemberCreate, PermissionMemberUpdate, PermissionMemberDelete,
		PermissionRoleRead, PermissionRoleCreate, PermissionRoleUpdate, PermissionRoleDelete,
		PermissionAPIKeyRead, PermissionAPIKeyCreate, PermissionAPIKeyUpdate, PermissionAPIKeyDelete, PermissionAPIKeyRotate,
		PermissionAddressRead, PermissionAddressCreate, PermissionAddressUpdate, PermissionAddressDelete,
		PermissionProviderRead,
		PermissionProviderAccountRead, PermissionProviderAccountCreate, PermissionProviderAccountUpdate, PermissionProviderAccountDelete, PermissionProviderAccountVerify,
		PermissionQuoteRead, PermissionQuoteCreate,
		PermissionShipmentRead, PermissionShipmentCreate, PermissionShipmentUpdate, PermissionShipmentCancel, PermissionShipmentLabel, PermissionShipmentPickup, PermissionShipmentReturn,
		PermissionTrackingRead,
		PermissionWebhookRead, PermissionWebhookCreate, PermissionWebhookUpdate, PermissionWebhookDelete, PermissionWebhookTest, PermissionWebhookReplay,
		PermissionBillingRead, PermissionBillingTopup,
	}
}

// SystemRoleOperator returns the permissions for the operator role.
func SystemRoleOperator() []string {
	return []string{
		PermissionOrgRead,
		PermissionMemberRead,
		PermissionAddressRead, PermissionAddressCreate, PermissionAddressUpdate,
		PermissionProviderRead,
		PermissionProviderAccountRead,
		PermissionQuoteRead, PermissionQuoteCreate,
		PermissionShipmentRead, PermissionShipmentCreate, PermissionShipmentUpdate, PermissionShipmentCancel, PermissionShipmentLabel, PermissionShipmentPickup, PermissionShipmentReturn,
		PermissionTrackingRead,
		PermissionWebhookRead,
		PermissionBillingRead,
	}
}

// SystemRoleViewer returns the permissions for the viewer role.
func SystemRoleViewer() []string {
	return []string{
		PermissionOrgRead,
		PermissionMemberRead,
		PermissionAddressRead,
		PermissionProviderRead,
		PermissionProviderAccountRead,
		PermissionQuoteRead,
		PermissionShipmentRead,
		PermissionTrackingRead,
		PermissionWebhookRead,
		PermissionBillingRead,
	}
}
