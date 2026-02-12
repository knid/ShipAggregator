// Package paytr provides integration with the PayTR payment gateway.
package paytr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// GetTokenRequest holds the parameters for obtaining an iframe token.
type GetTokenRequest struct {
	MerchantOID    string // Unique order ID (topup UUID)
	Email          string
	PaymentAmount  int    // Amount in kurus-style: 9.99 TRY → 999
	UserIP         string
	UserName       string
	UserAddress    string
	UserPhone      string
	UserBasket     string // Base64 encoded JSON basket
	MerchantOkURL  string
	MerchantFailURL string
	Currency       string // TL, USD, EUR, GBP, RUB
	TimeoutLimit   int    // Minutes (default 30)
	NoInstallment  int    // 1 = disable installments
	MaxInstallment int    // 0 = use default
}

// GetTokenResponse holds the response from the get-token endpoint.
type GetTokenResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
	Reason string `json:"reason"`
}

// CallbackPayload holds the fields POSTed by PayTR's server-to-server callback.
type CallbackPayload struct {
	MerchantOID string `json:"merchant_oid"`
	Status      string `json:"status"` // "success" or "failed"
	TotalAmount string `json:"total_amount"`
	Hash        string `json:"hash"`
}

// PaymentStatusResponse holds the response from the payment status query endpoint.
type PaymentStatusResponse struct {
	Status        string `json:"status"`
	PaymentAmount string `json:"payment_amount"`
	PaymentTotal  string `json:"payment_total"`
	PaymentDate   string `json:"payment_date"`
	Currency      string `json:"currency"`
	Returns       string `json:"returns"`
	ErrNo         string `json:"err_no"`
	ErrMsg        string `json:"err_msg"`
}

// PayTRError represents an error from the PayTR API.
type PayTRError struct {
	Status string
	Reason string
}

// Error implements the error interface.
func (e *PayTRError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("paytr error [%s]: %s", e.Status, e.Reason)
	}
	return fmt.Sprintf("paytr error: status=%s", e.Status)
}

// basketItem represents a single item in the PayTR basket format.
// PayTR expects: [[name, price, quantity], ...]
type basketItem = [3]string

// NewWalletTopupBasket creates a base64-encoded basket JSON for a wallet top-up.
// PayTR basket format: base64(json([[name, price, quantity], ...]))
// price is a string like "9.99".
func NewWalletTopupBasket(name, price string) string {
	basket := []basketItem{
		{name, price, "1"},
	}
	data, _ := json.Marshal(basket)
	return base64.StdEncoding.EncodeToString(data)
}
