// Package aras implements the provider adapter for Aras Kargo.
// It uses the ArasCargoIntegrationService.svc unified SOAP service
// with GetQueryJSON for queries and SetDataXML for writes.
package aras

import (
	"encoding/json"
	"encoding/xml"
	"time"
)

// ---------------------------------------------------------------------------
// Credentials
// ---------------------------------------------------------------------------

// Credentials represents Aras API credentials.
type Credentials struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	CustomerCode string `json:"customer_code"`
}

// ---------------------------------------------------------------------------
// Integration Service — GetQueryJSON / SetDataXML
// (ArasCargoIntegrationService.svc, SOAP 1.1 with BasicHttpsBinding)
// ---------------------------------------------------------------------------

// GetQueryJSONRequest is the SOAP request for the integration query service.
type GetQueryJSONRequest struct {
	XMLName   xml.Name `xml:"http://tempuri.org/ GetQueryJSON"`
	LoginInfo string   `xml:"loginInfo"`
	QueryInfo string   `xml:"queryInfo"`
}

// GetQueryJSONResponse is the SOAP response wrapping a JSON string result.
type GetQueryJSONResponse struct {
	XMLName xml.Name `xml:"GetQueryJSONResponse"`
	Result  string   `xml:"GetQueryJSONResult"`
}

// SetDataXMLRequest is the SOAP request for setting data.
type SetDataXMLRequest struct {
	XMLName   xml.Name `xml:"http://tempuri.org/ SetDataXML"`
	LoginInfo string   `xml:"loginInfo"`
	QueryInfo string   `xml:"queryInfo"`
}

// SetDataXMLResponse is the SOAP response for SetDataXML.
type SetDataXMLResponse struct {
	XMLName xml.Name `xml:"SetDataXMLResponse"`
	Result  string   `xml:"SetDataXMLResult"`
}

// QueryResponse is the top-level JSON response from GetQueryJSON.
type QueryResponse struct {
	QueryResult json.RawMessage `json:"QueryResult"`
}

// QueryType constants for the integration service.
const (
	QueryTypeCargoInfo        = 1  // Cargo status by tracking number
	QueryTypeCargoByDate      = 2  // Cargo list by send date
	QueryTypeDeliveredByDate  = 3  // Delivered cargo by delivery date
	QueryTypeUndelivered      = 5  // Undelivered cargo at branch
	QueryTypeCargoMovement    = 9  // Cargo movement history by tracking number
	QueryTypeAllBranches      = 10 // All branch addresses
	QueryTypeCargoByDateRange = 18 // Cargo by date range
	QueryTypeCargoRealInfo    = 22 // Full cargo info (DataSet format)
	QueryTypeFindBarcode      = 24 // Find cargo by barcode
	QueryTypeTodayCount       = 31 // Today's delivery count
	QueryTypeRepresentatives  = 33 // Customer representatives
)

// ---------------------------------------------------------------------------
// Webhook types (JSON-based, kept from existing implementation)
// ---------------------------------------------------------------------------

// TrackingWebhookPayload represents the webhook payload from Aras.
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

// ---------------------------------------------------------------------------
// SetOrder SOAP types (shipment .asmx endpoint)
// ---------------------------------------------------------------------------

// SOAP actions for the .asmx shipment endpoint.
const (
	soapActionSetOrder       = "http://tempuri.org/SetOrder"
	soapActionCancelDispatch = "http://tempuri.org/CancelDispatch"
)

// SetOrderRequest is the SOAP request for creating a shipment via the .asmx endpoint.
type SetOrderRequest struct {
	XMLName   xml.Name  `xml:"http://tempuri.org/ SetOrder"`
	OrderInfo OrderInfo `xml:"orderInfo"`
	UserName  string    `xml:"userName"`
	Password  string    `xml:"password"`
}

// OrderInfo wraps the Order element inside SetOrder.
type OrderInfo struct {
	Order Order `xml:"Order"`
}

