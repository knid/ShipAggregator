// Package surat implements the provider adapter for Sürat Kargo.
// Sürat Kargo uses a SOAP-based API for shipment operations.
package surat

import (
	"encoding/xml"
	"time"
)

// SOAP request/response types for Sürat Kargo API.

// Credentials represents Sürat API credentials.
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// VerifyCredentialsRequest is the SOAP request for credential verification.
type VerifyCredentialsRequest struct {
	XMLName  xml.Name `xml:"urn:SuratKargo VerifyCredentials"`
	UserName string   `xml:"UserName"`
	Password string   `xml:"Password"`
}

// VerifyCredentialsResponse is the SOAP response for credential verification.
type VerifyCredentialsResponse struct {
	XMLName xml.Name                    `xml:"VerifyCredentialsResponse"`
	Result  VerifyCredentialsResultData `xml:"VerifyCredentialsResult"`
}

// VerifyCredentialsResultData contains the verification result.
type VerifyCredentialsResultData struct {
	Success       bool   `xml:"Success"`
	ErrorCode     string `xml:"ErrorCode,omitempty"`
	ErrorMessage  string `xml:"ErrorMessage,omitempty"`
	AccountNumber string `xml:"AccountNumber,omitempty"`
	CompanyName   string `xml:"CompanyName,omitempty"`
	ContractType  string `xml:"ContractType,omitempty"`
}

// GetQuoteRequest is the SOAP request for getting a shipping quote.
type GetQuoteRequest struct {
	XMLName          xml.Name `xml:"urn:SuratKargo GetQuote"`
	UserName         string   `xml:"UserName"`
	Password         string   `xml:"Password"`
	SenderCityCode   int      `xml:"SenderCityCode"`
	SenderTownCode   int      `xml:"SenderTownCode"`
	ReceiverCityCode int      `xml:"ReceiverCityCode"`
	ReceiverTownCode int      `xml:"ReceiverTownCode"`
	Weight           float64  `xml:"Weight"` // kg
	Desi             float64  `xml:"Desi"`
	CODAmount        float64  `xml:"CODAmount,omitempty"`
	DeclaredValue    float64  `xml:"DeclaredValue,omitempty"`
	ServiceType      string   `xml:"ServiceType,omitempty"` // "standard" or "express"
}

// GetQuoteResponse is the SOAP response for getting a shipping quote.
type GetQuoteResponse struct {
	XMLName xml.Name           `xml:"GetQuoteResponse"`
	Result  GetQuoteResultData `xml:"GetQuoteResult"`
}

// GetQuoteResultData contains the quote result.
type GetQuoteResultData struct {
	Success      bool         `xml:"Success"`
	ErrorCode    string       `xml:"ErrorCode,omitempty"`
	ErrorMessage string       `xml:"ErrorMessage,omitempty"`
	Quotes       []QuoteOffer `xml:"Quotes>Quote,omitempty"`
}

// QuoteOffer represents a single quote offer from Sürat.
type QuoteOffer struct {
	ServiceCode      string  `xml:"ServiceCode"`
	ServiceName      string  `xml:"ServiceName"`
	BasePrice        float64 `xml:"BasePrice"`
	FuelSurcharge    float64 `xml:"FuelSurcharge"`
	CODFee           float64 `xml:"CODFee"`
	InsuranceFee     float64 `xml:"InsuranceFee"`
	VAT              float64 `xml:"VAT"`
	TotalPrice       float64 `xml:"TotalPrice"`
	EstimatedDaysMin int     `xml:"EstimatedDaysMin"`
	EstimatedDaysMax int     `xml:"EstimatedDaysMax"`
	QuoteID          string  `xml:"QuoteID,omitempty"`
	ValidUntil       string  `xml:"ValidUntil,omitempty"`
}

// CreateShipmentRequest is the SOAP request for creating a shipment.
type CreateShipmentRequest struct {
	XMLName  xml.Name        `xml:"urn:SuratKargo CreateShipment"`
	UserName string          `xml:"UserName"`
	Password string          `xml:"Password"`
	Shipment ShipmentDetails `xml:"Shipment"`
}

