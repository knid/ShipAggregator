package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProviderAPIType represents the API protocol type of a provider.
type ProviderAPIType string

const (
	// ProviderAPITypeSOAP indicates a SOAP-based API.
	ProviderAPITypeSOAP ProviderAPIType = "soap"
	// ProviderAPITypeREST indicates a REST-based API.
	ProviderAPITypeREST ProviderAPIType = "rest"
)

// IsValid checks if the provider API type is valid.
func (t ProviderAPIType) IsValid() bool {
	switch t {
	case ProviderAPITypeSOAP, ProviderAPITypeREST:
		return true
	default:
		return false
	}
}

// String returns the string representation of the provider API type.
func (t ProviderAPIType) String() string {
	return string(t)
}

// ProviderCode represents a carrier provider code.
type ProviderCode string

const (
	// ProviderCodeAras is the code for Aras Kargo.
	ProviderCodeAras ProviderCode = "aras"
	// ProviderCodeYurtici is the code for Yurtiçi Kargo.
	ProviderCodeYurtici ProviderCode = "yurtici"
	// ProviderCodeSurat is the code for Sürat Kargo.
	ProviderCodeSurat ProviderCode = "surat"
	// ProviderCodePTT is the code for PTT Kargo.
	ProviderCodePTT ProviderCode = "ptt"
	// ProviderCodeHepsijet is the code for Hepsijet.
	ProviderCodeHepsijet ProviderCode = "hepsijet"
	// ProviderCodeMNG is the code for MNG Kargo.
	ProviderCodeMNG ProviderCode = "mng"
	// ProviderCodeKolaygelsin is the code for Kolaygelsin.
	ProviderCodeKolaygelsin ProviderCode = "kolaygelsin"
	// ProviderCodeSendeo is the code for Sendeo.
	ProviderCodeSendeo ProviderCode = "sendeo"
	// ProviderCodePakettaxi is the code for Pakettaxi.
	ProviderCodePakettaxi ProviderCode = "pakettaxi"
)

// IsValid checks if the provider code is valid.
func (c ProviderCode) IsValid() bool {
	switch c {
	case ProviderCodeAras, ProviderCodeYurtici, ProviderCodeSurat, ProviderCodePTT,
		ProviderCodeHepsijet, ProviderCodeMNG, ProviderCodeKolaygelsin,
		ProviderCodeSendeo, ProviderCodePakettaxi:
		return true
	default:
		return false
	}
}

// String returns the string representation of the provider code.
func (c ProviderCode) String() string {
	return string(c)
}

// AllProviderCodes returns all valid provider codes.
func AllProviderCodes() []ProviderCode {
	return []ProviderCode{
		ProviderCodeAras, ProviderCodeYurtici, ProviderCodeSurat, ProviderCodePTT,
		ProviderCodeHepsijet, ProviderCodeMNG, ProviderCodeKolaygelsin,
		ProviderCodeSendeo, ProviderCodePakettaxi,
	}
}

// ProviderCapabilities represents the capabilities of a provider.
type ProviderCapabilities struct {
	// COD indicates if the provider supports Cash on Delivery.
	COD bool `json:"cod"`
	// Insurance indicates if the provider supports insurance.
	Insurance bool `json:"insurance"`
	// Returns indicates if the provider supports return shipments.
	Returns bool `json:"returns"`
	// Pickup indicates if the provider supports pickup scheduling.
	Pickup bool `json:"pickup"`
	// DropoffPoints indicates if the provider has dropoff points.
	DropoffPoints bool `json:"dropoff_points"`
	// TrackingWebhook indicates if the provider supports tracking webhooks.
	TrackingWebhook bool `json:"tracking_webhook"`
	// TrackingPoll indicates if the provider supports tracking polling.
	TrackingPoll bool `json:"tracking_poll"`
	// MultiParcel indicates if the provider supports multi-parcel shipments.
	MultiParcel bool `json:"multi_parcel"`
	// SameDayDelivery indicates if the provider supports same-day delivery.
	SameDayDelivery bool `json:"same_day_delivery"`
	// NextDayDelivery indicates if the provider supports next-day delivery.
	NextDayDelivery bool `json:"next_day_delivery"`
	// InternationalShipping indicates if the provider supports international shipping.
	InternationalShipping bool `json:"international_shipping"`
}

