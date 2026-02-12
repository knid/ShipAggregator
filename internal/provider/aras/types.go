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
