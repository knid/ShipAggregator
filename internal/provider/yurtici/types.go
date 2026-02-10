// Package yurtici implements the provider adapter for Yurtici Kargo.
// Yurtici Kargo uses a SOAP-based API (V3) with three methods:
// createShipment, cancelShipment, and queryShipment.
package yurtici

import (
	"encoding/xml"
	"time"
)

// SOAP namespace for Yurtici Kargo API.
const soapNamespace = "http://yurticikargo.com.tr/ShippingOrderDispatcherServices"

// Credentials represents Yurtici API credentials.
type Credentials struct {
	Username string `json:"username"`  // wsUserName
	Password string `json:"password"`  // wsPassword
	Language string `json:"language"`  // userLanguage, default "TR"
}

// --- createShipment ---

// CreateShipmentRequest is the SOAP request for createShipment.
type CreateShipmentRequest struct {
	XMLName        xml.Name          `xml:"ser:createShipment"`
	XMLNSSer       string            `xml:"xmlns:ser,attr"`
	WsUserName     string            `xml:"wsUserName"`
	WsPassword     string            `xml:"wsPassword"`
	UserLanguage   string            `xml:"userLanguage"`
	ShippingOrders []ShippingOrderVO `xml:"ShippingOrderVO"`
}

// ShippingOrderVO represents a single shipment order in the createShipment request.
type ShippingOrderVO struct {
	CargoKey          string `xml:"cargoKey"`                    // Unique cargo identifier (max 20 chars, required)
	InvoiceKey        string `xml:"invoiceKey"`                  // Unique per shipment (max 20 chars, required)
	ReceiverCustName  string `xml:"receiverCustName"`            // Receiver name (min 5 chars, min 4 letters, required)
	ReceiverAddress   string `xml:"receiverAddress"`             // Receiver address (min 10, max 200 chars, required)
	ReceiverPhone1    string `xml:"receiverPhone1"`              // Receiver phone (10 digits with area code, required)
	ReceiverPhone2    string `xml:"receiverPhone2,omitempty"`    // Optional second phone
	ReceiverPhone3    string `xml:"receiverPhone3,omitempty"`    // Optional third phone
	CityName          string `xml:"cityName,omitempty"`          // City name
	TownName          string `xml:"townName,omitempty"`          // District name
	Desi              string `xml:"desi,omitempty"`              // Volumetric weight (Double 9,3)
	Kg                string `xml:"kg,omitempty"`                // Actual weight (Double 9,3)
	CargoCount        int    `xml:"cargoCount,omitempty"`        // Number of cargo pieces
	WaybillNo         string `xml:"waybillNo,omitempty"`         // Waybill number
	SpecialField1     string `xml:"specialField1,omitempty"`     // Custom field 1
	SpecialField2     string `xml:"specialField2,omitempty"`     // Custom field 2
	SpecialField3     string `xml:"specialField3,omitempty"`     // Custom field 3
	TtCollectionType  string `xml:"ttCollectionType,omitempty"`  // COD type: "0"=Cash, "1"=CreditCard
	TtInvoiceAmount   string `xml:"ttInvoiceAmount,omitempty"`   // COD invoice amount
	TtDocumentId      string `xml:"ttDocumentId,omitempty"`      // COD document ID
	TtDocumentSaveType string `xml:"ttDocumentSaveType,omitempty"` // COD document save type
	DcSelectedCredit  string `xml:"dcSelectedCredit,omitempty"`  // CC COD selected credit
	DcCreditRule      string `xml:"dcCreditRule,omitempty"`      // CC COD credit rule
	OrgReceiverCustId string `xml:"orgReceiverCustId,omitempty"` // Organization receiver customer ID
	Description       string `xml:"description,omitempty"`       // Description
	TaxNumber         string `xml:"taxNumber,omitempty"`         // Tax number
	TaxOfficeId       string `xml:"taxOfficeId,omitempty"`       // Tax office ID
	TaxOfficeName     string `xml:"taxOfficeName,omitempty"`     // Tax office name
	EmailAddress      string `xml:"emailAddress,omitempty"`      // Email address
	OrgGeoCode        string `xml:"orgGeoCode,omitempty"`        // Geo code
	PrivilegeOrder    string `xml:"privilegeOrder,omitempty"`    // Privilege order
}

