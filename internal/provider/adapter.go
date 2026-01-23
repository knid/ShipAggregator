// Package provider implements the provider adapter framework for integrating
// with Turkish cargo carriers. It defines the common interface that all
// provider adapters must implement, along with shared types and utilities.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/domain"
)

// Common errors returned by provider adapters.
var (
	// ErrProviderNotFound indicates the requested provider does not exist.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrProviderUnavailable indicates the provider is temporarily unavailable.
	ErrProviderUnavailable = errors.New("provider unavailable")
	// ErrProviderTimeout indicates the provider request timed out.
	ErrProviderTimeout = errors.New("provider timeout")
	// ErrProviderAuthFailed indicates authentication with the provider failed.
	ErrProviderAuthFailed = errors.New("provider authentication failed")
	// ErrProviderRejected indicates the provider rejected the request.
	ErrProviderRejected = errors.New("provider rejected request")
	// ErrProviderRateLimited indicates the provider rate limited the request.
	ErrProviderRateLimited = errors.New("provider rate limited")
	// ErrUnsupportedOperation indicates the operation is not supported by the provider.
	ErrUnsupportedOperation = errors.New("operation not supported by provider")
	// ErrInvalidCredentials indicates the provided credentials are invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidRequest indicates the request is invalid.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrCircuitOpen indicates the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker open")
)

