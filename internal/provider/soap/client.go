// Package soap provides SOAP client utilities for integrating with
// Turkish cargo carriers that use SOAP-based APIs (Aras, Yurtiçi, Sürat, PTT, MNG).
package soap

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/knid/kargonet/internal/provider"
)

// Common SOAP errors.
var (
	// ErrSOAPFault indicates a SOAP fault was returned.
	ErrSOAPFault = errors.New("SOAP fault")
	// ErrInvalidResponse indicates the response could not be parsed.
	ErrInvalidResponse = errors.New("invalid SOAP response")
	// ErrEmptyResponse indicates an empty response was received.
	ErrEmptyResponse = errors.New("empty SOAP response")
)

// SOAPVersion represents the SOAP protocol version.
type SOAPVersion string

const (
	// SOAP11 is SOAP 1.1.
	SOAP11 SOAPVersion = "1.1"
	// SOAP12 is SOAP 1.2.
	SOAP12 SOAPVersion = "1.2"
)

// ContentType returns the content type for the SOAP version.
func (v SOAPVersion) ContentType() string {
	switch v {
	case SOAP12:
		return "application/soap+xml; charset=utf-8"
	default:
		return "text/xml; charset=utf-8"
	}
}

// Namespace returns the SOAP envelope namespace for the version.
func (v SOAPVersion) Namespace() string {
	switch v {
	case SOAP12:
		return "http://www.w3.org/2003/05/soap-envelope"
	default:
		return "http://schemas.xmlsoap.org/soap/envelope/"
	}
}

// ClientConfig contains configuration for the SOAP client.
type ClientConfig struct {
	// Endpoint is the SOAP service endpoint URL.
	Endpoint string
	// Timeout is the request timeout.
	Timeout time.Duration
	// SOAPVersion is the SOAP protocol version.
	SOAPVersion SOAPVersion
	// SOAPAction is the default SOAPAction header value.
	SOAPAction string
	// Headers are additional HTTP headers to include.
	Headers map[string]string
	// TLSConfig is the TLS configuration (optional).
	TLSConfig *tls.Config
	// SkipTLSVerify skips TLS certificate verification (not recommended for production).
	SkipTLSVerify bool
	// Logger is the logger instance.
	Logger *slog.Logger
	// MaxRetries is the maximum number of retries for transient errors.
	MaxRetries int
	// RetryBaseDelay is the base delay between retries.
	RetryBaseDelay time.Duration
}

// DefaultClientConfig returns the default client configuration.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:        30 * time.Second,
		SOAPVersion:    SOAP11,
		Headers:        make(map[string]string),
		MaxRetries:     3,
		RetryBaseDelay: 1 * time.Second,
	}
}

// Client is a SOAP client for making SOAP requests.
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new SOAP client.
func NewClient(config ClientConfig) *Client {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	transport := &http.Transport{
		TLSClientConfig:     config.TLSConfig,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	if config.SkipTLSVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		logger:     config.Logger,
	}
}

// Call makes a SOAP request and parses the response.
// The request and response parameters should be pointers to structs
// that can be marshaled/unmarshaled to/from XML.
func (c *Client) Call(ctx context.Context, action string, request, response interface{}) error {
	return c.CallWithEndpoint(ctx, c.config.Endpoint, action, request, response)
}

// CallWithEndpoint makes a SOAP request to a specific endpoint.
func (c *Client) CallWithEndpoint(ctx context.Context, endpoint, action string, request, response interface{}) error {
	// Build SOAP envelope once before the retry loop
	envelope, err := c.buildEnvelope(request)
	if err != nil {
		return fmt.Errorf("failed to build SOAP envelope: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateRetryDelay(attempt)
			c.logger.Debug("retrying SOAP request",
				slog.String("endpoint", endpoint),
				slog.String("action", action),
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.doSOAPRequest(ctx, endpoint, action, envelope, response)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if the error is retryable
		if !c.isRetryableError(err) {
			return err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	return errors.New("max retries exceeded")
}

// doSOAPRequest executes a single SOAP HTTP request.
func (c *Client) doSOAPRequest(ctx context.Context, endpoint, action string, envelope []byte, response interface{}) error {
	startTime := time.Now()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(envelope))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", c.config.SOAPVersion.ContentType())
	if action != "" {
		if c.config.SOAPVersion == SOAP12 {
			// SOAP 1.2 uses action parameter in Content-Type
			httpReq.Header.Set("Content-Type", fmt.Sprintf("%s; action=\"%s\"", c.config.SOAPVersion.ContentType(), action))
		} else {
			// SOAP 1.1 uses SOAPAction header
			httpReq.Header.Set("SOAPAction", action)
		}
	} else if c.config.SOAPAction != "" {
		httpReq.Header.Set("SOAPAction", c.config.SOAPAction)
	}

	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Log request
	c.logger.Debug("SOAP request",
		slog.String("endpoint", endpoint),
		slog.String("action", action),
		slog.Int("body_size", len(envelope)),
	)

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		latency := time.Since(startTime)
		c.logger.Error("SOAP request failed",
			slog.String("endpoint", endpoint),
			slog.String("action", action),
			slog.Duration("latency", latency),
			slog.String("error", err.Error()),
		)
		return c.mapHTTPError(err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	latency := time.Since(startTime)

	// Log response
	c.logger.Debug("SOAP response",
		slog.String("endpoint", endpoint),
		slog.String("action", action),
		slog.Int("status", httpResp.StatusCode),
		slog.Int("body_size", len(respBody)),
		slog.Duration("latency", latency),
	)

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		// Try to parse SOAP fault
		fault, faultErr := c.parseFault(respBody)
		if faultErr == nil && fault != nil {
			return fault.ToProviderError()
		}
		return &provider.ProviderError{
			Code:      "HTTP_ERROR",
			Message:   fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, httpResp.Status),
			Retryable: httpResp.StatusCode >= 500,
		}
	}

	// Check for empty response
	if len(respBody) == 0 {
		return ErrEmptyResponse
	}

	// Parse response
	if err := c.parseResponse(respBody, response); err != nil {
		return err
	}

	return nil
}

// calculateRetryDelay calculates the backoff delay for a retry attempt using exponential backoff with jitter.
func (c *Client) calculateRetryDelay(attempt int) time.Duration {
	delay := float64(c.config.RetryBaseDelay) * math.Pow(2.0, float64(attempt-1))

	// Add jitter (+-25%)
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter

	return time.Duration(delay)
}

// isRetryableError checks if an error is retryable (timeouts, connection errors, 5xx).
func (c *Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a provider error with retryable flag
	var providerErr *provider.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Retryable
	}

	// Check for transient network errors
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "EOF")
}

