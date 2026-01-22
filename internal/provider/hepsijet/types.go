// Package hepsijet implements the provider adapter for Hepsijet.
// Hepsijet uses a REST-based API for shipment operations.
package hepsijet

import (
	"time"
)

// REST request/response types for Hepsijet API.

// Credentials represents Hepsijet API credentials.
type Credentials struct {
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	CustomerCode string `json:"customer_code,omitempty"`
}

// AuthRequest is the request for authentication.
type AuthRequest struct {
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret"`
}

// AuthResponse is the response for authentication.
type AuthResponse struct {
	Success     bool           `json:"success"`
	AccessToken string         `json:"accessToken,omitempty"`
	ExpiresIn   int            `json:"expiresIn,omitempty"` // seconds
	TokenType   string         `json:"tokenType,omitempty"`
	Error       *ErrorResponse `json:"error,omitempty"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// VerifyCredentialsResponse is the response for credential verification.
type VerifyCredentialsResponse struct {
	Success       bool           `json:"success"`
	AccountNumber string         `json:"accountNumber,omitempty"`
	CompanyName   string         `json:"companyName,omitempty"`
	ContractType  string         `json:"contractType,omitempty"`
	Error         *ErrorResponse `json:"error,omitempty"`
}

// GetQuoteRequest is the request for getting a shipping quote.
type GetQuoteRequest struct {
	SenderCityCode       string  `json:"senderCityCode"`
	SenderDistrictCode   string  `json:"senderDistrictCode"`
	ReceiverCityCode     string  `json:"receiverCityCode"`
	ReceiverDistrictCode string  `json:"receiverDistrictCode"`
	Weight               float64 `json:"weight"` // kg
	Desi                 float64 `json:"desi"`
	CODAmount            float64 `json:"codAmount,omitempty"`
	DeclaredValue        float64 `json:"declaredValue,omitempty"`
	ServiceType          string  `json:"serviceType,omitempty"`
}

// GetQuoteResponse is the response for getting a shipping quote.
type GetQuoteResponse struct {
	Success bool           `json:"success"`
	Quotes  []QuoteOffer   `json:"quotes,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// QuoteOffer represents a single quote offer from Hepsijet.
type QuoteOffer struct {
	ServiceCode      string  `json:"serviceCode"`
	ServiceName      string  `json:"serviceName"`
	BasePrice        float64 `json:"basePrice"`
	FuelSurcharge    float64 `json:"fuelSurcharge"`
	CODFee           float64 `json:"codFee"`
	InsuranceFee     float64 `json:"insuranceFee"`
	VAT              float64 `json:"vat"`
	TotalPrice       float64 `json:"totalPrice"`
	EstimatedDaysMin int     `json:"estimatedDaysMin"`
	EstimatedDaysMax int     `json:"estimatedDaysMax"`
	QuoteID          string  `json:"quoteId,omitempty"`
	ValidUntil       string  `json:"validUntil,omitempty"`
}

// CreateShipmentRequest is the request for creating a shipment.
type CreateShipmentRequest struct {
	ServiceType         string         `json:"serviceType"`
	QuoteID             string         `json:"quoteId,omitempty"`
	Reference           string         `json:"reference,omitempty"`
	Sender              AddressDetails `json:"sender"`
	Receiver            AddressDetails `json:"receiver"`
	Parcel              ParcelDetails  `json:"parcel"`
	CODAmount           float64        `json:"codAmount,omitempty"`
	DeclaredValue       float64        `json:"declaredValue,omitempty"`
	LabelFormat         string         `json:"labelFormat"`            // "PDF" or "ZPL"
	PaymentType         string         `json:"paymentType"`            // "SENDER" or "RECEIVER"
	DeliveryType        string         `json:"deliveryType,omitempty"` // "DOOR" or "PICKUP_POINT"
	PickupPointID       string         `json:"pickupPointId,omitempty"`
	SpecialInstructions string         `json:"specialInstructions,omitempty"`
}

// AddressDetails contains address information.
type AddressDetails struct {
	Name         string  `json:"name"`
	CompanyName  string  `json:"companyName,omitempty"`
	Phone        string  `json:"phone"`
	Email        string  `json:"email,omitempty"`
	Address      string  `json:"address"`
	CityCode     string  `json:"cityCode"`
	DistrictCode string  `json:"districtCode"`
	PostalCode   string  `json:"postalCode,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
}

// ParcelDetails contains parcel information.
type ParcelDetails struct {
	PieceCount  int     `json:"pieceCount"`
	Weight      float64 `json:"weight"` // kg
	Desi        float64 `json:"desi"`
	Length      float64 `json:"length,omitempty"` // cm
	Width       float64 `json:"width,omitempty"`  // cm
	Height      float64 `json:"height,omitempty"` // cm
	Description string  `json:"description,omitempty"`
}

// CreateShipmentResponse is the response for creating a shipment.
type CreateShipmentResponse struct {
	Success        bool           `json:"success"`
	ShipmentID     string         `json:"shipmentId,omitempty"`
	TrackingNumber string         `json:"trackingNumber,omitempty"`
	Barcode        string         `json:"barcode,omitempty"`
	LabelData      string         `json:"labelData,omitempty"` // Base64 encoded
	LabelFormat    string         `json:"labelFormat,omitempty"`
	Error          *ErrorResponse `json:"error,omitempty"`
}

// CancelShipmentRequest is the request for cancelling a shipment.
type CancelShipmentRequest struct {
	ShipmentID     string `json:"shipmentId,omitempty"`
	TrackingNumber string `json:"trackingNumber,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// CancelShipmentResponse is the response for cancelling a shipment.
type CancelShipmentResponse struct {
	Success      bool           `json:"success"`
	RefundAmount float64        `json:"refundAmount,omitempty"`
	Error        *ErrorResponse `json:"error,omitempty"`
}

// GetLabelRequest is the request for getting a shipping label.
type GetLabelRequest struct {
	ShipmentID     string `json:"shipmentId,omitempty"`
	TrackingNumber string `json:"trackingNumber,omitempty"`
	LabelFormat    string `json:"labelFormat"` // "PDF" or "ZPL"
}

// GetLabelResponse is the response for getting a shipping label.
type GetLabelResponse struct {
	Success     bool           `json:"success"`
	LabelData   string         `json:"labelData,omitempty"` // Base64 encoded
	LabelFormat string         `json:"labelFormat,omitempty"`
	ContentType string         `json:"contentType,omitempty"`
	Error       *ErrorResponse `json:"error,omitempty"`
}

// GetTrackingRequest is the request for getting tracking information.
type GetTrackingRequest struct {
	TrackingNumber string `json:"trackingNumber"`
	ShipmentID     string `json:"shipmentId,omitempty"`
}

// GetTrackingResponse is the response for getting tracking information.
type GetTrackingResponse struct {
	Success           bool            `json:"success"`
	CurrentStatus     string          `json:"currentStatus,omitempty"`
	StatusCode        string          `json:"statusCode,omitempty"`
	EstimatedDelivery string          `json:"estimatedDelivery,omitempty"`
	Events            []TrackingEvent `json:"events,omitempty"`
	Error             *ErrorResponse  `json:"error,omitempty"`
}

// TrackingEvent represents a single tracking event.
type TrackingEvent struct {
	StatusCode        string `json:"statusCode"`
	StatusDescription string `json:"statusDescription"`
	LocationCity      string `json:"locationCity,omitempty"`
	LocationDetail    string `json:"locationDetail,omitempty"`
	OccurredAt        string `json:"occurredAt"` // ISO 8601 format
}

// SchedulePickupRequest is the request for scheduling a pickup.
type SchedulePickupRequest struct {
	PickupAddress   AddressDetails `json:"pickupAddress"`
	ScheduledDate   string         `json:"scheduledDate"`             // YYYY-MM-DD
	TimeWindowStart string         `json:"timeWindowStart,omitempty"` // HH:MM
	TimeWindowEnd   string         `json:"timeWindowEnd,omitempty"`   // HH:MM
	ParcelCount     int            `json:"parcelCount"`
	TotalWeight     float64        `json:"totalWeight"` // kg
	TrackingNumbers []string       `json:"trackingNumbers,omitempty"`
	Instructions    string         `json:"instructions,omitempty"`
}

// SchedulePickupResponse is the response for scheduling a pickup.
type SchedulePickupResponse struct {
	Success         bool           `json:"success"`
	PickupID        string         `json:"pickupId,omitempty"`
	ConfirmedDate   string         `json:"confirmedDate,omitempty"`
	TimeWindowStart string         `json:"timeWindowStart,omitempty"`
	TimeWindowEnd   string         `json:"timeWindowEnd,omitempty"`
	Error           *ErrorResponse `json:"error,omitempty"`
}

// CancelPickupRequest is the request for cancelling a pickup.
type CancelPickupRequest struct {
	PickupID string `json:"pickupId"`
	Reason   string `json:"reason,omitempty"`
}

// CancelPickupResponse is the response for cancelling a pickup.
type CancelPickupResponse struct {
	Success bool           `json:"success"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// CreateReturnRequest is the request for creating a return shipment.
type CreateReturnRequest struct {
	OriginalTrackingNumber string         `json:"originalTrackingNumber"`
	OriginalShipmentID     string         `json:"originalShipmentId,omitempty"`
	ServiceType            string         `json:"serviceType"`
	Reference              string         `json:"reference,omitempty"`
	Sender                 AddressDetails `json:"sender"`
	Receiver               AddressDetails `json:"receiver"`
	Parcel                 ParcelDetails  `json:"parcel"`
	LabelFormat            string         `json:"labelFormat"`
}

// CreateReturnResponse is the response for creating a return shipment.
type CreateReturnResponse struct {
	Success        bool           `json:"success"`
	ShipmentID     string         `json:"shipmentId,omitempty"`
	TrackingNumber string         `json:"trackingNumber,omitempty"`
	Barcode        string         `json:"barcode,omitempty"`
	LabelData      string         `json:"labelData,omitempty"`
	LabelFormat    string         `json:"labelFormat,omitempty"`
	Error          *ErrorResponse `json:"error,omitempty"`
}

// GetDropoffPointsRequest is the request for getting dropoff points.
type GetDropoffPointsRequest struct {
	CityCode     string  `json:"cityCode,omitempty"`
	DistrictCode string  `json:"districtCode,omitempty"`
	PostalCode   string  `json:"postalCode,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	RadiusKm     float64 `json:"radiusKm,omitempty"`
	Limit        int     `json:"limit,omitempty"`
}

// GetDropoffPointsResponse is the response for getting dropoff points.
type GetDropoffPointsResponse struct {
	Success bool           `json:"success"`
	Points  []DropoffPoint `json:"points,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// DropoffPoint represents a dropoff location.
type DropoffPoint struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	CityCode     string  `json:"cityCode"`
	CityName     string  `json:"cityName"`
	DistrictCode string  `json:"districtCode"`
	DistrictName string  `json:"districtName"`
	PostalCode   string  `json:"postalCode,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Phone        string  `json:"phone,omitempty"`
	WorkingHours string  `json:"workingHours,omitempty"`
	IsActive     bool    `json:"isActive"`
}

// TrackingWebhookPayload represents the webhook payload from Hepsijet.
type TrackingWebhookPayload struct {
	TrackingNumber string                 `json:"trackingNumber"`
	ShipmentID     string                 `json:"shipmentId,omitempty"`
	Events         []TrackingWebhookEvent `json:"events"`
	Signature      string                 `json:"signature,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// TrackingWebhookEvent represents a tracking event in a webhook.
type TrackingWebhookEvent struct {
	StatusCode        string    `json:"statusCode"`
	StatusDescription string    `json:"statusDescription"`
	LocationCity      string    `json:"locationCity,omitempty"`
	LocationDetail    string    `json:"locationDetail,omitempty"`
	OccurredAt        time.Time `json:"occurredAt"`
}

// ServiceType constants for Hepsijet.
const (
	ServiceTypeStandard = "STANDARD"
	ServiceTypeExpress  = "EXPRESS"
	ServiceTypeSameDay  = "SAME_DAY"
)

// LabelFormatType constants for Hepsijet.
const (
	LabelFormatPDF = "PDF"
	LabelFormatZPL = "ZPL"
)

// PaymentType constants for Hepsijet.
const (
	PaymentTypeSender   = "SENDER"
	PaymentTypeReceiver = "RECEIVER"
)

// DeliveryType constants for Hepsijet.
const (
	DeliveryTypeDoor        = "DOOR"
	DeliveryTypePickupPoint = "PICKUP_POINT"
)

// API endpoints.
const (
	EndpointAuth          = "/auth/token"
	EndpointVerify        = "/auth/verify"
	EndpointQuote         = "/quotes"
	EndpointShipments     = "/shipments"
	EndpointLabels        = "/labels"
	EndpointTracking      = "/tracking"
	EndpointPickups       = "/pickups"
	EndpointReturns       = "/returns"
	EndpointDropoffPoints = "/dropoff-points"
)