// ShipmentDetails contains the shipment information.
type ShipmentDetails struct {
	ServiceType         string         `xml:"ServiceType"`
	QuoteID             string         `xml:"QuoteID,omitempty"`
	Reference           string         `xml:"Reference,omitempty"`
	Sender              AddressDetails `xml:"Sender"`
	Receiver            AddressDetails `xml:"Receiver"`
	Parcel              ParcelDetails  `xml:"Parcel"`
	CODAmount           float64        `xml:"CODAmount,omitempty"`
	DeclaredValue       float64        `xml:"DeclaredValue,omitempty"`
	LabelFormat         string         `xml:"LabelFormat"`            // "PDF" only
	PaymentType         string         `xml:"PaymentType"`            // "SENDER" or "RECEIVER"
	DeliveryType        string         `xml:"DeliveryType,omitempty"` // "DOOR" or "BRANCH"
	SpecialInstructions string         `xml:"SpecialInstructions,omitempty"`
}

// AddressDetails contains address information.
type AddressDetails struct {
	Name        string `xml:"Name"`
	CompanyName string `xml:"CompanyName,omitempty"`
	Phone       string `xml:"Phone"`
	Email       string `xml:"Email,omitempty"`
	Address     string `xml:"Address"`
	CityCode    int    `xml:"CityCode"`
	TownCode    int    `xml:"TownCode"`
	PostalCode  string `xml:"PostalCode,omitempty"`
}

// ParcelDetails contains parcel information.
type ParcelDetails struct {
	PieceCount  int     `xml:"PieceCount"`
	Weight      float64 `xml:"Weight"` // kg
	Desi        float64 `xml:"Desi"`
	Length      float64 `xml:"Length,omitempty"` // cm
	Width       float64 `xml:"Width,omitempty"`  // cm
	Height      float64 `xml:"Height,omitempty"` // cm
	Description string  `xml:"Description,omitempty"`
}

// CreateShipmentResponse is the SOAP response for creating a shipment.
type CreateShipmentResponse struct {
	XMLName xml.Name                 `xml:"CreateShipmentResponse"`
	Result  CreateShipmentResultData `xml:"CreateShipmentResult"`
}

// CreateShipmentResultData contains the shipment creation result.
type CreateShipmentResultData struct {
	Success        bool   `xml:"Success"`
	ErrorCode      string `xml:"ErrorCode,omitempty"`
	ErrorMessage   string `xml:"ErrorMessage,omitempty"`
	ShipmentID     string `xml:"ShipmentID,omitempty"`
	TrackingNumber string `xml:"TrackingNumber,omitempty"`
	Barcode        string `xml:"Barcode,omitempty"`
	LabelData      string `xml:"LabelData,omitempty"` // Base64 encoded
	LabelFormat    string `xml:"LabelFormat,omitempty"`
}

// CancelShipmentRequest is the SOAP request for cancelling a shipment.
type CancelShipmentRequest struct {
	XMLName        xml.Name `xml:"urn:SuratKargo CancelShipment"`
	UserName       string   `xml:"UserName"`
	Password       string   `xml:"Password"`
	ShipmentID     string   `xml:"ShipmentID,omitempty"`
	TrackingNumber string   `xml:"TrackingNumber,omitempty"`
	Reason         string   `xml:"Reason,omitempty"`
}

// CancelShipmentResponse is the SOAP response for cancelling a shipment.
type CancelShipmentResponse struct {
	XMLName xml.Name                 `xml:"CancelShipmentResponse"`
	Result  CancelShipmentResultData `xml:"CancelShipmentResult"`
}

// CancelShipmentResultData contains the cancellation result.
type CancelShipmentResultData struct {
	Success      bool    `xml:"Success"`
	ErrorCode    string  `xml:"ErrorCode,omitempty"`
	ErrorMessage string  `xml:"ErrorMessage,omitempty"`
	RefundAmount float64 `xml:"RefundAmount,omitempty"`
}

// GetLabelRequest is the SOAP request for getting a shipping label.
type GetLabelRequest struct {
	XMLName        xml.Name `xml:"urn:SuratKargo GetLabel"`
	UserName       string   `xml:"UserName"`
	Password       string   `xml:"Password"`
	ShipmentID     string   `xml:"ShipmentID,omitempty"`
	TrackingNumber string   `xml:"TrackingNumber,omitempty"`
	LabelFormat    string   `xml:"LabelFormat"` // "PDF" only
}

// GetLabelResponse is the SOAP response for getting a shipping label.
type GetLabelResponse struct {
	XMLName xml.Name           `xml:"GetLabelResponse"`
	Result  GetLabelResultData `xml:"GetLabelResult"`
}

// GetLabelResultData contains the label data.
type GetLabelResultData struct {
	Success      bool   `xml:"Success"`
	ErrorCode    string `xml:"ErrorCode,omitempty"`
	ErrorMessage string `xml:"ErrorMessage,omitempty"`
	LabelData    string `xml:"LabelData,omitempty"` // Base64 encoded
	LabelFormat  string `xml:"LabelFormat,omitempty"`
	ContentType  string `xml:"ContentType,omitempty"`
}