// CallRaw makes a raw SOAP request with a pre-built envelope.
func (c *Client) CallRaw(ctx context.Context, action string, envelope []byte) ([]byte, error) {
	return c.CallRawWithEndpoint(ctx, c.config.Endpoint, action, envelope)
}

// CallRawWithEndpoint makes a raw SOAP request to a specific endpoint.
func (c *Client) CallRawWithEndpoint(ctx context.Context, endpoint, action string, envelope []byte) ([]byte, error) {
	startTime := time.Now()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(envelope))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", c.config.SOAPVersion.ContentType())
	if action != "" {
		if c.config.SOAPVersion == SOAP12 {
			httpReq.Header.Set("Content-Type", fmt.Sprintf("%s; action=\"%s\"", c.config.SOAPVersion.ContentType(), action))
		} else {
			httpReq.Header.Set("SOAPAction", action)
		}
	}

	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		latency := time.Since(startTime)
		c.logger.Error("SOAP raw request failed",
			slog.String("endpoint", endpoint),
			slog.String("action", action),
			slog.Duration("latency", latency),
			slog.String("error", err.Error()),
		)
		return nil, c.mapHTTPError(err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	latency := time.Since(startTime)
	c.logger.Debug("SOAP raw response",
		slog.String("endpoint", endpoint),
		slog.String("action", action),
		slog.Int("status", httpResp.StatusCode),
		slog.Int("body_size", len(respBody)),
		slog.Duration("latency", latency),
	)

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		fault, faultErr := c.parseFault(respBody)
		if faultErr == nil && fault != nil {
			return nil, fault.ToProviderError()
		}
		return nil, &provider.ProviderError{
			Code:      "HTTP_ERROR",
			Message:   fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, httpResp.Status),
			Retryable: httpResp.StatusCode >= 500,
		}
	}

	return respBody, nil
}