// Provider represents an integrated carrier provider.
type Provider struct {
	// Code is the unique provider code.
	Code ProviderCode `json:"code"`
	// Name is the display name.
	Name string `json:"name"`
	// LogoURL is the logo image URL.
	LogoURL *string `json:"logo_url,omitempty"`
	// Website is the provider website.
	Website *string `json:"website,omitempty"`
	// SupportEmail is the support contact email.
	SupportEmail *string `json:"support_email,omitempty"`
	// SupportPhone is the support contact phone.
	SupportPhone *string `json:"support_phone,omitempty"`
	// APIType is the API protocol type (soap or rest).
	APIType ProviderAPIType `json:"api_type"`
	// APIBaseURL is the base URL for API calls.
	APIBaseURL string `json:"api_base_url"`
	// APISandboxURL is the sandbox environment URL.
	APISandboxURL *string `json:"api_sandbox_url,omitempty"`
	// IsActive indicates if the provider is available.
	IsActive bool `json:"is_active"`
	// Capabilities contains the provider capability flags.
	Capabilities ProviderCapabilities `json:"capabilities"`
	// CreatedAt is the timestamp when the provider was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the provider was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// MarshalCapabilitiesJSON marshals the capabilities to JSON.
func (p *Provider) MarshalCapabilitiesJSON() (json.RawMessage, error) {
	return json.Marshal(p.Capabilities)
}

// UnmarshalCapabilitiesJSON unmarshals capabilities from JSON.
func (p *Provider) UnmarshalCapabilitiesJSON(data json.RawMessage) error {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		p.Capabilities = ProviderCapabilities{}
		return nil
	}
	return json.Unmarshal(data, &p.Capabilities)
}

// SupportsFeature checks if the provider supports a specific feature.
func (p *Provider) SupportsFeature(feature string) bool {
	switch feature {
	case "cod":
		return p.Capabilities.COD
	case "insurance":
		return p.Capabilities.Insurance
	case "returns":
		return p.Capabilities.Returns
	case "pickup":
		return p.Capabilities.Pickup
	case "dropoff_points":
		return p.Capabilities.DropoffPoints
	case "tracking_webhook":
		return p.Capabilities.TrackingWebhook
	case "tracking_poll":
		return p.Capabilities.TrackingPoll
	case "multi_parcel":
		return p.Capabilities.MultiParcel
	case "same_day_delivery":
		return p.Capabilities.SameDayDelivery
	case "next_day_delivery":
		return p.Capabilities.NextDayDelivery
	case "international_shipping":
		return p.Capabilities.InternationalShipping
	default:
		return false
	}
}

// ProviderService represents a service level offered by a provider.
type ProviderService struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// ProviderCode is the parent provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// ServiceCode is the provider-specific service code.
	ServiceCode string `json:"service_code"`
	// Name is the service display name.
	Name string `json:"name"`
	// Description is the service description.
	Description *string `json:"description,omitempty"`
	// EstimatedDaysMin is the minimum delivery days.
	EstimatedDaysMin int `json:"estimated_days_min"`
	// EstimatedDaysMax is the maximum delivery days.
	EstimatedDaysMax int `json:"estimated_days_max"`
	// SupportsCOD indicates if the service supports COD.
	SupportsCOD bool `json:"supports_cod"`
	// SupportsInsurance indicates if the service supports insurance.
	SupportsInsurance bool `json:"supports_insurance"`
	// SupportsReturn indicates if the service supports returns.
	SupportsReturn bool `json:"supports_return"`
	// MaxWeightGrams is the maximum weight in grams.
	MaxWeightGrams int `json:"max_weight_grams"`
	// MaxLengthCm is the maximum length in cm.
	MaxLengthCm int `json:"max_length_cm"`
	// MaxWidthCm is the maximum width in cm.
	MaxWidthCm int `json:"max_width_cm"`
	// MaxHeightCm is the maximum height in cm.
	MaxHeightCm int `json:"max_height_cm"`
	// LabelFormats is the list of supported label formats.
	LabelFormats []string `json:"label_formats"`
	// IsActive indicates if the service is available.
	IsActive bool `json:"is_active"`
	// CreatedAt is the timestamp when the service was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the service was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// CanHandleParcel checks if the service can handle a parcel with the given dimensions.