// ProviderError represents an error from a provider API.
type ProviderError struct {
	// Code is the normalized error code.
	Code string `json:"code"`
	// Message is the human-readable error message.
	Message string `json:"message"`
	// ProviderCode is the provider's error code.
	ProviderCode string `json:"provider_code,omitempty"`
	// ProviderMessage is the provider's error message.
	ProviderMessage string `json:"provider_message,omitempty"`
	// Retryable indicates if the error is retryable.
	Retryable bool `json:"retryable"`
	// RetryAfter is the duration to wait before retrying (for rate limit errors).
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	// Cause is the underlying error.
	Cause error `json:"-"`
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.ProviderMessage != "" {
		return fmt.Sprintf("%s: %s (provider: %s - %s)", e.Code, e.Message, e.ProviderCode, e.ProviderMessage)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a new ProviderError.
func NewProviderError(code, message string, retryable bool) *ProviderError {
	return &ProviderError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

// WithProviderDetails adds provider-specific error details.
func (e *ProviderError) WithProviderDetails(code, message string) *ProviderError {
	e.ProviderCode = code
	e.ProviderMessage = message
	return e
}

// WithCause adds an underlying cause to the error.
func (e *ProviderError) WithCause(cause error) *ProviderError {
	e.Cause = cause
	return e
}

// Adapter defines the interface that all provider adapters must implement.
// Each carrier (Aras, Yurtiçi, Hepsijet, etc.) implements this interface
// to provide a unified API for the shipping aggregation platform.
type Adapter interface {
	// ProviderCode returns the unique provider code (e.g., "aras", "yurtici").
	ProviderCode() domain.ProviderCode

	// APIType returns the API protocol type ("soap" or "rest").
	APIType() domain.ProviderAPIType

	// VerifyCredentials validates the provided credentials with the carrier API.
	// Returns verification result with account info if successful.
	VerifyCredentials(ctx context.Context, creds domain.ProviderCredentials) (*VerificationResult, error)

	// GetQuote retrieves shipping quotes from the provider.
	// Returns available service options with pricing.
	GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error)

	// CreateShipment creates a new shipment with the provider.
	// Returns the provider's shipment ID and tracking number.
	CreateShipment(ctx context.Context, req CreateShipmentRequest) (*CreateShipmentResponse, error)

	// CancelShipment cancels an existing shipment.
	// Returns the cancellation result.
	CancelShipment(ctx context.Context, req CancelShipmentRequest) (*CancelShipmentResponse, error)

	// GetLabel retrieves the shipping label for a shipment.
	// Returns the label data in the requested format.
	GetLabel(ctx context.Context, req GetLabelRequest) (*GetLabelResponse, error)

	// GetTracking retrieves tracking information for a shipment.
	// Returns the current status and tracking events.
	GetTracking(ctx context.Context, req GetTrackingRequest) (*GetTrackingResponse, error)

	// ParseTrackingWebhook parses an incoming tracking webhook payload.
	// Returns normalized tracking events.
	ParseTrackingWebhook(ctx context.Context, payload []byte) ([]TrackingWebhookEvent, error)

	// SchedulePickup schedules a pickup for shipments.
	// Returns the pickup confirmation.
	SchedulePickup(ctx context.Context, req SchedulePickupRequest) (*SchedulePickupResponse, error)

	// CancelPickup cancels a scheduled pickup.
	// Returns the cancellation result.
	CancelPickup(ctx context.Context, req CancelPickupRequest) (*CancelPickupResponse, error)

	// CreateReturn creates a return shipment.
	// Returns the return shipment details.
	CreateReturn(ctx context.Context, req CreateReturnRequest) (*CreateReturnResponse, error)

	// GetDropoffPoints retrieves available dropoff points.
	// Returns a list of dropoff locations.
	GetDropoffPoints(ctx context.Context, req GetDropoffPointsRequest) (*GetDropoffPointsResponse, error)
}

// VerificationResult contains the result of credential verification.
type VerificationResult struct {
	// Success indicates if the credentials are valid.
	Success bool `json:"success"`
	// Error contains the error message if verification failed.
	Error string `json:"error,omitempty"`
	// AccountInfo contains account details if verification succeeded.
	AccountInfo *AccountInfo `json:"account_info,omitempty"`
}

// AccountInfo contains provider account information.
type AccountInfo struct {
	// AccountNumber is the provider's account number.
	AccountNumber string `json:"account_number,omitempty"`
	// CompanyName is the registered company name.
	CompanyName string `json:"company_name,omitempty"`
	// ContractType is the contract type (e.g., "standard", "premium").
	ContractType string `json:"contract_type,omitempty"`
	// ExpiresAt is the contract expiration date.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// QuoteRequest contains the parameters for requesting a shipping quote.
type QuoteRequest struct {
	// Origin is the sender address.
	Origin domain.AddressSnapshot `json:"origin"`
	// Destination is the recipient address.
	Destination domain.AddressSnapshot `json:"destination"`
	// Parcel contains the parcel dimensions and weight.
	Parcel domain.Parcel `json:"parcel"`
	// CODAmountKurus is the COD amount in kuruş (optional).
	CODAmountKurus *int64 `json:"cod_amount_kurus,omitempty"`
	// DeclaredValueKurus is the declared value for insurance in kuruş (optional).
	DeclaredValueKurus *int64 `json:"declared_value_kurus,omitempty"`
	// ServiceCode is the specific service code to quote (optional).
	ServiceCode string `json:"service_code,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// QuoteResponse contains the quote results from a provider.
type QuoteResponse struct {
	// Offers contains the available shipping options.
	Offers []ProviderOffer `json:"offers"`
	// Errors contains any errors that occurred during quoting.
	Errors []ProviderError `json:"errors,omitempty"`
}

// ProviderOffer represents a shipping option from a provider.
type ProviderOffer struct {
	// ServiceCode is the provider's service code.
	ServiceCode string `json:"service_code"`
	// ServiceName is the human-readable service name.
	ServiceName string `json:"service_name"`
	// BasePriceKurus is the base shipping price in kuruş.
	BasePriceKurus int64 `json:"base_price_kurus"`
	// FuelSurchargeKurus is the fuel surcharge in kuruş.
	FuelSurchargeKurus int64 `json:"fuel_surcharge_kurus"`
	// CODFeeKurus is the COD handling fee in kuruş.
	CODFeeKurus int64 `json:"cod_fee_kurus"`
	// InsuranceFeeKurus is the insurance fee in kuruş.
	InsuranceFeeKurus int64 `json:"insurance_fee_kurus"`
	// VATKurus is the VAT amount in kuruş.
	VATKurus int64 `json:"vat_kurus"`
	// TotalPriceKurus is the total price including all fees and VAT.
	TotalPriceKurus int64 `json:"total_price_kurus"`
	// EstimatedDaysMin is the minimum delivery days.
	EstimatedDaysMin int `json:"estimated_days_min"`
	// EstimatedDaysMax is the maximum delivery days.
	EstimatedDaysMax int `json:"estimated_days_max"`
	// ProviderQuoteID is the provider's quote reference (if any).
	ProviderQuoteID string `json:"provider_quote_id,omitempty"`
	// ExpiresAt is when the quote expires.
	ExpiresAt time.Time `json:"expires_at"`
}

// ToPriceBreakdown converts the offer to a domain PriceBreakdown.
func (o *ProviderOffer) ToPriceBreakdown() domain.PriceBreakdown {
	return domain.PriceBreakdown{
		BasePriceKurus:     o.BasePriceKurus,
		FuelSurchargeKurus: o.FuelSurchargeKurus,
		CODFeeKurus:        o.CODFeeKurus,
		InsuranceFeeKurus:  o.InsuranceFeeKurus,
		OtherFeesKurus:     0,
		SubtotalKurus:      o.BasePriceKurus + o.FuelSurchargeKurus + o.CODFeeKurus + o.InsuranceFeeKurus,
		VATKurus:           o.VATKurus,
		TotalPriceKurus:    o.TotalPriceKurus,
	}
}

// CreateShipmentRequest contains the parameters for creating a shipment.
type CreateShipmentRequest struct {
	// Origin is the sender address.
	Origin domain.AddressSnapshot `json:"origin"`
	// Destination is the recipient address.
	Destination domain.AddressSnapshot `json:"destination"`
	// Parcel contains the parcel dimensions and weight.
	Parcel domain.Parcel `json:"parcel"`
	// ServiceCode is the service code to use.
	ServiceCode string `json:"service_code"`
	// CODAmountKurus is the COD amount in kuruş (optional).
	CODAmountKurus *int64 `json:"cod_amount_kurus,omitempty"`
	// DeclaredValueKurus is the declared value for insurance in kuruş (optional).
	DeclaredValueKurus *int64 `json:"declared_value_kurus,omitempty"`
	// Reference is the customer reference number (optional).
	Reference string `json:"reference,omitempty"`
	// LabelFormat is the desired label format.
	LabelFormat domain.LabelFormat `json:"label_format"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
	// ProviderQuoteID is the provider's quote reference (if any).
	ProviderQuoteID string `json:"provider_quote_id,omitempty"`
}

// CreateShipmentResponse contains the result of creating a shipment.
type CreateShipmentResponse struct {
	// ProviderShipmentID is the provider's internal shipment ID.
	ProviderShipmentID string `json:"provider_shipment_id"`
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// Barcode is the barcode for the label.
	Barcode string `json:"barcode,omitempty"`
	// Label contains the shipping label data.
	Label *LabelData `json:"label,omitempty"`
}

// LabelData contains shipping label information.
type LabelData struct {
	// Format is the label format (pdf, zpl, png).
	Format domain.LabelFormat `json:"format"`
	// Content is the raw label content.
	Content []byte `json:"content"`
	// ContentType is the MIME type.
	ContentType string `json:"content_type"`
}

// CancelShipmentRequest contains the parameters for cancelling a shipment.
type CancelShipmentRequest struct {
	// ProviderShipmentID is the provider's shipment ID.
	ProviderShipmentID string `json:"provider_shipment_id"`
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// Reason is the cancellation reason.
	Reason string `json:"reason,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// CancelShipmentResponse contains the result of cancelling a shipment.
type CancelShipmentResponse struct {
	// Success indicates if the cancellation was successful.
	Success bool `json:"success"`
	// Message contains any additional information.
	Message string `json:"message,omitempty"`
	// RefundAmountKurus is the refund amount in kuruş (if applicable).
	RefundAmountKurus *int64 `json:"refund_amount_kurus,omitempty"`
}

// GetLabelRequest contains the parameters for retrieving a label.
type GetLabelRequest struct {
	// ProviderShipmentID is the provider's shipment ID.
	ProviderShipmentID string `json:"provider_shipment_id"`
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// Format is the desired label format.
	Format domain.LabelFormat `json:"format"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// GetLabelResponse contains the label data.
type GetLabelResponse struct {
	// Label contains the shipping label data.
	Label LabelData `json:"label"`
}

// GetTrackingRequest contains the parameters for retrieving tracking info.
type GetTrackingRequest struct {
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// ProviderShipmentID is the provider's shipment ID (optional).
	ProviderShipmentID string `json:"provider_shipment_id,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// GetTrackingResponse contains the tracking information.
type GetTrackingResponse struct {
	// CurrentStatus is the current normalized status.
	CurrentStatus domain.TrackingStatus `json:"current_status"`
	// Events contains the tracking events.
	Events []TrackingEventData `json:"events"`
	// EstimatedDelivery is the estimated delivery date (if available).
	EstimatedDelivery *time.Time `json:"estimated_delivery,omitempty"`
}

// TrackingEventData represents a single tracking event from a provider.
type TrackingEventData struct {
	// StatusCode is the normalized status code.
	StatusCode domain.TrackingStatus `json:"status_code"`
	// StatusDescription is the human-readable description.
	StatusDescription string `json:"status_description"`
	// LocationCity is the event location city.
	LocationCity string `json:"location_city,omitempty"`
	// LocationDetail is the detailed location.
	LocationDetail string `json:"location_detail,omitempty"`
	// OccurredAt is when the event occurred.
	OccurredAt time.Time `json:"occurred_at"`
	// RawStatusCode is the provider's original status code.
	RawStatusCode string `json:"raw_status_code,omitempty"`
	// RawData is the full provider response.
	RawData json.RawMessage `json:"raw_data,omitempty"`
}

// ToTrackingEvent converts to a domain TrackingEvent.
func (e *TrackingEventData) ToTrackingEvent(shipmentID uuid.UUID, providerCode domain.ProviderCode, source domain.TrackingEventSource) *domain.TrackingEvent {
	event := domain.NewTrackingEvent(
		shipmentID,
		providerCode,
		e.StatusCode,
		e.StatusDescription,
		e.OccurredAt,
		source,
	)
	event.SetLocation(e.LocationCity, e.LocationDetail)
	event.SetRawData(e.RawStatusCode, e.RawData)
	return event
}

// TrackingWebhookEvent represents a tracking event from a webhook.
type TrackingWebhookEvent struct {
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// ProviderShipmentID is the provider's shipment ID (if available).
	ProviderShipmentID string `json:"provider_shipment_id,omitempty"`
	// Events contains the tracking events.
	Events []TrackingEventData `json:"events"`
}

// SchedulePickupRequest contains the parameters for scheduling a pickup.
type SchedulePickupRequest struct {
	// Address is the pickup address.
	Address domain.AddressSnapshot `json:"address"`
	// ScheduledDate is the requested pickup date.
	ScheduledDate time.Time `json:"scheduled_date"`
	// TimeWindowStart is the start of the pickup time window.
	TimeWindowStart string `json:"time_window_start,omitempty"`
	// TimeWindowEnd is the end of the pickup time window.
	TimeWindowEnd string `json:"time_window_end,omitempty"`
	// ShipmentIDs are the provider shipment IDs to pick up.
	ShipmentIDs []string `json:"shipment_ids"`
	// TrackingNumbers are the tracking numbers to pick up.
	TrackingNumbers []string `json:"tracking_numbers"`
	// ParcelCount is the number of parcels to pick up.
	ParcelCount int `json:"parcel_count"`
	// TotalWeightGrams is the total weight in grams.
	TotalWeightGrams int `json:"total_weight_grams"`
	// Instructions are special pickup instructions.
	Instructions string `json:"instructions,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// SchedulePickupResponse contains the result of scheduling a pickup.
type SchedulePickupResponse struct {
	// PickupID is the provider's pickup confirmation ID.
	PickupID string `json:"pickup_id"`
	// ConfirmedDate is the confirmed pickup date.
	ConfirmedDate time.Time `json:"confirmed_date"`
	// TimeWindowStart is the confirmed start of the pickup time window.
	TimeWindowStart string `json:"time_window_start,omitempty"`
	// TimeWindowEnd is the confirmed end of the pickup time window.
	TimeWindowEnd string `json:"time_window_end,omitempty"`
	// Message contains any additional information.
	Message string `json:"message,omitempty"`
}

// CancelPickupRequest contains the parameters for cancelling a pickup.
type CancelPickupRequest struct {
	// PickupID is the provider's pickup confirmation ID.
	PickupID string `json:"pickup_id"`
	// Reason is the cancellation reason.
	Reason string `json:"reason,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// CancelPickupResponse contains the result of cancelling a pickup.
type CancelPickupResponse struct {
	// Success indicates if the cancellation was successful.
	Success bool `json:"success"`
	// Message contains any additional information.
	Message string `json:"message,omitempty"`
}

// CreateReturnRequest contains the parameters for creating a return shipment.
type CreateReturnRequest struct {
	// OriginalTrackingNumber is the original shipment's tracking number.
	OriginalTrackingNumber string `json:"original_tracking_number"`
	// OriginalProviderShipmentID is the original provider shipment ID.
	OriginalProviderShipmentID string `json:"original_provider_shipment_id,omitempty"`
	// Origin is the return origin (original destination).
	Origin domain.AddressSnapshot `json:"origin"`
	// Destination is the return destination (original origin).
	Destination domain.AddressSnapshot `json:"destination"`
	// Parcel contains the parcel dimensions and weight.
	Parcel domain.Parcel `json:"parcel"`
	// ServiceCode is the service code to use.
	ServiceCode string `json:"service_code"`
	// Reference is the customer reference number (optional).
	Reference string `json:"reference,omitempty"`
	// LabelFormat is the desired label format.
	LabelFormat domain.LabelFormat `json:"label_format"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// CreateReturnResponse contains the result of creating a return shipment.
type CreateReturnResponse struct {
	// ProviderShipmentID is the provider's internal shipment ID.
	ProviderShipmentID string `json:"provider_shipment_id"`
	// TrackingNumber is the tracking number.
	TrackingNumber string `json:"tracking_number"`
	// Barcode is the barcode for the label.
	Barcode string `json:"barcode,omitempty"`
	// Label contains the shipping label data.
	Label *LabelData `json:"label,omitempty"`
}

// GetDropoffPointsRequest contains the parameters for retrieving dropoff points.
type GetDropoffPointsRequest struct {
	// CityCode is the city code to filter by.
	CityCode string `json:"city_code,omitempty"`
	// DistrictCode is the district code to filter by.
	DistrictCode string `json:"district_code,omitempty"`
	// PostalCode is the postal code to filter by.
	PostalCode string `json:"postal_code,omitempty"`
	// Latitude is the center latitude for proximity search.
	Latitude *float64 `json:"latitude,omitempty"`
	// Longitude is the center longitude for proximity search.
	Longitude *float64 `json:"longitude,omitempty"`
	// RadiusKm is the search radius in kilometers.
	RadiusKm *float64 `json:"radius_km,omitempty"`
	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`
	// Credentials are the provider credentials (for BYOA).
	Credentials *domain.ProviderCredentials `json:"credentials,omitempty"`
}

// GetDropoffPointsResponse contains the dropoff points.
type GetDropoffPointsResponse struct {
	// Points contains the dropoff points.
	Points []DropoffPoint `json:"points"`
}

// DropoffPoint represents a carrier dropoff location.
type DropoffPoint struct {
	// ID is the provider's point ID.
	ID string `json:"id"`
	// Name is the point name.
	Name string `json:"name"`
	// Address is the full address.
	Address string `json:"address"`
	// CityCode is the city code.
	CityCode string `json:"city_code"`
	// CityName is the city name.
	CityName string `json:"city_name"`
	// DistrictCode is the district code.
	DistrictCode string `json:"district_code"`
	// DistrictName is the district name.
	DistrictName string `json:"district_name"`
	// PostalCode is the postal code.
	PostalCode string `json:"postal_code,omitempty"`
	// Latitude is the GPS latitude.
	Latitude *float64 `json:"latitude,omitempty"`
	// Longitude is the GPS longitude.
	Longitude *float64 `json:"longitude,omitempty"`
	// Phone is the contact phone.
	Phone string `json:"phone,omitempty"`
	// WorkingHours describes the working hours.
	WorkingHours string `json:"working_hours,omitempty"`
	// IsActive indicates if the point is currently active.
	IsActive bool `json:"is_active"`
}

// BaseAdapter provides common functionality for provider adapters.
// Concrete adapters should embed this struct.
type BaseAdapter struct {
	providerCode domain.ProviderCode
	apiType      domain.ProviderAPIType
}

// NewBaseAdapter creates a new BaseAdapter.
func NewBaseAdapter(providerCode domain.ProviderCode, apiType domain.ProviderAPIType) BaseAdapter {
	return BaseAdapter{
		providerCode: providerCode,
		apiType:      apiType,
	}
}

// ProviderCode returns the provider code.
func (a *BaseAdapter) ProviderCode() domain.ProviderCode {
	return a.providerCode
}

// APIType returns the API type.
func (a *BaseAdapter) APIType() domain.ProviderAPIType {
	return a.apiType
}

// UnsupportedOperation returns an error for unsupported operations.
func (a *BaseAdapter) UnsupportedOperation(operation string) error {
	return &ProviderError{
		Code:    "UNSUPPORTED_OPERATION",
		Message: fmt.Sprintf("operation %s is not supported by provider %s", operation, a.providerCode),
	}
}

// AdapterConfig contains common configuration for adapters.
type AdapterConfig struct {
	// BaseURL is the API base URL.
	BaseURL string `json:"base_url"`
	// SandboxURL is the sandbox API URL.
	SandboxURL string `json:"sandbox_url,omitempty"`
	// Timeout is the request timeout.
	Timeout time.Duration `json:"timeout"`
	// UseSandbox indicates whether to use sandbox mode.
	UseSandbox bool `json:"use_sandbox"`
	// MaxRetries is the maximum number of retries.
	MaxRetries int `json:"max_retries"`
	// RetryBaseDelay is the base delay between retries.
	RetryBaseDelay time.Duration `json:"retry_base_delay"`
	// RetryMaxDelay is the maximum delay between retries.
	RetryMaxDelay time.Duration `json:"retry_max_delay"`
}

// DefaultAdapterConfig returns the default adapter configuration.
func DefaultAdapterConfig() AdapterConfig {
	return AdapterConfig{
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryBaseDelay: 1 * time.Second,
		RetryMaxDelay:  10 * time.Second,
	}
}

// GetEffectiveURL returns the effective URL based on sandbox mode.
func (c *AdapterConfig) GetEffectiveURL() string {
	if c.UseSandbox && c.SandboxURL != "" {
		return c.SandboxURL
	}
	return c.BaseURL
}