// GetTrackingRequest is the SOAP request for getting tracking information.
type GetTrackingRequest struct {
	XMLName        xml.Name `xml:"urn:SuratKargo GetTracking"`
	UserName       string   `xml:"UserName"`
	Password       string   `xml:"Password"`
	TrackingNumber string   `xml:"TrackingNumber"`
	ShipmentID     string   `xml:"ShipmentID,omitempty"`
}

// GetTrackingResponse is the SOAP response for getting tracking information.
type GetTrackingResponse struct {
	XMLName xml.Name              `xml:"GetTrackingResponse"`
	Result  GetTrackingResultData `xml:"GetTrackingResult"`
}

// GetTrackingResultData contains the tracking result.
type GetTrackingResultData struct {
	Success           bool            `xml:"Success"`
	ErrorCode         string          `xml:"ErrorCode,omitempty"`
	ErrorMessage      string          `xml:"ErrorMessage,omitempty"`
	CurrentStatus     string          `xml:"CurrentStatus,omitempty"`
	EstimatedDelivery string          `xml:"EstimatedDelivery,omitempty"`
	Events            []TrackingEvent `xml:"Events>Event,omitempty"`
}

// TrackingEvent represents a single tracking event.
type TrackingEvent struct {
	StatusCode        string `xml:"StatusCode"`
	StatusDescription string `xml:"StatusDescription"`
	LocationCity      string `xml:"LocationCity,omitempty"`
	LocationDetail    string `xml:"LocationDetail,omitempty"`
	OccurredAt        string `xml:"OccurredAt"` // ISO 8601 format
}

// SchedulePickupRequest is the SOAP request for scheduling a pickup.
type SchedulePickupRequest struct {
	XMLName         xml.Name       `xml:"urn:SuratKargo SchedulePickup"`
	UserName        string         `xml:"UserName"`
	Password        string         `xml:"Password"`
	PickupAddress   AddressDetails `xml:"PickupAddress"`
	ScheduledDate   string         `xml:"ScheduledDate"`             // YYYY-MM-DD
	TimeWindowStart string         `xml:"TimeWindowStart,omitempty"` // HH:MM
	TimeWindowEnd   string         `xml:"TimeWindowEnd,omitempty"`   // HH:MM
	ParcelCount     int            `xml:"ParcelCount"`
	TotalWeight     float64        `xml:"TotalWeight"`               // kg
	TrackingNumbers string         `xml:"TrackingNumbers,omitempty"` // Comma-separated
	Instructions    string         `xml:"Instructions,omitempty"`
}

// SchedulePickupResponse is the SOAP response for scheduling a pickup.
type SchedulePickupResponse struct {
	XMLName xml.Name                 `xml:"SchedulePickupResponse"`
	Result  SchedulePickupResultData `xml:"SchedulePickupResult"`
}

// SchedulePickupResultData contains the pickup scheduling result.
type SchedulePickupResultData struct {
	Success         bool   `xml:"Success"`
	ErrorCode       string `xml:"ErrorCode,omitempty"`
	ErrorMessage    string `xml:"ErrorMessage,omitempty"`
	PickupID        string `xml:"PickupID,omitempty"`
	ConfirmedDate   string `xml:"ConfirmedDate,omitempty"`
	TimeWindowStart string `xml:"TimeWindowStart,omitempty"`
	TimeWindowEnd   string `xml:"TimeWindowEnd,omitempty"`
}

// CancelPickupRequest is the SOAP request for cancelling a pickup.
type CancelPickupRequest struct {
	XMLName  xml.Name `xml:"urn:SuratKargo CancelPickup"`
	UserName string   `xml:"UserName"`
	Password string   `xml:"Password"`
	PickupID string   `xml:"PickupID"`
	Reason   string   `xml:"Reason,omitempty"`
}

// CancelPickupResponse is the SOAP response for cancelling a pickup.
type CancelPickupResponse struct {
	XMLName xml.Name               `xml:"CancelPickupResponse"`
	Result  CancelPickupResultData `xml:"CancelPickupResult"`
}

// CancelPickupResultData contains the pickup cancellation result.
type CancelPickupResultData struct {
	Success      bool   `xml:"Success"`
	ErrorCode    string `xml:"ErrorCode,omitempty"`
	ErrorMessage string `xml:"ErrorMessage,omitempty"`
}

