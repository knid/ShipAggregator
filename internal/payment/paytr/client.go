// Package paytr provides integration with the PayTR payment gateway.
package paytr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/knid/kargonet/internal/observability"
)

const (
	defaultBaseURL = "https://www.paytr.com"
	defaultTimeout = 30 * time.Second

	endpointGetToken     = "/odeme/api/get-token"
	endpointStatusQuery  = "/odeme/durum-sorgu"
	iframePath           = "/odeme/guvenli/"
)

// Config holds the configuration for the PayTR client.
type Config struct {
	MerchantID   string
	MerchantKey  string
	MerchantSalt string
	BaseURL      string
	TestMode     bool
	CallbackURL  string // Server-to-server notification URL
	OkURL        string // User-facing success redirect (shown in iframe)
	FailURL      string // User-facing failure redirect (shown in iframe)
	Timeout      time.Duration
}

// Client is the PayTR API client.
type Client struct {
	config     Config
	httpClient *http.Client
	logger     *observability.Logger
}

// NewClient creates a new PayTR client.
func NewClient(config Config, logger *observability.Logger) *Client {
	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
	}
}

// GetToken obtains an iframe token from PayTR.
// POST /odeme/api/get-token with form-encoded body.
func (c *Client) GetToken(ctx context.Context, req *GetTokenRequest) (*GetTokenResponse, error) {
	testMode := "0"
	if c.config.TestMode {
		testMode = "1"
	}

	noInstallment := strconv.Itoa(req.NoInstallment)
	maxInstallment := strconv.Itoa(req.MaxInstallment)
	paymentAmount := strconv.Itoa(req.PaymentAmount)

	currency := req.Currency
	if currency == "" {
		currency = "TL"
	}

	timeoutLimit := req.TimeoutLimit
	if timeoutLimit == 0 {
		timeoutLimit = 30
	}

	// Build HMAC hash string:
	// merchant_id + user_ip + merchant_oid + email + payment_amount +
	// user_basket + no_installment + max_installment + currency + test_mode
	hashStr := strings.Join([]string{
		c.config.MerchantID,
		req.UserIP,
		req.MerchantOID,
		req.Email,
		paymentAmount,
		req.UserBasket,
		noInstallment,
		maxInstallment,
		currency,
		testMode,
	}, "")

	token := c.computeHMAC(hashStr)

	// Build form values
	form := url.Values{
		"merchant_id":      {c.config.MerchantID},
		"user_ip":          {req.UserIP},
		"merchant_oid":     {req.MerchantOID},
		"email":            {req.Email},
		"payment_amount":   {paymentAmount},
		"paytr_token":      {token},
		"user_basket":      {req.UserBasket},
		"debug_on":         {"1"},
		"no_installment":   {noInstallment},
		"max_installment":  {maxInstallment},
		"user_name":        {req.UserName},
		"user_address":     {req.UserAddress},
		"user_phone":       {req.UserPhone},
		"merchant_ok_url":  {req.MerchantOkURL},
		"merchant_fail_url": {req.MerchantFailURL},
		"timeout_limit":    {strconv.Itoa(timeoutLimit)},
		"currency":         {currency},
		"test_mode":        {testMode},
	}

	// Override ok/fail URLs from config if not set in request
	if form.Get("merchant_ok_url") == "" {
		form.Set("merchant_ok_url", c.config.OkURL)
	}
	if form.Get("merchant_fail_url") == "" {
		form.Set("merchant_fail_url", c.config.FailURL)
	}

	c.logger.Debug("PayTR get-token request",
		"merchant_oid", req.MerchantOID,
		"payment_amount", paymentAmount,
	)

	apiURL := c.config.BaseURL + endpointGetToken
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("PayTR get-token request failed",
			"error", err,
			"duration_ms", duration.Milliseconds(),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("PayTR get-token response",
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	var result GetTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body))
	}

	if result.Status != "success" {
		return &result, &PayTRError{
			Status: result.Status,
			Reason: result.Reason,
		}
	}

	return &result, nil
}

// QueryPaymentStatus queries the status of a payment.
// POST /odeme/durum-sorgu with form-encoded body.
func (c *Client) QueryPaymentStatus(ctx context.Context, merchantOID string) (*PaymentStatusResponse, error) {
	// Hash: merchant_id + merchant_oid + merchant_salt
	hashStr := c.config.MerchantID + merchantOID + c.config.MerchantSalt
	token := c.computeHMAC(hashStr)

	form := url.Values{
		"merchant_id":  {c.config.MerchantID},
		"merchant_oid": {merchantOID},
		"paytr_token":  {token},
	}

	apiURL := c.config.BaseURL + endpointStatusQuery
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result PaymentStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// VerifyCallbackHash verifies the HMAC hash from a PayTR callback.
// Hash: HMAC-SHA256(merchant_oid + merchant_salt + status + total_amount, merchant_key) → base64
func (c *Client) VerifyCallbackHash(merchantOID, status, totalAmount, receivedHash string) bool {
	hashStr := merchantOID + c.config.MerchantSalt + status + totalAmount
	expected := c.computeHMAC(hashStr)
	return hmac.Equal([]byte(expected), []byte(receivedHash))
}

// IFrameURL returns the full iframe URL for the given token.
func (c *Client) IFrameURL(token string) string {
	return c.config.BaseURL + iframePath + token
}

// computeHMAC computes HMAC-SHA256 and returns base64 encoded result.
func (c *Client) computeHMAC(data string) string {
	mac := hmac.New(sha256.New, []byte(c.config.MerchantKey))
	mac.Write([]byte(data + c.config.MerchantSalt))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