func (s *ProviderService) CanHandleParcel(weightGrams, lengthCm, widthCm, heightCm int) bool {
	return weightGrams <= s.MaxWeightGrams &&
		lengthCm <= s.MaxLengthCm &&
		widthCm <= s.MaxWidthCm &&
		heightCm <= s.MaxHeightCm
}

// SupportsLabelFormat checks if the service supports a specific label format.
func (s *ProviderService) SupportsLabelFormat(format string) bool {
	for _, f := range s.LabelFormats {
		if f == format {
			return true
		}
	}
	return false
}

// ProviderAccountStatus represents the status of a provider account.
type ProviderAccountStatus string

const (
	// ProviderAccountStatusPendingVerification indicates the account is pending verification.
	ProviderAccountStatusPendingVerification ProviderAccountStatus = "pending_verification"
	// ProviderAccountStatusVerified indicates the account has been verified.
	ProviderAccountStatusVerified ProviderAccountStatus = "verified"
	// ProviderAccountStatusVerificationFailed indicates verification failed.
	ProviderAccountStatusVerificationFailed ProviderAccountStatus = "verification_failed"
	// ProviderAccountStatusSuspended indicates the account is suspended.
	ProviderAccountStatusSuspended ProviderAccountStatus = "suspended"
)

// IsValid checks if the provider account status is valid.
func (s ProviderAccountStatus) IsValid() bool {
	switch s {
	case ProviderAccountStatusPendingVerification, ProviderAccountStatusVerified,
		ProviderAccountStatusVerificationFailed, ProviderAccountStatusSuspended:
		return true
	default:
		return false
	}
}

// String returns the string representation of the provider account status.
func (s ProviderAccountStatus) String() string {
	return string(s)
}