// CreateShipmentResponse is the SOAP response for createShipment.
type CreateShipmentResponse struct {
	XMLName xml.Name                `xml:"createShipmentResponse"`
	Result  ShippingOrderResultVO   `xml:"ShippingOrderResultVO"`
}

// ShippingOrderResultVO is the result of a createShipment or cancelShipment operation.
type ShippingOrderResultVO struct {
	OutFlag  string `xml:"outFlag"`  // 0=at least one success, 1=all failed, 2=unexpected error
	OutResult string `xml:"outResult"` // Description of result
	JobId    string `xml:"jobId"`    // YK request number
	Count    string `xml:"count"`    // Processed count
	ShippingOrderDetailVO []ShippingOrderDetailVO `xml:"shippingOrderDetailVO,omitempty"`
}

// ShippingOrderDetailVO contains per-item results from createShipment.
type ShippingOrderDetailVO struct {
	CargoKey   string `xml:"cargoKey"`
	InvoiceKey string `xml:"invoiceKey"`
	ErrCode    string `xml:"errCode"`    // 0=success
	ErrMessage string `xml:"errMessage"`
}

// --- cancelShipment ---

// CancelShipmentRequest is the SOAP request for cancelShipment.
type CancelShipmentRequest struct {
	XMLName      xml.Name `xml:"ser:cancelShipment"`
	XMLNSSer     string   `xml:"xmlns:ser,attr"`
	WsUserName   string   `xml:"wsUserName"`
	WsPassword   string   `xml:"wsPassword"`
	UserLanguage string   `xml:"userLanguage"`
	CargoKeys    []string `xml:"cargoKeys"`
}

// CancelShipmentResponse is the SOAP response for cancelShipment.
type CancelShipmentResponse struct {
	XMLName xml.Name                      `xml:"cancelShipmentResponse"`
	Result  CancelShipmentResultVO        `xml:"ShippingOrderResultVO"`
}

// CancelShipmentResultVO is the result of cancelShipment.
type CancelShipmentResultVO struct {
	OutFlag   string `xml:"outFlag"`
	OutResult string `xml:"outResult"`
	ShippingCancelDetailVO []ShippingCancelDetailVO `xml:"shippingCancelDetailVO,omitempty"`
}

// ShippingCancelDetailVO contains per-item results from cancelShipment.
type ShippingCancelDetailVO struct {
	CargoKey        string `xml:"cargoKey"`
	InvoiceKey      string `xml:"invoiceKey"`
	JobId           string `xml:"jobId"`
	DocId           string `xml:"docId"`
	OperationCode   string `xml:"operationCode"`   // 0=NOP, 1=IND, 2=ISR, 3=CNL, 4=ISC, 5=DLV
	OperationMessage string `xml:"operationMessage"`
	OperationStatus string `xml:"operationStatus"` // NOP, IND, ISR, CNL, ISC, DLV, BI
	ErrCode         string `xml:"errCode"`
	ErrMessage      string `xml:"errMessage"`
}

// --- queryShipment ---

// QueryShipmentRequest is the SOAP request for queryShipment.
type QueryShipmentRequest struct {
	XMLName           xml.Name `xml:"ser:queryShipment"`
	XMLNSSer          string   `xml:"xmlns:ser,attr"`
	WsUserName        string   `xml:"wsUserName"`
	WsPassword        string   `xml:"wsPassword"`
	WsLanguage        string   `xml:"wsLanguage"` // Note: different param name from other methods
	Keys              []string `xml:"keys"`
	KeyType           int      `xml:"keyType"`           // 0=cargo key, 1=invoice key
	AddHistoricalData bool     `xml:"addHistoricalData"` // Include tracking history
	OnlyTracking      bool     `xml:"onlyTracking"`      // Only tracking data
}

