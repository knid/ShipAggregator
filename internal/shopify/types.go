// Package shopify provides types and a client for the Shopify Admin API.
package shopify

import "time"

// ShopInfo represents a Shopify store's basic information.
type ShopInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Domain      string `json:"domain"`
	MyshopifyDomain string `json:"myshopify_domain"`
	Currency    string `json:"currency"`
	PlanName    string `json:"plan_name"`
}

// ShopResponse wraps the shop endpoint response.
type ShopResponse struct {
	Shop ShopInfo `json:"shop"`
}

// AccessTokenResponse is the response from the OAuth token exchange.
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
}

// Order represents a Shopify order from the webhook payload.
type Order struct {
	ID                 int64       `json:"id"`
	Name               string      `json:"name"`
	OrderNumber        int64       `json:"order_number"`
	Email              string      `json:"email"`
	Phone              string      `json:"phone"`
	FinancialStatus    string      `json:"financial_status"`
	FulfillmentStatus  *string     `json:"fulfillment_status"`
	Currency           string      `json:"currency"`
	TotalPrice         string      `json:"total_price"`
	SubtotalPrice      string      `json:"subtotal_price"`
	TotalWeight        int64       `json:"total_weight"` // grams
	Note               string      `json:"note"`
	Tags               string      `json:"tags"`
	Customer           *Customer   `json:"customer"`
	ShippingAddress    *Address    `json:"shipping_address"`
	BillingAddress     *Address    `json:"billing_address"`
	LineItems          []LineItem  `json:"line_items"`
	ShippingLines      []ShippingLine `json:"shipping_lines"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	CancelledAt        *time.Time  `json:"cancelled_at"`
	ClosedAt           *time.Time  `json:"closed_at"`
}

// Customer represents a Shopify customer.
type Customer struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// FullName returns the customer's full name.
func (c *Customer) FullName() string {
	if c == nil {
		return ""
	}
	name := c.FirstName
	if c.LastName != "" {
		if name != "" {
			name += " "
		}
		name += c.LastName
	}
	return name
}

// Address represents a Shopify address.
type Address struct {
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Company      string  `json:"company"`
	Address1     string  `json:"address1"`
	Address2     string  `json:"address2"`
	City         string  `json:"city"`
	Province     string  `json:"province"`
	ProvinceCode string  `json:"province_code"`
	Country      string  `json:"country"`
	CountryCode  string  `json:"country_code"`
	Zip          string  `json:"zip"`
	Phone        string  `json:"phone"`
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
}

// LineItem represents a line item in a Shopify order.
type LineItem struct {
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	Quantity         int    `json:"quantity"`
	Price            string `json:"price"`
	Grams            int64  `json:"grams"`
	SKU              string `json:"sku"`
	RequiresShipping bool   `json:"requires_shipping"`
	FulfillmentStatus *string `json:"fulfillment_status"`
	ProductID        *int64 `json:"product_id"`
	VariantID        *int64 `json:"variant_id"`
}

// ShippingLine represents a shipping line on a Shopify order.
type ShippingLine struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Price string `json:"price"`
}

// FulfillmentRequest is the request body for creating a fulfillment.
type FulfillmentRequest struct {
	OrderID        int64              `json:"order_id"`
	TrackingNumber string             `json:"tracking_number"`
	TrackingURL    string             `json:"tracking_url,omitempty"`
	TrackingCompany string            `json:"tracking_company"`
	LineItems      []FulfillmentLineItem `json:"line_items,omitempty"`
}

// FulfillmentLineItem specifies which line items are fulfilled.
type FulfillmentLineItem struct {
	ID       int64 `json:"id"`
	Quantity int   `json:"quantity,omitempty"`
}

// FulfillmentResponse is returned from the fulfillment creation.
type FulfillmentResponse struct {
	Fulfillment struct {
		ID             int64  `json:"id"`
		Status         string `json:"status"`
		TrackingNumber string `json:"tracking_number"`
		TrackingURL    string `json:"tracking_url"`
	} `json:"fulfillment"`
}

// WebhookSubscription represents a Shopify webhook subscription.
type WebhookSubscription struct {
	ID      string `json:"id"`
	Topic   string `json:"topic"`
	Address string `json:"address"`
}
