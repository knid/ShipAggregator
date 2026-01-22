// Package iyzico provides integration with the Iyzico payment gateway.
package iyzico

import (
	"fmt"
	"time"
)

// BaseRequest contains common fields for all Iyzico requests.
type BaseRequest struct {
	Locale         string `json:"locale,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
}

// BaseResponse contains common fields for all Iyzico responses.
type BaseResponse struct {
	Status         string `json:"status"`
	Locale         string `json:"locale,omitempty"`
	SystemTime     int64  `json:"systemTime,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	ErrorCode      string `json:"errorCode,omitempty"`
	ErrorMessage   string `json:"errorMessage,omitempty"`
	ErrorGroup     string `json:"errorGroup,omitempty"`
}

// IsSuccess returns true if the response indicates success.
func (r *BaseResponse) IsSuccess() bool {
	return r.Status == "success"
}

// GetError returns an error if the response indicates failure.
func (r *BaseResponse) GetError() error {
	if r.IsSuccess() {
		return nil
	}
	return &IyzicoError{
		Code:    r.ErrorCode,
		Message: r.ErrorMessage,
	}
}

// Buyer represents buyer information for Iyzico.
type Buyer struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Surname             string `json:"surname"`
	GsmNumber           string `json:"gsmNumber,omitempty"`
	Email               string `json:"email"`
	IdentityNumber      string `json:"identityNumber"`
	LastLoginDate       string `json:"lastLoginDate,omitempty"`
	RegistrationDate    string `json:"registrationDate,omitempty"`
	RegistrationAddress string `json:"registrationAddress"`
	IP                  string `json:"ip"`
	City                string `json:"city"`
	Country             string `json:"country"`
	ZipCode             string `json:"zipCode,omitempty"`
}