// QueryShipmentResponse is the SOAP response for queryShipment.
type QueryShipmentResponse struct {
	XMLName xml.Name              `xml:"queryShipmentResponse"`
	Result  ShippingDeliveryVO    `xml:"ShippingDeliveryVO"`
}

// ShippingDeliveryVO is the top-level result of queryShipment.
type ShippingDeliveryVO struct {
	OutFlag   string `xml:"outFlag"`
	OutResult string `xml:"outResult"`
	Count     string `xml:"count"`
	ShippingDeliveryDetailVO []ShippingDeliveryDetailVO `xml:"shippingDeliveryDetailVO,omitempty"`
}

// ShippingDeliveryDetailVO contains per-item delivery details from queryShipment.
type ShippingDeliveryDetailVO struct {
	// Identification
	CargoKey   string `xml:"cargoKey"`
	InvoiceKey string `xml:"invoiceKey"`
	DocId      string `xml:"docId"`
	JobId      string `xml:"jobId"`

	// Operation status
	OperationCode    string `xml:"operationCode"`    // 0=NOP, 1=IND, 2=ISR, 3=CNL, 4=ISC, 5=DLV, 6=BI
	OperationMessage string `xml:"operationMessage"`
	OperationStatus  string `xml:"operationStatus"`  // NOP, IND, ISR, CNL, ISC, DLV, BI

	// Receiver info
	ReceiverName    string `xml:"receiverName"`
	ReceiverAddress string `xml:"receiverAddress"`
	ReceiverPhone   string `xml:"receiverPhone"`
	CityName        string `xml:"cityName"`
	TownName        string `xml:"townName"`

	// Delivery info
	DeliveryDate      string `xml:"deliveryDate"`
	DeliveryTime      string `xml:"deliveryTime"`
	DeliveredTo       string `xml:"deliveredTo"`
	DeliveredUnitName string `xml:"deliveredUnitName"`
	DeliveredUnitId   string `xml:"deliveredUnitId"`

	// Return info
	ReturnDocId       string `xml:"returnDocId"`
	ReturnOperationCode string `xml:"returnOperationCode"`
	ReturnOperationStatus string `xml:"returnOperationStatus"`

	// Pricing
	Desi          string `xml:"desi"`
	Kg            string `xml:"kg"`
	CargoCount    string `xml:"cargoCount"`
	InvoiceAmount string `xml:"invoiceAmount"`

	// Error info
	ErrCode    string `xml:"errCode"`
	ErrMessage string `xml:"errMessage"`

	// Historical tracking events
	InvDocCargoVOArray []InvDocCargoVO `xml:"invDocCargoVOArray,omitempty"`

	// Custom fields
	InvDocFieldVOArray []InvDocFieldVO `xml:"invDocFieldVOArray,omitempty"`
}

// InvDocCargoVO represents a historical tracking event from queryShipment.
type InvDocCargoVO struct {
	UnitId     string `xml:"unitId"`
	UnitName   string `xml:"unitName"`
	EventId    string `xml:"eventId"`    // IN, YK, CK, TM, DG, etc.
	EventName  string `xml:"eventName"`
	ReasonId   string `xml:"reasonId"`
	ReasonName string `xml:"reasonName"`
	EventDate  string `xml:"eventDate"`  // YYYYMMDD
	EventTime  string `xml:"eventTime"`  // HHMMSS
	CityName   string `xml:"cityName"`
	TownName   string `xml:"townName"`
}

// InvDocFieldVO represents a custom field from queryShipment.
type InvDocFieldVO struct {
	FieldName  string `xml:"fieldName"`
	FieldValue string `xml:"fieldValue"`
}

// --- Webhook types (kept from existing implementation) ---

// TrackingWebhookPayload represents the webhook payload from Yurtici.
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

// Language constants for Yurtici.
const (
	LanguageTurkish = "TR"
	LanguageEnglish = "EN"
)