// CreateReturnRequest is the SOAP request for creating a return shipment.
type CreateReturnRequest struct {
	XMLName                xml.Name        `xml:"urn:SuratKargo CreateReturn"`
	UserName               string          `xml:"UserName"`
	Password               string          `xml:"Password"`
	OriginalTrackingNumber string          `xml:"OriginalTrackingNumber"`
	OriginalShipmentID     string          `xml:"OriginalShipmentID,omitempty"`
	Shipment               ShipmentDetails `xml:"Shipment"`
}

// CreateReturnResponse is the SOAP response for creating a return shipment.
type CreateReturnResponse struct {
	XMLName xml.Name               `xml:"CreateReturnResponse"`
	Result  CreateReturnResultData `xml:"CreateReturnResult"`
}

// CreateReturnResultData contains the return creation result.
type CreateReturnResultData struct {
	Success        bool   `xml:"Success"`
	ErrorCode      string `xml:"ErrorCode,omitempty"`
	ErrorMessage   string `xml:"ErrorMessage,omitempty"`
	ShipmentID     string `xml:"ShipmentID,omitempty"`
	TrackingNumber string `xml:"TrackingNumber,omitempty"`
	Barcode        string `xml:"Barcode,omitempty"`
	LabelData      string `xml:"LabelData,omitempty"`
	LabelFormat    string `xml:"LabelFormat,omitempty"`
}

// GetDropoffPointsRequest is the SOAP request for getting dropoff points.
type GetDropoffPointsRequest struct {
	XMLName    xml.Name `xml:"urn:SuratKargo GetDropoffPoints"`
	UserName   string   `xml:"UserName"`
	Password   string   `xml:"Password"`
	CityCode   int      `xml:"CityCode,omitempty"`
	TownCode   int      `xml:"TownCode,omitempty"`
	PostalCode string   `xml:"PostalCode,omitempty"`
	Limit      int      `xml:"Limit,omitempty"`
}

// GetDropoffPointsResponse is the SOAP response for getting dropoff points.
type GetDropoffPointsResponse struct {
	XMLName xml.Name                   `xml:"GetDropoffPointsResponse"`
	Result  GetDropoffPointsResultData `xml:"GetDropoffPointsResult"`
}

// GetDropoffPointsResultData contains the dropoff points result.
type GetDropoffPointsResultData struct {
	Success      bool           `xml:"Success"`
	ErrorCode    string         `xml:"ErrorCode,omitempty"`
	ErrorMessage string         `xml:"ErrorMessage,omitempty"`
	Points       []DropoffPoint `xml:"Points>Point,omitempty"`
}

// DropoffPoint represents a dropoff location.
type DropoffPoint struct {
	ID           string  `xml:"ID"`
	Name         string  `xml:"Name"`
	Address      string  `xml:"Address"`
	CityCode     int     `xml:"CityCode"`
	CityName     string  `xml:"CityName"`
	TownCode     int     `xml:"TownCode"`
	TownName     string  `xml:"TownName"`
	PostalCode   string  `xml:"PostalCode,omitempty"`
	Latitude     float64 `xml:"Latitude,omitempty"`
	Longitude    float64 `xml:"Longitude,omitempty"`
	Phone        string  `xml:"Phone,omitempty"`
	WorkingHours string  `xml:"WorkingHours,omitempty"`
	IsActive     bool    `xml:"IsActive"`
}

// TrackingWebhookPayload represents the webhook payload from Sürat.
// Note: Sürat does not support tracking webhooks, only polling.
// This type is kept for interface compatibility.
type TrackingWebhookPayload struct {
	TrackingNumber string                 `json:"tracking_number"`
	ShipmentID     string                 `json:"shipment_id,omitempty"`
	Events         []TrackingWebhookEvent `json:"events"`
	Signature      string                 `json:"signature,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
}

// TrackingWebhookEvent represents a tracking event in a webhook.
type TrackingWebhookEvent struct {
	StatusCode        string    `json:"status_code"`
	StatusDescription string    `json:"status_description"`
	LocationCity      string    `json:"location_city,omitempty"`
	LocationDetail    string    `json:"location_detail,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

// ServiceType constants for Sürat.
const (
	ServiceTypeStandard = "standard"
	ServiceTypeExpress  = "express"
)

// LabelFormatType constants for Sürat.
// Note: Sürat only supports PDF format.
const (
	LabelFormatPDF = "PDF"
)

// PaymentType constants for Sürat.
const (
	PaymentTypeSender   = "SENDER"
	PaymentTypeReceiver = "RECEIVER"
)

// DeliveryType constants for Sürat.
const (
	DeliveryTypeDoor   = "DOOR"
	DeliveryTypeBranch = "BRANCH"
)