// Address represents an address for Iyzico.
type Address struct {
	ContactName string `json:"contactName"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Address     string `json:"address"`
	ZipCode     string `json:"zipCode,omitempty"`
}

// BasketItem represents an item in the basket.
type BasketItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category1 string `json:"category1"`
	Category2 string `json:"category2,omitempty"`
	ItemType  string `json:"itemType"` // PHYSICAL, VIRTUAL
	Price     string `json:"price"`
}

// ItemType constants.
const (
	ItemTypePhysical = "PHYSICAL"
	ItemTypeVirtual  = "VIRTUAL"
)

// PaymentGroup constants.
const (
	PaymentGroupProduct      = "PRODUCT"
	PaymentGroupListing      = "LISTING"
	PaymentGroupSubscription = "SUBSCRIPTION"
)

// CheckoutFormInitializeRequest represents a request to initialize a checkout form.
type CheckoutFormInitializeRequest struct {
	BaseRequest
	Price               string       `json:"price"`
	PaidPrice           string       `json:"paidPrice"`
	Currency            string       `json:"currency"`
	BasketID            string       `json:"basketId"`
	PaymentGroup        string       `json:"paymentGroup"`
	CallbackURL         string       `json:"callbackUrl"`
	EnabledInstallments []int        `json:"enabledInstallments"`
	Buyer               Buyer        `json:"buyer"`
	ShippingAddress     Address      `json:"shippingAddress"`
	BillingAddress      Address      `json:"billingAddress"`
	BasketItems         []BasketItem `json:"basketItems"`
	PaymentSource       string       `json:"paymentSource,omitempty"`
	ForceThreeDS        int          `json:"forceThreeDS,omitempty"`
	CardUserKey         string       `json:"cardUserKey,omitempty"`
}

// CheckoutFormInitializeResponse represents the response from initializing a checkout form.
type CheckoutFormInitializeResponse struct {
	BaseResponse
	Token               string `json:"token,omitempty"`
	CheckoutFormContent string `json:"checkoutFormContent,omitempty"`
	TokenExpireTime     int    `json:"tokenExpireTime,omitempty"`
	PaymentPageURL      string `json:"paymentPageUrl,omitempty"`
}

// GetPaymentURL returns the payment URL for the checkout form.
func (r *CheckoutFormInitializeResponse) GetPaymentURL() string {
	return r.PaymentPageURL
}

// GetToken returns the checkout form token.
func (r *CheckoutFormInitializeResponse) GetToken() string {
	return r.Token
}

// GetExpiresAt returns the expiration time of the token.
func (r *CheckoutFormInitializeResponse) GetExpiresAt() time.Time {
	return time.Now().Add(time.Duration(r.TokenExpireTime) * time.Second)
}

// CheckoutFormRetrieveRequest represents a request to retrieve checkout form result.
type CheckoutFormRetrieveRequest struct {
	BaseRequest
	Token string `json:"token"`
}

// ItemTransaction represents a transaction for a basket item.
type ItemTransaction struct {
	ItemID                        string          `json:"itemId,omitempty"`
	PaymentTransactionID          string          `json:"paymentTransactionId,omitempty"`
	TransactionStatus             int             `json:"transactionStatus,omitempty"`
	Price                         float64         `json:"price,omitempty"`
	PaidPrice                     float64         `json:"paidPrice,omitempty"`
	MerchantCommissionRate        float64         `json:"merchantCommissionRate,omitempty"`
	MerchantCommissionRateAmount  float64         `json:"merchantCommissionRateAmount,omitempty"`
	IyziCommissionRateAmount      float64         `json:"iyziCommissionRateAmount,omitempty"`
	IyziCommissionFee             float64         `json:"iyziCommissionFee,omitempty"`
	BlockageRate                  float64         `json:"blockageRate,omitempty"`
	BlockageRateAmountMerchant    float64         `json:"blockageRateAmountMerchant,omitempty"`
	BlockageRateAmountSubMerchant float64         `json:"blockageRateAmountSubMerchant,omitempty"`
	BlockageResolvedDate          string          `json:"blockageResolvedDate,omitempty"`
	SubMerchantKey                string          `json:"subMerchantKey,omitempty"`
	SubMerchantPrice              float64         `json:"subMerchantPrice,omitempty"`
	SubMerchantPayoutRate         float64         `json:"subMerchantPayoutRate,omitempty"`
	SubMerchantPayoutAmount       float64         `json:"subMerchantPayoutAmount,omitempty"`
	MerchantPayoutAmount          float64         `json:"merchantPayoutAmount,omitempty"`
	ConvertedPayout               ConvertedPayout `json:"convertedPayout,omitempty"`
}

// ConvertedPayout represents converted payout information.
type ConvertedPayout struct {
	PaidPrice                     float64 `json:"paidPrice,omitempty"`
	IyziCommissionRateAmount      float64 `json:"iyziCommissionRateAmount,omitempty"`
	IyziCommissionFee             float64 `json:"iyziCommissionFee,omitempty"`
	BlockageRateAmountMerchant    float64 `json:"blockageRateAmountMerchant,omitempty"`
	BlockageRateAmountSubMerchant float64 `json:"blockageRateAmountSubMerchant,omitempty"`
	SubMerchantPayoutAmount       float64 `json:"subMerchantPayoutAmount,omitempty"`
	MerchantPayoutAmount          float64 `json:"merchantPayoutAmount,omitempty"`
	IyziConversionRate            float64 `json:"iyziConversionRate,omitempty"`
	IyziConversionRateAmount      float64 `json:"iyziConversionRateAmount,omitempty"`
	Currency                      string  `json:"currency,omitempty"`
}

// CheckoutFormRetrieveResponse represents the response from retrieving checkout form result.
type CheckoutFormRetrieveResponse struct {
	BaseResponse
	Price                        float64           `json:"price,omitempty"`
	PaidPrice                    float64           `json:"paidPrice,omitempty"`
	Currency                     string            `json:"currency,omitempty"`
	Installment                  int               `json:"installment,omitempty"`
	PaymentID                    string            `json:"paymentId,omitempty"`
	PaymentStatus                string            `json:"paymentStatus,omitempty"`
	FraudStatus                  int               `json:"fraudStatus,omitempty"`
	MerchantCommissionRate       float64           `json:"merchantCommissionRate,omitempty"`
	MerchantCommissionRateAmount float64           `json:"merchantCommissionRateAmount,omitempty"`
	IyziCommissionRateAmount     float64           `json:"iyziCommissionRateAmount,omitempty"`
	IyziCommissionFee            float64           `json:"iyziCommissionFee,omitempty"`
	CardType                     string            `json:"cardType,omitempty"`
	CardAssociation              string            `json:"cardAssociation,omitempty"`
	CardFamily                   string            `json:"cardFamily,omitempty"`
	CardToken                    string            `json:"cardToken,omitempty"`
	CardUserKey                  string            `json:"cardUserKey,omitempty"`
	BinNumber                    string            `json:"binNumber,omitempty"`
	LastFourDigits               string            `json:"lastFourDigits,omitempty"`
	BasketID                     string            `json:"basketId,omitempty"`
	ItemTransactions             []ItemTransaction `json:"itemTransactions,omitempty"`
	AuthCode                     string            `json:"authCode,omitempty"`
	Phase                        string            `json:"phase,omitempty"`
	HostReference                string            `json:"hostReference,omitempty"`
	Token                        string            `json:"token,omitempty"`
	CallbackURL                  string            `json:"callbackUrl,omitempty"`
}

// IsPaymentSuccess returns true if the payment was successful.
func (r *CheckoutFormRetrieveResponse) IsPaymentSuccess() bool {
	return r.Status == "success" && r.PaymentStatus == "SUCCESS"
}

// IsFraudSuspected returns true if fraud is suspected.
func (r *CheckoutFormRetrieveResponse) IsFraudSuspected() bool {
	// FraudStatus: -1 = not checked, 0 = fraud suspected, 1 = passed
	return r.FraudStatus == 0
}

// GetPaymentID returns the payment ID.
func (r *CheckoutFormRetrieveResponse) GetPaymentID() string {
	return r.PaymentID
}

// GetCardInfo returns a formatted card info string.
func (r *CheckoutFormRetrieveResponse) GetCardInfo() string {
	return fmt.Sprintf("%s ****%s (%s)", r.CardAssociation, r.LastFourDigits, r.CardFamily)
}

// RefundRequest represents a request to refund a payment.
type RefundRequest struct {
	BaseRequest
	PaymentTransactionID string `json:"paymentTransactionId"`
	Price                string `json:"price"`
	Currency             string `json:"currency,omitempty"`
	IP                   string `json:"ip"`
	Reason               string `json:"reason,omitempty"`
	Description          string `json:"description,omitempty"`
}

// RefundResponse represents the response from a refund request.
type RefundResponse struct {
	BaseResponse
	PaymentID            string  `json:"paymentId,omitempty"`
	PaymentTransactionID string  `json:"paymentTransactionId,omitempty"`
	Price                float64 `json:"price,omitempty"`
	Currency             string  `json:"currency,omitempty"`
	AuthCode             string  `json:"authCode,omitempty"`
	HostReference        string  `json:"hostReference,omitempty"`
}

// PaymentDetailRequest represents a request to get payment details.
type PaymentDetailRequest struct {
	BaseRequest
	PaymentID             string `json:"paymentId,omitempty"`
	PaymentConversationID string `json:"paymentConversationId,omitempty"`
}

// PaymentDetailResponse represents the response from getting payment details.
type PaymentDetailResponse struct {
	BaseResponse
	Price                        float64           `json:"price,omitempty"`
	PaidPrice                    float64           `json:"paidPrice,omitempty"`
	Currency                     string            `json:"currency,omitempty"`
	Installment                  int               `json:"installment,omitempty"`
	PaymentID                    string            `json:"paymentId,omitempty"`
	PaymentStatus                string            `json:"paymentStatus,omitempty"`
	FraudStatus                  int               `json:"fraudStatus,omitempty"`
	MerchantCommissionRate       float64           `json:"merchantCommissionRate,omitempty"`
	MerchantCommissionRateAmount float64           `json:"merchantCommissionRateAmount,omitempty"`
	IyziCommissionRateAmount     float64           `json:"iyziCommissionRateAmount,omitempty"`
	IyziCommissionFee            float64           `json:"iyziCommissionFee,omitempty"`
	CardType                     string            `json:"cardType,omitempty"`
	CardAssociation              string            `json:"cardAssociation,omitempty"`
	CardFamily                   string            `json:"cardFamily,omitempty"`
	BinNumber                    string            `json:"binNumber,omitempty"`
	LastFourDigits               string            `json:"lastFourDigits,omitempty"`
	BasketID                     string            `json:"basketId,omitempty"`
	ItemTransactions             []ItemTransaction `json:"itemTransactions,omitempty"`
}

// WebhookPayload represents the payload of an Iyzico webhook.
type WebhookPayload struct {
	IyziEventType         string `json:"iyziEventType"`
	IyziEventTime         int64  `json:"iyziEventTime"`
	IyziReferenceCode     string `json:"iyziReferenceCode"`
	Token                 string `json:"token"`
	Status                string `json:"status"`
	PaymentID             int64  `json:"paymentId"`
	PaymentConversationID string `json:"paymentConversationId"`
	MerchantID            int64  `json:"merchantId,omitempty"`
	PaymentStatus         string `json:"paymentStatus,omitempty"`
}

// WebhookEventType constants.
const (
	WebhookEventCheckoutFormAuth = "CHECKOUT_FORM_AUTH"
	WebhookEventPaymentSuccess   = "PAYMENT_SUCCESS"
	WebhookEventPaymentFailure   = "PAYMENT_FAILURE"
	WebhookEventRefund           = "REFUND"
)

// IsSuccess returns true if the webhook indicates success.
func (w *WebhookPayload) IsSuccess() bool {
	return w.Status == "SUCCESS" || w.PaymentStatus == "SUCCESS"
}

// GetTopupID extracts the topup ID from the conversation ID.
func (w *WebhookPayload) GetTopupID() string {
	return w.PaymentConversationID
}

// CallbackPayload represents the payload from an Iyzico callback (form POST).
type CallbackPayload struct {
	Token string `json:"token"`
}

// Currency constants.
const (
	CurrencyTRY = "TRY"
	CurrencyUSD = "USD"
	CurrencyEUR = "EUR"
	CurrencyGBP = "GBP"
)

// Locale constants.
const (
	LocaleTR = "tr"
	LocaleEN = "en"
)

// PaymentStatus constants.
const (
	PaymentStatusSuccess     = "SUCCESS"
	PaymentStatusFailure     = "FAILURE"
	PaymentStatusInit        = "INIT_THREEDS"
	PaymentStatusCallback    = "CALLBACK_THREEDS"
	PaymentStatusBkmInit     = "BKM_POS_SELECTED"
	PaymentStatusBkmCallback = "CALLBACK_PECCO"
)

// FraudStatus constants.
const (
	FraudStatusNotChecked = -1
	FraudStatusSuspected  = 0
	FraudStatusPassed     = 1
)

// TransactionStatus constants.
const (
	TransactionStatusApproved = 2
	TransactionStatusPending  = 1
	TransactionStatusRejected = 0
)

// NewCheckoutFormRequest creates a new checkout form initialization request.
func NewCheckoutFormRequest(
	conversationID string,
	price string,
	currency string,
	basketID string,
	callbackURL string,
	buyer Buyer,
	billingAddress Address,
	basketItems []BasketItem,
) *CheckoutFormInitializeRequest {
	return &CheckoutFormInitializeRequest{
		BaseRequest: BaseRequest{
			Locale:         LocaleTR,
			ConversationID: conversationID,
		},
		Price:               price,
		PaidPrice:           price,
		Currency:            currency,
		BasketID:            basketID,
		PaymentGroup:        PaymentGroupProduct,
		CallbackURL:         callbackURL,
		EnabledInstallments: []int{1}, // Only single payment, no installments
		Buyer:               buyer,
		ShippingAddress:     billingAddress, // Same as billing for virtual goods
		BillingAddress:      billingAddress,
		BasketItems:         basketItems,
	}
}

// NewWalletTopupBasketItem creates a basket item for wallet top-up.
func NewWalletTopupBasketItem(price string) BasketItem {
	return BasketItem{
		ID:        "wallet_topup",
		Name:      "Wallet Top-Up",
		Category1: "Wallet",
		ItemType:  ItemTypeVirtual,
		Price:     price,
	}
}

// NewBuyerFromOrganization creates a Buyer from organization details.
func NewBuyerFromOrganization(
	orgID string,
	name string,
	email string,
	phone string,
	address string,
	city string,
	ip string,
) Buyer {
	return Buyer{
		ID:                  orgID,
		Name:                name,
		Surname:             "Company",
		GsmNumber:           phone,
		Email:               email,
		IdentityNumber:      "11111111111", // Placeholder for corporate
		RegistrationAddress: address,
		IP:                  ip,
		City:                city,
		Country:             "Turkey",
	}
}

// NewAddressFromOrganization creates an Address from organization details.
func NewAddressFromOrganization(name string, address string, city string) Address {
	return Address{
		ContactName: name,
		City:        city,
		Country:     "Turkey",
		Address:     address,
	}
}
