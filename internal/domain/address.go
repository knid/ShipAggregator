package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Address represents a saved address for an organization.
type Address struct {
	// ID is the unique identifier for the address.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the ID of the parent organization.
	OrganizationID uuid.UUID `json:"organization_id"`
	// Label is a user-defined label (e.g., "Warehouse", "HQ").
	Label *string `json:"label,omitempty"`
	// ContactName is the contact person name.
	ContactName string `json:"contact_name"`
	// CompanyName is the company/business name.
	CompanyName *string `json:"company_name,omitempty"`
	// Phone is the phone number in E.164 format.
	Phone string `json:"phone"`
	// Email is the contact email.
	Email *string `json:"email,omitempty"`
	// StreetLine1 is the primary street address.
	StreetLine1 string `json:"street_line_1"`
	// StreetLine2 is the secondary address line.
	StreetLine2 *string `json:"street_line_2,omitempty"`
	// NeighborhoodCode is the reference to Turkish neighborhood (mahalle).
	NeighborhoodCode string `json:"neighborhood_code"`
	// DistrictCode is the reference to Turkish district (ilçe).
	DistrictCode string `json:"district_code"`
	// CityCode is the reference to Turkish city (il).
	CityCode string `json:"city_code"`
	// PostalCode is the postal code.
	PostalCode *string `json:"postal_code,omitempty"`
	// Latitude is the GPS latitude.
	Latitude *decimal.Decimal `json:"latitude,omitempty"`
	// Longitude is the GPS longitude.
	Longitude *decimal.Decimal `json:"longitude,omitempty"`
	// IsDefaultSender indicates if this is the default sender address.
	IsDefaultSender bool `json:"is_default_sender"`
	// IsDefaultRecipient indicates if this is the default recipient address.
	IsDefaultRecipient bool `json:"is_default_recipient"`
	// CreatedAt is the timestamp when the address was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the address was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// DeletedAt is the soft delete timestamp (nil if not deleted).
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// NewAddress creates a new Address with the given parameters.
func NewAddress(organizationID uuid.UUID, contactName, phone, streetLine1, neighborhoodCode, districtCode, cityCode string) *Address {
	now := time.Now().UTC()
	return &Address{
		ID:                 uuid.New(),
		OrganizationID:     organizationID,
		ContactName:        contactName,
		Phone:              phone,
		StreetLine1:        streetLine1,
		NeighborhoodCode:   neighborhoodCode,
		DistrictCode:       districtCode,
		CityCode:           cityCode,
		IsDefaultSender:    false,
		IsDefaultRecipient: false,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// IsDeleted returns true if the address has been soft deleted.
func (a *Address) IsDeleted() bool {
	return a.DeletedAt != nil
}

// SoftDelete marks the address as deleted.
func (a *Address) SoftDelete() {
	now := time.Now().UTC()
	a.DeletedAt = &now
	a.UpdatedAt = now
}

// SetLabel sets the address label.
func (a *Address) SetLabel(label string) {
	a.Label = &label
	a.UpdatedAt = time.Now().UTC()
}

// ClearLabel clears the address label.
func (a *Address) ClearLabel() {
	a.Label = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetCompanyName sets the company name.
func (a *Address) SetCompanyName(name string) {
	a.CompanyName = &name
	a.UpdatedAt = time.Now().UTC()
}

// ClearCompanyName clears the company name.
func (a *Address) ClearCompanyName() {
	a.CompanyName = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetEmail sets the email address.
func (a *Address) SetEmail(email string) {
	a.Email = &email
	a.UpdatedAt = time.Now().UTC()
}

// ClearEmail clears the email address.
func (a *Address) ClearEmail() {
	a.Email = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetStreetLine2 sets the secondary address line.
func (a *Address) SetStreetLine2(line string) {
	a.StreetLine2 = &line
	a.UpdatedAt = time.Now().UTC()
}

// ClearStreetLine2 clears the secondary address line.
func (a *Address) ClearStreetLine2() {
	a.StreetLine2 = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetPostalCode sets the postal code.
func (a *Address) SetPostalCode(code string) {
	a.PostalCode = &code
	a.UpdatedAt = time.Now().UTC()
}

// ClearPostalCode clears the postal code.
func (a *Address) ClearPostalCode() {
	a.PostalCode = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetCoordinates sets the GPS coordinates.
func (a *Address) SetCoordinates(lat, lng decimal.Decimal) {
	a.Latitude = &lat
	a.Longitude = &lng
	a.UpdatedAt = time.Now().UTC()
}

// ClearCoordinates clears the GPS coordinates.
func (a *Address) ClearCoordinates() {
	a.Latitude = nil
	a.Longitude = nil
	a.UpdatedAt = time.Now().UTC()
}

// HasCoordinates returns true if the address has GPS coordinates.
func (a *Address) HasCoordinates() bool {
	return a.Latitude != nil && a.Longitude != nil
}

// SetAsDefaultSender sets this address as the default sender.
func (a *Address) SetAsDefaultSender() {
	a.IsDefaultSender = true
	a.UpdatedAt = time.Now().UTC()
}

// ClearDefaultSender removes the default sender flag.
func (a *Address) ClearDefaultSender() {
	a.IsDefaultSender = false
	a.UpdatedAt = time.Now().UTC()
}

// SetAsDefaultRecipient sets this address as the default recipient.
func (a *Address) SetAsDefaultRecipient() {
	a.IsDefaultRecipient = true
	a.UpdatedAt = time.Now().UTC()
}

// ClearDefaultRecipient removes the default recipient flag.
func (a *Address) ClearDefaultRecipient() {
	a.IsDefaultRecipient = false
	a.UpdatedAt = time.Now().UTC()
}

// Update updates the address fields.
func (a *Address) Update(contactName, phone, streetLine1, neighborhoodCode, districtCode, cityCode string) {
	a.ContactName = contactName
	a.Phone = phone
	a.StreetLine1 = streetLine1
	a.NeighborhoodCode = neighborhoodCode
	a.DistrictCode = districtCode
	a.CityCode = cityCode
	a.UpdatedAt = time.Now().UTC()
}

// AddressSnapshot represents an immutable snapshot of an address for shipments.
// This is stored as JSON in quotes and shipments to preserve the address at the time of creation.
type AddressSnapshot struct {
	// ContactName is the contact person name.
	ContactName string `json:"contact_name"`
	// CompanyName is the company/business name.
	CompanyName *string `json:"company_name,omitempty"`
	// Phone is the phone number in E.164 format.
	Phone string `json:"phone"`
	// Email is the contact email.
	Email *string `json:"email,omitempty"`
	// StreetLine1 is the primary street address.
	StreetLine1 string `json:"street_line_1"`
	// StreetLine2 is the secondary address line.
	StreetLine2 *string `json:"street_line_2,omitempty"`
	// Neighborhood is the neighborhood name.
	Neighborhood string `json:"neighborhood"`
	// NeighborhoodCode is the neighborhood code.
	NeighborhoodCode string `json:"neighborhood_code"`
	// District is the district name.
	District string `json:"district"`
	// DistrictCode is the district code.
	DistrictCode string `json:"district_code"`
	// City is the city name.
	City string `json:"city"`
	// CityCode is the city code.
	CityCode string `json:"city_code"`
	// PostalCode is the postal code.
	PostalCode *string `json:"postal_code,omitempty"`
	// Latitude is the GPS latitude.
	Latitude *decimal.Decimal `json:"latitude,omitempty"`
	// Longitude is the GPS longitude.
	Longitude *decimal.Decimal `json:"longitude,omitempty"`
}

// ToSnapshot converts an Address to an AddressSnapshot.
// Note: This requires the resolved names for neighborhood, district, and city.
func (a *Address) ToSnapshot(neighborhood, district, city string) AddressSnapshot {
	return AddressSnapshot{
		ContactName:      a.ContactName,
		CompanyName:      a.CompanyName,
		Phone:            a.Phone,
		Email:            a.Email,
		StreetLine1:      a.StreetLine1,
		StreetLine2:      a.StreetLine2,
		Neighborhood:     neighborhood,
		NeighborhoodCode: a.NeighborhoodCode,
		District:         district,
		DistrictCode:     a.DistrictCode,
		City:             city,
		CityCode:         a.CityCode,
		PostalCode:       a.PostalCode,
		Latitude:         a.Latitude,
		Longitude:        a.Longitude,
	}
}

// FullAddress returns a formatted full address string.
func (s *AddressSnapshot) FullAddress() string {
	addr := s.StreetLine1
	if s.StreetLine2 != nil && *s.StreetLine2 != "" {
		addr += ", " + *s.StreetLine2
	}
	addr += ", " + s.Neighborhood
	addr += ", " + s.District
	addr += ", " + s.City
	if s.PostalCode != nil && *s.PostalCode != "" {
		addr += " " + *s.PostalCode
	}
	return addr
}

// RefCity represents a Turkish city reference data.
type RefCity struct {
	// Code is the city plate code (e.g., "34" for Istanbul).
	Code string `json:"code"`
	// Name is the city name.
	Name string `json:"name"`
	// NameNormalized is the lowercase, ASCII normalized name for search.
	NameNormalized string `json:"name_normalized"`
	// IsActive indicates if the city is serviceable.
	IsActive bool `json:"is_active"`
	// UpdatedAt is the last sync timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// RefDistrict represents a Turkish district (ilçe) reference data.
type RefDistrict struct {
	// Code is the district code.
	Code string `json:"code"`
	// CityCode is the parent city code.
	CityCode string `json:"city_code"`
	// Name is the district name.
	Name string `json:"name"`
	// NameNormalized is the lowercase, ASCII normalized name for search.
	NameNormalized string `json:"name_normalized"`
	// IsActive indicates if the district is serviceable.
	IsActive bool `json:"is_active"`
	// UpdatedAt is the last sync timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// RefNeighborhood represents a Turkish neighborhood (mahalle) reference data.
type RefNeighborhood struct {
	// Code is the neighborhood code.
	Code string `json:"code"`
	// DistrictCode is the parent district code.
	DistrictCode string `json:"district_code"`
	// Name is the neighborhood name.
	Name string `json:"name"`
	// NameNormalized is the lowercase, ASCII normalized name for search.
	NameNormalized string `json:"name_normalized"`
	// PostalCode is the associated postal code.
	PostalCode *string `json:"postal_code,omitempty"`
	// IsActive indicates if the neighborhood is serviceable.
	IsActive bool `json:"is_active"`
	// UpdatedAt is the last sync timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}

// RefPostalCode represents a Turkish postal code reference data.
type RefPostalCode struct {
	// Code is the 5-digit Turkish postal code.
	Code string `json:"code"`
	// DistrictCode is the parent district code.
	DistrictCode string `json:"district_code"`
	// IsActive indicates if the postal code is serviceable.
	IsActive bool `json:"is_active"`
	// UpdatedAt is the last sync timestamp.
	UpdatedAt time.Time `json:"updated_at"`
}
