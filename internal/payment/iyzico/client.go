// Package iyzico provides integration with the Iyzico payment gateway.
package iyzico

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/knid/kargonet/internal/observability"
)

// Environment represents the Iyzico environment.
type Environment string

const (
	// EnvironmentSandbox is the sandbox environment for testing.
	EnvironmentSandbox Environment = "sandbox"
	// EnvironmentProduction is the production environment.
	EnvironmentProduction Environment = "production"
)

// BaseURLs for Iyzico environments.
const (
	SandboxBaseURL    = "https://sandbox-api.iyzipay.com"
	ProductionBaseURL = "https://api.iyzipay.com"
)

// API endpoints.
const (
	EndpointCheckoutFormInitialize = "/payment/iyzipos/checkoutform/initialize/auth/ecom"
	EndpointCheckoutFormRetrieve   = "/payment/iyzipos/checkoutform/auth/ecom/detail"
	EndpointRefund                 = "/payment/refund"
	EndpointPaymentDetail          = "/reporting/payment/detail"
)

// Config holds the configuration for the Iyzico client.
type Config struct {
	// APIKey is the Iyzico API key.
	APIKey string
	// SecretKey is the Iyzico secret key.
	SecretKey string
	// BaseURL is the Iyzico API base URL.
	BaseURL string
	// Environment is the Iyzico environment.
	Environment Environment
	// Timeout is the HTTP client timeout.
	Timeout time.Duration
	// CallbackURL is the URL for Iyzico callbacks.
	CallbackURL string
}

// DefaultConfig returns a default configuration for sandbox.
func DefaultConfig() Config {
	return Config{
		BaseURL:     SandboxBaseURL,
		Environment: EnvironmentSandbox,
		Timeout:     30 * time.Second,
	}
}

// Client is the Iyzico API client.
type Client struct {
	config     Config
	httpClient *http.Client
	logger     *observability.Logger
}

// NewClient creates a new Iyzico client.
func NewClient(config Config, logger *observability.Logger) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.BaseURL == "" {
		if config.Environment == EnvironmentProduction {
			config.BaseURL = ProductionBaseURL
		} else {
			config.BaseURL = SandboxBaseURL
		}
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
	}
}

