// Package domain contains the core domain models for the shipping aggregation platform.
// These models are pure domain objects with no external dependencies.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// OrganizationStatus represents the status of an organization account.
type OrganizationStatus string

const (
	// OrganizationStatusActive indicates the organization is active and can use the platform.
	OrganizationStatusActive OrganizationStatus = "active"
	// OrganizationStatusSuspended indicates the organization is temporarily suspended.
	OrganizationStatusSuspended OrganizationStatus = "suspended"
	// OrganizationStatusClosed indicates the organization account has been closed.
	OrganizationStatusClosed OrganizationStatus = "closed"
)

// IsValid checks if the organization status is a valid value.
func (s OrganizationStatus) IsValid() bool {
	switch s {
	case OrganizationStatusActive, OrganizationStatusSuspended, OrganizationStatusClosed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the organization status.
func (s OrganizationStatus) String() string {
	return string(s)
}

// OrganizationSettings contains organization-level configuration settings.
type OrganizationSettings struct {
	// DefaultLabelFormat is the preferred label format (pdf, zpl, png).
	DefaultLabelFormat string `json:"default_label_format,omitempty"`
	// DefaultCurrency is the preferred currency (default: TRY).
	DefaultCurrency string `json:"default_currency,omitempty"`
	// WebhookRetryEnabled indicates if webhook retries are enabled.
	WebhookRetryEnabled *bool `json:"webhook_retry_enabled,omitempty"`
	// NotificationEmail is the email for system notifications.
	NotificationEmail string `json:"notification_email,omitempty"`
	// Timezone is the organization's preferred timezone.
	Timezone string `json:"timezone,omitempty"`
}

// Organization represents a multi-tenant root entity.
// All customer resources belong to an organization.
type Organization struct {
	// ID is the unique identifier for the organization.
	ID uuid.UUID `json:"id"`
	// Name is the display name of the organization.
	Name string `json:"name"`
	// Slug is the URL-safe identifier for the organization.
	Slug string `json:"slug"`
	// BillingEmail is the primary billing contact email.
	BillingEmail string `json:"billing_email"`
	// Settings contains organization-level configuration.
	Settings OrganizationSettings `json:"settings"`
	// Status is the current account status.
	Status OrganizationStatus `json:"status"`
	// CreatedAt is the timestamp when the organization was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the organization was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// DeletedAt is the soft delete timestamp (nil if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// NewOrganization creates a new Organization with the given parameters.
func NewOrganization(name, slug, billingEmail string) *Organization {
	now := time.Now().UTC()
	return &Organization{
		ID:           uuid.New(),
		Name:         name,
		Slug:         slug,
		BillingEmail: billingEmail,
		Settings:     OrganizationSettings{},
		Status:       OrganizationStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// IsActive returns true if the organization is in active status.
func (o *Organization) IsActive() bool {
	return o.Status == OrganizationStatusActive && o.DeletedAt == nil
}

// IsSuspended returns true if the organization is suspended.
func (o *Organization) IsSuspended() bool {
	return o.Status == OrganizationStatusSuspended
}

// IsClosed returns true if the organization is closed.
func (o *Organization) IsClosed() bool {
	return o.Status == OrganizationStatusClosed
}

// IsDeleted returns true if the organization has been soft deleted.
func (o *Organization) IsDeleted() bool {
	return o.DeletedAt != nil
}

// Suspend suspends the organization account.
func (o *Organization) Suspend() {
	o.Status = OrganizationStatusSuspended
	o.UpdatedAt = time.Now().UTC()
}

// Activate activates the organization account.
func (o *Organization) Activate() {
	o.Status = OrganizationStatusActive
	o.UpdatedAt = time.Now().UTC()
}

// Close closes the organization account.
func (o *Organization) Close() {
	o.Status = OrganizationStatusClosed
	o.UpdatedAt = time.Now().UTC()
}

// SoftDelete marks the organization as deleted.
func (o *Organization) SoftDelete() {
	now := time.Now().UTC()
	o.DeletedAt = &now
	o.UpdatedAt = now
}

// UpdateName updates the organization name.
func (o *Organization) UpdateName(name string) {
	o.Name = name
	o.UpdatedAt = time.Now().UTC()
}

// UpdateBillingEmail updates the billing email.
func (o *Organization) UpdateBillingEmail(email string) {
	o.BillingEmail = email
	o.UpdatedAt = time.Now().UTC()
}

// UpdateSettings updates the organization settings.
func (o *Organization) UpdateSettings(settings OrganizationSettings) {
	o.Settings = settings
	o.UpdatedAt = time.Now().UTC()
}

// MarshalSettingsJSON marshals the settings to JSON.
func (o *Organization) MarshalSettingsJSON() (json.RawMessage, error) {
	return json.Marshal(o.Settings)
}

// UnmarshalSettingsJSON unmarshals settings from JSON.
func (o *Organization) UnmarshalSettingsJSON(data json.RawMessage) error {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		o.Settings = OrganizationSettings{}
		return nil
	}
	return json.Unmarshal(data, &o.Settings)
}