// buildEnvelope builds a SOAP envelope from the request body.
func (c *Client) buildEnvelope(body interface{}) ([]byte, error) {
	envelope := &Envelope{
		XMLNSSoap: c.config.SOAPVersion.Namespace(),
		Body: Body{
			Content: body,
		},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	encoder := xml.NewEncoder(&buf)
	if err := encoder.Encode(envelope); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// parseResponse parses the SOAP response body.
func (c *Client) parseResponse(data []byte, response interface{}) error {
	// First, check for SOAP fault
	fault, err := c.parseFault(data)
	if err == nil && fault != nil {
		return fault.ToProviderError()
	}

	// Parse the response envelope
	envelope := &ResponseEnvelope{
		Body: ResponseBody{
			Content: response,
		},
	}

	if err := xml.Unmarshal(data, envelope); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	return nil
}

// parseFault attempts to parse a SOAP fault from the response.
func (c *Client) parseFault(data []byte) (*Fault, error) {
	// Try SOAP 1.1 fault
	var envelope11 struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault *Fault `xml:"Fault"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(data, &envelope11); err == nil && envelope11.Body.Fault != nil {
		return envelope11.Body.Fault, nil
	}

	// Try SOAP 1.2 fault
	var envelope12 struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault *Fault12 `xml:"Fault"`
		} `xml:"Body"`
	}

	if err := xml.Unmarshal(data, &envelope12); err == nil && envelope12.Body.Fault != nil {
		return envelope12.Body.Fault.ToFault(), nil
	}

	return nil, nil
}

// mapHTTPError maps HTTP errors to provider errors.
func (c *Client) mapHTTPError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return &provider.ProviderError{
			Code:      "PROVIDER_TIMEOUT",
			Message:   "request timed out",
			Retryable: true,
			Cause:     err,
		}
	}

	if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") {
		return &provider.ProviderError{
			Code:      "PROVIDER_DOWN",
			Message:   "provider is unavailable",
			Retryable: true,
			Cause:     err,
		}
	}

	return &provider.ProviderError{
		Code:      "PROVIDER_ERROR",
		Message:   err.Error(),
		Retryable: true,
		Cause:     err,
	}
}

// SetEndpoint updates the client endpoint.
func (c *Client) SetEndpoint(endpoint string) {
	c.config.Endpoint = endpoint
}

// SetTimeout updates the client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.config.Timeout = timeout
	c.httpClient.Timeout = timeout
}

// SetHeader sets a custom HTTP header.
func (c *Client) SetHeader(key, value string) {
	c.config.Headers[key] = value
}

// Envelope represents a SOAP envelope for requests.
type Envelope struct {
	XMLName   xml.Name `xml:"soap:Envelope"`
	XMLNSSoap string   `xml:"xmlns:soap,attr"`
	Header    *Header  `xml:"soap:Header,omitempty"`
	Body      Body     `xml:"soap:Body"`
}

// Header represents a SOAP header.
type Header struct {
	Content interface{} `xml:",any"`
}

// Body represents a SOAP body for requests.
type Body struct {
	Content interface{} `xml:",any"`
}

// ResponseEnvelope represents a SOAP envelope for responses.
type ResponseEnvelope struct {
	XMLName xml.Name     `xml:"Envelope"`
	Header  *Header      `xml:"Header,omitempty"`
	Body    ResponseBody `xml:"Body"`
}

// ResponseBody represents a SOAP body for responses.
type ResponseBody struct {
	Content interface{} `xml:",any"`
}

// Fault represents a SOAP 1.1 fault.
type Fault struct {
	XMLName     xml.Name `xml:"Fault"`
	FaultCode   string   `xml:"faultcode"`
	FaultString string   `xml:"faultstring"`
	FaultActor  string   `xml:"faultactor,omitempty"`
	Detail      string   `xml:"detail,omitempty"`
}

// Error implements the error interface.
func (f *Fault) Error() string {
	return fmt.Sprintf("SOAP Fault: %s - %s", f.FaultCode, f.FaultString)
}

// ToProviderError converts the fault to a ProviderError.
func (f *Fault) ToProviderError() *provider.ProviderError {
	return &provider.ProviderError{
		Code:            "PROVIDER_REJECTED",
		Message:         f.FaultString,
		ProviderCode:    f.FaultCode,
		ProviderMessage: f.Detail,
		Retryable:       false,
	}
}

// Fault12 represents a SOAP 1.2 fault.
type Fault12 struct {
	XMLName xml.Name `xml:"Fault"`
	Code    struct {
		Value   string `xml:"Value"`
		Subcode *struct {
			Value string `xml:"Value"`
		} `xml:"Subcode,omitempty"`
	} `xml:"Code"`
	Reason struct {
		Text string `xml:"Text"`
	} `xml:"Reason"`
	Detail string `xml:"Detail,omitempty"`
}

// ToFault converts a SOAP 1.2 fault to a SOAP 1.1 fault.
func (f *Fault12) ToFault() *Fault {
	faultCode := f.Code.Value
	if f.Code.Subcode != nil {
		faultCode = f.Code.Subcode.Value
	}
	return &Fault{
		FaultCode:   faultCode,
		FaultString: f.Reason.Text,
		Detail:      f.Detail,
	}
}

// BasicAuth represents HTTP Basic Authentication credentials.
type BasicAuth struct {
	Username string
	Password string
}

// WSSecurity represents WS-Security credentials for SOAP headers.
type WSSecurity struct {
	Username string
	Password string
	Nonce    string
	Created  time.Time
}

// ToHeader converts WSSecurity to a SOAP header element.
func (ws *WSSecurity) ToHeader() interface{} {
	return &wsSecurityHeader{
		XMLNSWsse:      "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd",
		XMLNSWsu:       "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd",
		MustUnderstand: "1",
		UsernameToken: wsUsernameToken{
			Username: ws.Username,
			Password: wsPassword{
				Type:  "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText",
				Value: ws.Password,
			},
		},
	}
}

type wsSecurityHeader struct {
	XMLName        xml.Name `xml:"wsse:Security"`
	XMLNSWsse      string   `xml:"xmlns:wsse,attr"`
	XMLNSWsu       string   `xml:"xmlns:wsu,attr"`
	MustUnderstand string   `xml:"soap:mustUnderstand,attr"`
	UsernameToken  wsUsernameToken
}

type wsUsernameToken struct {
	XMLName  xml.Name   `xml:"wsse:UsernameToken"`
	Username string     `xml:"wsse:Username"`
	Password wsPassword `xml:"wsse:Password"`
}

type wsPassword struct {
	Type  string `xml:"Type,attr"`
	Value string `xml:",chardata"`
}