// Order represents an Aras shipment order.
type Order struct {
	UserName             string        `xml:"UserName"`
	Password             string        `xml:"Password"`
	TradingWaybillNumber string        `xml:"TradingWaybillNumber"`
	InvoiceNumber        string        `xml:"InvoiceNumber"`
	IntegrationCode      string        `xml:"IntegrationCode"`
	ReceiverName         string        `xml:"ReceiverName"`
	ReceiverAddress      string        `xml:"ReceiverAddress"`
	ReceiverCityName     string        `xml:"ReceiverCityName"`
	ReceiverTownName     string        `xml:"ReceiverTownName"`
	ReceiverPhone1       string        `xml:"ReceiverPhone1"`
	ReceiverPhone2       string        `xml:"ReceiverPhone2,omitempty"`
	ReceiverPhone3       string        `xml:"ReceiverPhone3,omitempty"`
	VolumetricWeight     string        `xml:"VolumetricWeight"`
	Weight               string        `xml:"Weight"`
	CityCode             string        `xml:"CityCode,omitempty"`
	TownCode             string        `xml:"TownCode,omitempty"`
	IsCod                string        `xml:"IsCod"`
	CodAmount            string        `xml:"CodAmount,omitempty"`
	CodCollectionType    string        `xml:"CodCollectionType,omitempty"`
	CodBillingType       string        `xml:"CodBillingType,omitempty"`
	PayorTypeCode        string        `xml:"PayorTypeCode,omitempty"`
	IsWorldWide          string        `xml:"IsWorldWide,omitempty"`
	Description          string        `xml:"Description,omitempty"`
	SpecialField1        string        `xml:"SpecialField1,omitempty"`
	SpecialField2        string        `xml:"SpecialField2,omitempty"`
	SpecialField3        string        `xml:"SpecialField3,omitempty"`
	PieceCount           int           `xml:"PieceCount,omitempty"`
	PieceDetails         *PieceDetails `xml:"PieceDetails,omitempty"`
}

// PieceDetails wraps a list of PieceDetail elements.
type PieceDetails struct {
	PieceDetail []PieceDetail `xml:"PieceDetail"`
}

// PieceDetail represents a single piece in a shipment.
type PieceDetail struct {
	VolumetricWeight string `xml:"VolumetricWeight"`
	Weight           string `xml:"Weight"`
	BarcodeNumber    string `xml:"BarcodeNumber"`
	ProductNumber    string `xml:"ProductNumber,omitempty"`
	Description      string `xml:"Description,omitempty"`
}

// SetOrderResponse is the SOAP response for SetOrder.
type SetOrderResponse struct {
	XMLName xml.Name       `xml:"SetOrderResponse"`
	Result  SetOrderResult `xml:"SetOrderResult"`
}

// SetOrderResult wraps the order result info.
type SetOrderResult struct {
	OrderResultInfo *OrderResultInfo `xml:"OrderResultInfo"`
}

// OrderResultInfo contains the result of a SetOrder call.
type OrderResultInfo struct {
	IntegrationCode string `xml:"IntegrationCode"`
	TrackingNumber  string `xml:"TrackingNumber"`
	ResultCode      string `xml:"ResultCode"`
	ResultMessage   string `xml:"ResultMessage"`
}

// ---------------------------------------------------------------------------
// CancelDispatch SOAP types (shipment .asmx endpoint)
// ---------------------------------------------------------------------------

// CancelDispatchRequest is the SOAP request for cancelling a shipment via the .asmx endpoint.
type CancelDispatchRequest struct {
	XMLName           xml.Name `xml:"http://tempuri.org/ CancelDispatch"`
	UserName          string   `xml:"userName"`
	Password          string   `xml:"password"`
	IntegrationCode   string   `xml:"integrationCode"`
}

// CancelDispatchResponse is the SOAP response for CancelDispatch.
type CancelDispatchResponse struct {
	XMLName xml.Name             `xml:"CancelDispatchResponse"`
	Result  CancelDispatchResult `xml:"CancelDispatchResult"`
}

// CancelDispatchResult contains the result of a CancelDispatch call.
type CancelDispatchResult struct {
	ResultCode    string `xml:"ResultCode"`
	ResultMessage string `xml:"ResultMessage"`
}