// generateRandomKey generates a random key for request signing.
func generateRandomKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// generateAuthorizationHeader generates the Iyzico authorization header.
func (c *Client) generateAuthorizationHeader(requestBody string) (string, error) {
	randomKey, err := generateRandomKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Create hash string: randomKey + requestBody
	hashString := randomKey + requestBody

	// Generate HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(c.config.SecretKey))
	mac.Write([]byte(hashString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Create authorization string: apiKey:randomKey:signature
	authString := fmt.Sprintf("%s:%s:%s", c.config.APIKey, randomKey, signature)

	// Base64 encode final header
	return base64.StdEncoding.EncodeToString([]byte(authString)), nil
}

// doRequest performs an HTTP request to the Iyzico API.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}, response interface{}) error {
	// Serialize request body
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	// Create request
	url := c.config.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Generate and set authorization header
	authHeader, err := c.generateAuthorizationHeader(string(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to generate authorization header: %w", err)
	}
	req.Header.Set("Authorization", "IYZWS "+authHeader)

	// Log request (without sensitive data)
	c.logger.Debug("Iyzico API request",
		"method", method,
		"endpoint", endpoint,
		"url", url,
	)

	// Execute request
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		c.logger.Error("Iyzico API request failed",
			"method", method,
			"endpoint", endpoint,
			"error", err,
			"duration_ms", duration.Milliseconds(),
		)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Log response
	c.logger.Debug("Iyzico API response",
		"method", method,
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"duration_ms", duration.Milliseconds(),
	)

	// Check HTTP status code
	if resp.StatusCode >= 400 {
		return fmt.Errorf("iyzico API returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	if response != nil {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// InitializeCheckoutForm initializes a checkout form for payment.
func (c *Client) InitializeCheckoutForm(ctx context.Context, req *CheckoutFormInitializeRequest) (*CheckoutFormInitializeResponse, error) {
	var resp CheckoutFormInitializeResponse
	if err := c.doRequest(ctx, http.MethodPost, EndpointCheckoutFormInitialize, req, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return &resp, &IyzicoError{
			Code:    resp.ErrorCode,
			Message: resp.ErrorMessage,
		}
	}

	return &resp, nil
}

// RetrieveCheckoutFormResult retrieves the result of a checkout form payment.
func (c *Client) RetrieveCheckoutFormResult(ctx context.Context, req *CheckoutFormRetrieveRequest) (*CheckoutFormRetrieveResponse, error) {
	var resp CheckoutFormRetrieveResponse
	if err := c.doRequest(ctx, http.MethodPost, EndpointCheckoutFormRetrieve, req, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return &resp, &IyzicoError{
			Code:    resp.ErrorCode,
			Message: resp.ErrorMessage,
		}
	}

	return &resp, nil
}

// Refund processes a refund for a payment.
func (c *Client) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	var resp RefundResponse
	if err := c.doRequest(ctx, http.MethodPost, EndpointRefund, req, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return &resp, &IyzicoError{
			Code:    resp.ErrorCode,
			Message: resp.ErrorMessage,
		}
	}

	return &resp, nil
}

// GetPaymentDetail retrieves payment details.
func (c *Client) GetPaymentDetail(ctx context.Context, req *PaymentDetailRequest) (*PaymentDetailResponse, error) {
	var resp PaymentDetailResponse
	if err := c.doRequest(ctx, http.MethodPost, EndpointPaymentDetail, req, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return &resp, &IyzicoError{
			Code:    resp.ErrorCode,
			Message: resp.ErrorMessage,
		}
	}

	return &resp, nil
}

// VerifyWebhookSignature verifies the signature of an Iyzico webhook.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) bool {
	// Iyzico sends signature in X-IYZ-SIGNATURE header
	mac := hmac.New(sha256.New, []byte(c.config.SecretKey))
	mac.Write(payload)
	expected := fmt.Sprintf("%x", mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// IyzicoError represents an error from the Iyzico API.
type IyzicoError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *IyzicoError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("iyzico error [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("iyzico error: %s", e.Message)
}

// IsIyzicoError checks if an error is an IyzicoError.
func IsIyzicoError(err error) bool {
	_, ok := err.(*IyzicoError)
	return ok
}

// GetIyzicoError extracts the IyzicoError from an error.
func GetIyzicoError(err error) *IyzicoError {
	if e, ok := err.(*IyzicoError); ok {
		return e
	}
	return nil
}

// IsFraudError checks if the error indicates fraud.
func IsFraudError(err error) bool {
	e := GetIyzicoError(err)
	if e == nil {
		return false
	}
	fraudCodes := []string{"10034", "10041", "10043"}
	for _, code := range fraudCodes {
		if e.Code == code {
			return true
		}
	}
	return false
}

// IsUserError checks if the error is a user-recoverable error.
func IsUserError(err error) bool {
	e := GetIyzicoError(err)
	if e == nil {
		return false
	}
	userCodes := []string{"10051", "10005", "10012", "10054", "10057", "10084"}
	for _, code := range userCodes {
		if e.Code == code {
			return true
		}
	}
	return false
}

// MapErrorToUserMessage maps an Iyzico error to a user-friendly message.
func MapErrorToUserMessage(err error) string {
	e := GetIyzicoError(err)
	if e == nil {
		return "Ödeme işlemi sırasında bir hata oluştu."
	}

	switch e.Code {
	case "10051":
		return "Kart limiti yetersiz."
	case "10005":
		return "İşlem onaylanmadı."
	case "10012":
		return "Geçersiz kart numarası."
	case "10034", "10041", "10043":
		return "Ödeme işlemi gerçekleştirilemedi. Lütfen destek ile iletişime geçin."
	case "10054":
		return "Kartın son kullanma tarihi geçmiş."
	case "10057":
		return "Kart sahibi bu işlemi yapamaz."
	case "10084":
		return "Geçersiz CVV."
	default:
		if e.Message != "" {
			return e.Message
		}
		return "Ödeme işlemi sırasında bir hata oluştu."
	}
}
