// Package yurtici implements the provider adapter for Yurtiçi Kargo.
// Yurtiçi Kargo uses a SOAP-based API for shipment operations.
package yurtici

import (
	"encoding/xml"
	"time"
)

// SOAP request/response types for Yurtiçi Kargo API.

// Credentials represents Yurtiçi API credentials.
type Credentials struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	CustomerCode string `json:"customer_code"`
	Language     string `json:"language,omitempty"` // "TR" or "EN"
}

// VerifyCredentialsRequest is the SOAP request for credential verification.
type VerifyCredentialsRequest struct {
	XMLName      xml.Name `xml:"urn:YurticiKargo LoginCheck"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
}

// VerifyCredentialsResponse is the SOAP response for credential verification.
type VerifyCredentialsResponse struct {
	XMLName xml.Name                    `xml:"LoginCheckResponse"`
	Result  VerifyCredentialsResultData `xml:"LoginCheckResult"`
}

// VerifyCredentialsResultData contains the verification result.
type VerifyCredentialsResultData struct {
	Success       bool   `xml:"Success"`
	ErrorCode     string `xml:"ErrorCode,omitempty"`
	ErrorMessage  string `xml:"ErrorMessage,omitempty"`
	AccountNumber string `xml:"AccountNumber,omitempty"`
	CompanyName   string `xml:"CompanyName,omitempty"`
	ContractType  string `xml:"ContractType,omitempty"`
	SessionID     string `xml:"SessionID,omitempty"`
}

// GetQuoteRequest is the SOAP request for getting a shipping quote.
type GetQuoteRequest struct {
	XMLName          xml.Name `xml:"urn:YurticiKargo GetPrice"`
	UserName         string   `xml:"UserName"`
	Password         string   `xml:"Password"`
	CustomerCode     string   `xml:"CustomerCode"`
	Language         string   `xml:"Language,omitempty"`
	SenderCityCode   string   `xml:"SenderCityCode"`
	SenderTownCode   string   `xml:"SenderTownCode"`
	ReceiverCityCode string   `xml:"ReceiverCityCode"`
	ReceiverTownCode string   `xml:"ReceiverTownCode"`
	Weight           float64  `xml:"Weight"` // kg
	Desi             float64  `xml:"Desi"`
	CODAmount        float64  `xml:"CODAmount,omitempty"`
	DeclaredValue    float64  `xml:"DeclaredValue,omitempty"`
	CargoType        string   `xml:"CargoType,omitempty"` // "DOCUMENT" or "PARCEL"
	ProductCode      string   `xml:"ProductCode,omitempty"`
}

// GetQuoteResponse is the SOAP response for getting a shipping quote.
type GetQuoteResponse struct {
	XMLName xml.Name           `xml:"GetPriceResponse"`
	Result  GetQuoteResultData `xml:"GetPriceResult"`
}

// GetQuoteResultData contains the quote result.
type GetQuoteResultData struct {
	Success      bool         `xml:"Success"`
	ErrorCode    string       `xml:"ErrorCode,omitempty"`
	ErrorMessage string       `xml:"ErrorMessage,omitempty"`
	Prices       []QuotePrice `xml:"Prices>Price,omitempty"`
}

// QuotePrice represents a single price quote from Yurtiçi.
type QuotePrice struct {
	ProductCode      string  `xml:"ProductCode"`
	ProductName      string  `xml:"ProductName"`
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
	XMLName      xml.Name        `xml:"urn:YurticiKargo CreateShipment"`
	UserName     string          `xml:"UserName"`
	Password     string          `xml:"Password"`
	CustomerCode string          `xml:"CustomerCode"`
	Language     string          `xml:"Language,omitempty"`
	Shipment     ShipmentDetails `xml:"ShipmentInfo"`
}

// ShipmentDetails contains the shipment information.
type ShipmentDetails struct {
	ProductCode         string         `xml:"ProductCode"`
	QuoteID             string         `xml:"QuoteID,omitempty"`
	Reference           string         `xml:"Reference,omitempty"`
	InvoiceKey          string         `xml:"InvoiceKey,omitempty"`
	Sender              AddressDetails `xml:"Sender"`
	Receiver            AddressDetails `xml:"Receiver"`
	Cargo               CargoDetails   `xml:"Cargo"`
	CODAmount           float64        `xml:"CODAmount,omitempty"`
	DeclaredValue       float64        `xml:"DeclaredValue,omitempty"`
	LabelFormat         string         `xml:"LabelFormat"` // "PDF" or "ZPL"
	PaymentType         string         `xml:"PaymentType"` // "1" = Sender, "2" = Receiver
	CargoType           string         `xml:"CargoType"`   // "D" = Document, "K" = Parcel
	SpecialInstructions string         `xml:"SpecialInstructions,omitempty"`
	TTDocumentID        string         `xml:"TTDocumentID,omitempty"`
	TTDocumentSaveType  string         `xml:"TTDocumentSaveType,omitempty"`
}

// AddressDetails contains address information.
type AddressDetails struct {
	Name        string `xml:"Name"`
	CompanyName string `xml:"CompanyName,omitempty"`
	Phone       string `xml:"Phone"`
	Email       string `xml:"Email,omitempty"`
	Address     string `xml:"Address"`
	CityCode    string `xml:"CityCode"`
	TownCode    string `xml:"TownCode"`
	PostalCode  string `xml:"PostalCode,omitempty"`
	TaxNumber   string `xml:"TaxNumber,omitempty"`
	TaxOffice   string `xml:"TaxOffice,omitempty"`
}

// CargoDetails contains cargo information.
type CargoDetails struct {
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
	TrackingNumber string `xml:"CargoKey,omitempty"`
	Barcode        string `xml:"Barcode,omitempty"`
	LabelData      string `xml:"LabelData,omitempty"` // Base64 encoded
	LabelFormat    string `xml:"LabelFormat,omitempty"`
}

// CancelShipmentRequest is the SOAP request for cancelling a shipment.
type CancelShipmentRequest struct {
	XMLName      xml.Name `xml:"urn:YurticiKargo CancelShipment"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
	CargoKey     string   `xml:"CargoKey,omitempty"`
	InvoiceKey   string   `xml:"InvoiceKey,omitempty"`
	CancelReason string   `xml:"CancelReason,omitempty"`
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
	XMLName      xml.Name `xml:"urn:YurticiKargo GetLabel"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
	CargoKey     string   `xml:"CargoKey,omitempty"`
	InvoiceKey   string   `xml:"InvoiceKey,omitempty"`
	LabelFormat  string   `xml:"LabelFormat"` // "PDF" or "ZPL"
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
	XMLName      xml.Name `xml:"urn:YurticiKargo QueryShipment"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
	CargoKey     string   `xml:"CargoKey,omitempty"`
	InvoiceKey   string   `xml:"InvoiceKey,omitempty"`
}