// ProviderAccount represents BYOA credentials for customer-owned carrier accounts.
type ProviderAccount struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// Name is the account name/label.
	Name string `json:"name"`
	// CredentialsEncrypted is the AES-256-GCM encrypted credentials blob.
	CredentialsEncrypted []byte `json:"-"`
	// CredentialsIV is the encryption initialization vector.
	CredentialsIV []byte `json:"-"`
	// Status is the account verification status.
	Status ProviderAccountStatus `json:"status"`
	// VerificationError is the last verification error message.
	VerificationError *string `json:"verification_error,omitempty"`
	// VerifiedAt is the verification timestamp.
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	// LastUsedAt is the last successful use timestamp.
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// IsDefault indicates if this is the default account for the provider.
	IsDefault bool `json:"is_default"`
	// CreatedAt is the timestamp when the account was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the account was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// DeletedAt is the soft delete timestamp.
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// NewProviderAccount creates a new provider account.
func NewProviderAccount(organizationID uuid.UUID, providerCode ProviderCode, name string, credentialsEncrypted, credentialsIV []byte) *ProviderAccount {
	now := time.Now().UTC()
	return &ProviderAccount{
		ID:                   uuid.New(),
		OrganizationID:       organizationID,
		ProviderCode:         providerCode,
		Name:                 name,
		CredentialsEncrypted: credentialsEncrypted,
		CredentialsIV:        credentialsIV,
		Status:               ProviderAccountStatusPendingVerification,
		IsDefault:            false,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// IsVerified returns true if the account is verified.
func (a *ProviderAccount) IsVerified() bool {
	return a.Status == ProviderAccountStatusVerified
}

// IsPendingVerification returns true if the account is pending verification.
func (a *ProviderAccount) IsPendingVerification() bool {
	return a.Status == ProviderAccountStatusPendingVerification
}

// IsVerificationFailed returns true if verification failed.
func (a *ProviderAccount) IsVerificationFailed() bool {
	return a.Status == ProviderAccountStatusVerificationFailed
}

// IsSuspended returns true if the account is suspended.
func (a *ProviderAccount) IsSuspended() bool {
	return a.Status == ProviderAccountStatusSuspended
}

// IsDeleted returns true if the account has been soft deleted.
func (a *ProviderAccount) IsDeleted() bool {
	return a.DeletedAt != nil
}

// CanBeUsed returns true if the account can be used for shipments.
func (a *ProviderAccount) CanBeUsed() bool {
	return a.IsVerified() && !a.IsDeleted()
}

// MarkVerified marks the account as verified.
func (a *ProviderAccount) MarkVerified() {
	now := time.Now().UTC()
	a.Status = ProviderAccountStatusVerified
	a.VerifiedAt = &now
	a.VerificationError = nil
	a.UpdatedAt = now
}

// MarkVerificationFailed marks the account as verification failed.
func (a *ProviderAccount) MarkVerificationFailed(errorMsg string) {
	a.Status = ProviderAccountStatusVerificationFailed
	a.VerificationError = &errorMsg
	a.UpdatedAt = time.Now().UTC()
}

// Suspend suspends the account.
func (a *ProviderAccount) Suspend() {
	a.Status = ProviderAccountStatusSuspended
	a.UpdatedAt = time.Now().UTC()
}

// ResetToPendingVerification resets the account to pending verification.
func (a *ProviderAccount) ResetToPendingVerification() {
	a.Status = ProviderAccountStatusPendingVerification
	a.VerificationError = nil
	a.VerifiedAt = nil
	a.UpdatedAt = time.Now().UTC()
}

// SoftDelete marks the account as deleted.
func (a *ProviderAccount) SoftDelete() {
	now := time.Now().UTC()
	a.DeletedAt = &now
	a.UpdatedAt = now
}

// UpdateName updates the account name.
func (a *ProviderAccount) UpdateName(name string) {
	a.Name = name
	a.UpdatedAt = time.Now().UTC()
}

// UpdateCredentials updates the encrypted credentials.
func (a *ProviderAccount) UpdateCredentials(credentialsEncrypted, credentialsIV []byte) {
	a.CredentialsEncrypted = credentialsEncrypted
	a.CredentialsIV = credentialsIV
	a.Status = ProviderAccountStatusPendingVerification
	a.VerificationError = nil
	a.VerifiedAt = nil
	a.UpdatedAt = time.Now().UTC()
}

// SetAsDefault sets this account as the default for the provider.
func (a *ProviderAccount) SetAsDefault() {
	a.IsDefault = true
	a.UpdatedAt = time.Now().UTC()
}

// ClearDefault removes the default flag.
func (a *ProviderAccount) ClearDefault() {
	a.IsDefault = false
	a.UpdatedAt = time.Now().UTC()
}

// RecordUsage records a successful usage of the account.
func (a *ProviderAccount) RecordUsage() {
	now := time.Now().UTC()
	a.LastUsedAt = &now
}

// ProviderCredentials represents the decrypted credentials for a provider.
// The structure varies by provider.
type ProviderCredentials struct {
	// Username is the API username (common field).
	Username string `json:"username,omitempty"`
	// Password is the API password (common field).
	Password string `json:"password,omitempty"`
	// APIKey is the API key (for REST providers).
	APIKey string `json:"api_key,omitempty"`
	// APISecret is the API secret (for REST providers).
	APISecret string `json:"api_secret,omitempty"`
	// CustomerCode is the customer/merchant code.
	CustomerCode string `json:"customer_code,omitempty"`
	// AccountNumber is the account number.
	AccountNumber string `json:"account_number,omitempty"`
	// ContractID is the contract identifier.
	ContractID string `json:"contract_id,omitempty"`
	// Extra contains provider-specific additional fields.
	Extra map[string]string `json:"extra,omitempty"`
}

// MarketplaceRatePlan represents internal pricing configuration for marketplace model.
type MarketplaceRatePlan struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// ServiceCode is the service code.
	ServiceCode string `json:"service_code"`
	// Name is the rate plan name.
	Name string `json:"name"`
	// BasePriceKurus is the base price in kuruş.
	BasePriceKurus int64 `json:"base_price_kurus"`
	// PricePerKgKurus is the per-kg surcharge in kuruş.
	PricePerKgKurus int64 `json:"price_per_kg_kurus"`
	// PricePerDesiKurus is the per-desi surcharge in kuruş.
	PricePerDesiKurus int64 `json:"price_per_desi_kurus"`
	// FuelSurchargePercent is the fuel surcharge percentage.
	FuelSurchargePercent float64 `json:"fuel_surcharge_percent"`
	// CODFeePercent is the COD fee percentage.
	CODFeePercent float64 `json:"cod_fee_percent"`
	// CODFeeMinKurus is the minimum COD fee in kuruş.
	CODFeeMinKurus int64 `json:"cod_fee_min_kurus"`
	// VATPercent is the VAT percentage (default 20% for Turkey).
	VATPercent float64 `json:"vat_percent"`
	// EffectiveFrom is the rate effective start date.
	EffectiveFrom time.Time `json:"effective_from"`
	// EffectiveTo is the rate effective end date (nil = current).
	EffectiveTo *time.Time `json:"effective_to,omitempty"`
	// IsActive indicates if the rate plan is active.
	IsActive bool `json:"is_active"`
	// CreatedAt is the timestamp when the rate plan was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the rate plan was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// IsEffective checks if the rate plan is effective at the given time.
func (r *MarketplaceRatePlan) IsEffective(at time.Time) bool {
	if !r.IsActive {
		return false
	}
	if at.Before(r.EffectiveFrom) {
		return false
	}
	if r.EffectiveTo != nil && at.After(*r.EffectiveTo) {
		return false
	}
	return true
}

// CalculatePrice calculates the price for a shipment using this rate plan.
func (r *MarketplaceRatePlan) CalculatePrice(weightGrams int, desi float64, codAmountKurus int64) PriceBreakdown {
	// Base price
	basePrice := r.BasePriceKurus

	// Weight surcharge (convert grams to kg, round up)
	weightKg := (weightGrams + 999) / 1000
	weightSurcharge := int64(weightKg) * r.PricePerKgKurus

	// Desi surcharge
	desiSurcharge := int64(desi * float64(r.PricePerDesiKurus))

	// Use the higher of weight or desi surcharge
	surcharge := weightSurcharge
	if desiSurcharge > weightSurcharge {
		surcharge = desiSurcharge
	}

	// Fuel surcharge
	subtotalBeforeFuel := basePrice + surcharge
	fuelSurcharge := int64(float64(subtotalBeforeFuel) * r.FuelSurchargePercent / 100)

	// COD fee
	var codFee int64
	if codAmountKurus > 0 {
		codFee = int64(float64(codAmountKurus) * r.CODFeePercent / 100)
		if codFee < r.CODFeeMinKurus {
			codFee = r.CODFeeMinKurus
		}
	}

	// Subtotal before VAT
	subtotal := basePrice + surcharge + fuelSurcharge + codFee

	// VAT
	vat := int64(float64(subtotal) * r.VATPercent / 100)

	// Total
	total := subtotal + vat

	return PriceBreakdown{
		BasePriceKurus:     basePrice,
		FuelSurchargeKurus: fuelSurcharge,
		CODFeeKurus:        codFee,
		InsuranceFeeKurus:  0,
		OtherFeesKurus:     surcharge,
		SubtotalKurus:      subtotal,
		VATKurus:           vat,
		TotalPriceKurus:    total,
	}
}

// PriceBreakdown represents a detailed price breakdown.
type PriceBreakdown struct {
	// BasePriceKurus is the base shipping price in kuruş.
	BasePriceKurus int64 `json:"base_price_kurus"`
	// FuelSurchargeKurus is the fuel surcharge in kuruş.
	FuelSurchargeKurus int64 `json:"fuel_surcharge_kurus"`
	// CODFeeKurus is the COD handling fee in kuruş.
	CODFeeKurus int64 `json:"cod_fee_kurus"`
	// InsuranceFeeKurus is the insurance fee in kuruş.
	InsuranceFeeKurus int64 `json:"insurance_fee_kurus"`
	// OtherFeesKurus is other fees in kuruş.
	OtherFeesKurus int64 `json:"other_fees_kurus"`
	// SubtotalKurus is the subtotal before VAT in kuruş.
	SubtotalKurus int64 `json:"subtotal_kurus"`
	// VATKurus is the VAT amount in kuruş.
	VATKurus int64 `json:"vat_kurus"`
	// TotalPriceKurus is the total price including VAT in kuruş.
	TotalPriceKurus int64 `json:"total_price_kurus"`
}