// GetTrackingResponse is the SOAP response for getting tracking information.
type GetTrackingResponse struct {
	XMLName xml.Name              `xml:"QueryShipmentResponse"`
	Result  GetTrackingResultData `xml:"QueryShipmentResult"`
}

// GetTrackingResultData contains the tracking result.
type GetTrackingResultData struct {
	Success           bool            `xml:"Success"`
	ErrorCode         string          `xml:"ErrorCode,omitempty"`
	ErrorMessage      string          `xml:"ErrorMessage,omitempty"`
	CurrentStatus     string          `xml:"CurrentStatus,omitempty"`
	StatusCode        string          `xml:"StatusCode,omitempty"`
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
	UnitName          string `xml:"UnitName,omitempty"`
}

// SchedulePickupRequest is the SOAP request for scheduling a pickup.
type SchedulePickupRequest struct {
	XMLName         xml.Name       `xml:"urn:YurticiKargo SchedulePickup"`
	UserName        string         `xml:"UserName"`
	Password        string         `xml:"Password"`
	CustomerCode    string         `xml:"CustomerCode"`
	Language        string         `xml:"Language,omitempty"`
	PickupAddress   AddressDetails `xml:"PickupAddress"`
	ScheduledDate   string         `xml:"ScheduledDate"`             // YYYY-MM-DD
	TimeWindowStart string         `xml:"TimeWindowStart,omitempty"` // HH:MM
	TimeWindowEnd   string         `xml:"TimeWindowEnd,omitempty"`   // HH:MM
	ParcelCount     int            `xml:"ParcelCount"`
	TotalWeight     float64        `xml:"TotalWeight"`         // kg
	CargoKeys       string         `xml:"CargoKeys,omitempty"` // Comma-separated
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
	XMLName      xml.Name `xml:"urn:YurticiKargo CancelPickup"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
	PickupID     string   `xml:"PickupID"`
	Reason       string   `xml:"Reason,omitempty"`
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
	XMLName          xml.Name        `xml:"urn:YurticiKargo CreateReturn"`
	UserName         string          `xml:"UserName"`
	Password         string          `xml:"Password"`
	CustomerCode     string          `xml:"CustomerCode"`
	Language         string          `xml:"Language,omitempty"`
	OriginalCargoKey string          `xml:"OriginalCargoKey"`
	Shipment         ShipmentDetails `xml:"ShipmentInfo"`
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
	TrackingNumber string `xml:"CargoKey,omitempty"`
	Barcode        string `xml:"Barcode,omitempty"`
	LabelData      string `xml:"LabelData,omitempty"`
	LabelFormat    string `xml:"LabelFormat,omitempty"`
}

// GetDropoffPointsRequest is the SOAP request for getting dropoff points.
type GetDropoffPointsRequest struct {
	XMLName      xml.Name `xml:"urn:YurticiKargo GetAgencies"`
	UserName     string   `xml:"UserName"`
	Password     string   `xml:"Password"`
	CustomerCode string   `xml:"CustomerCode"`
	Language     string   `xml:"Language,omitempty"`
	CityCode     string   `xml:"CityCode,omitempty"`
	TownCode     string   `xml:"TownCode,omitempty"`
	PostalCode   string   `xml:"PostalCode,omitempty"`
	Limit        int      `xml:"Limit,omitempty"`
}

// GetDropoffPointsResponse is the SOAP response for getting dropoff points.
type GetDropoffPointsResponse struct {
	XMLName xml.Name                   `xml:"GetAgenciesResponse"`
	Result  GetDropoffPointsResultData `xml:"GetAgenciesResult"`
}

// GetDropoffPointsResultData contains the dropoff points result.
type GetDropoffPointsResultData struct {
	Success      bool           `xml:"Success"`
	ErrorCode    string         `xml:"ErrorCode,omitempty"`
	ErrorMessage string         `xml:"ErrorMessage,omitempty"`
	Agencies     []DropoffPoint `xml:"Agencies>Agency,omitempty"`
}

// DropoffPoint represents a dropoff location (agency).
type DropoffPoint struct {
	ID           string  `xml:"AgencyCode"`
	Name         string  `xml:"AgencyName"`
	Address      string  `xml:"Address"`
	CityCode     string  `xml:"CityCode"`
	CityName     string  `xml:"CityName"`
	TownCode     string  `xml:"TownCode"`
	TownName     string  `xml:"TownName"`
	PostalCode   string  `xml:"PostalCode,omitempty"`
	Latitude     float64 `xml:"Latitude,omitempty"`
	Longitude    float64 `xml:"Longitude,omitempty"`
	Phone        string  `xml:"Phone,omitempty"`
	WorkingHours string  `xml:"WorkingHours,omitempty"`
	IsActive     bool    `xml:"IsActive"`
}

// TrackingWebhookPayload represents the webhook payload from Yurtiçi.
type TrackingWebhookPayload struct {
	CargoKey   string                 `json:"cargo_key"`
	InvoiceKey string                 `json:"invoice_key,omitempty"`
	Events     []TrackingWebhookEvent `json:"events"`
	Signature  string                 `json:"signature,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// TrackingWebhookEvent represents a tracking event in a webhook.
type TrackingWebhookEvent struct {
	StatusCode        string    `json:"status_code"`
	StatusDescription string    `json:"status_description"`
	LocationCity      string    `json:"location_city,omitempty"`
	LocationDetail    string    `json:"location_detail,omitempty"`
	OccurredAt        time.Time `json:"occurred_at"`
}

// ProductCode constants for Yurtiçi.
const (
	ProductCodeStandard = "STANDART"
	ProductCodeExpress  = "EKSPRES"
	ProductCodeSameDay  = "AYNI_GUN"
)

// CargoType constants for Yurtiçi.
const (
	CargoTypeDocument = "D"
	CargoTypeParcel   = "K"
)

// LabelFormatType constants for Yurtiçi.
const (
	LabelFormatPDF = "PDF"
	LabelFormatZPL = "ZPL"
)

// PaymentType constants for Yurtiçi.
const (
	PaymentTypeSender   = "1"
	PaymentTypeReceiver = "2"
)

// Language constants for Yurtiçi.
const (
	LanguageTurkish = "TR"
	LanguageEnglish = "EN"
)
