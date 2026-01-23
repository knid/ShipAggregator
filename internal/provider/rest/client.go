// Package rest provides REST client utilities for integrating with
// Turkish cargo carriers that use REST-based APIs (Hepsijet, Kolaygelsin, Sendeo, Pakettaxi).
package rest

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/knid/kargonet/internal/provider"
)

// Common REST errors.
var (
	// ErrInvalidResponse indicates the response could not be parsed.
	ErrInvalidResponse = errors.New("invalid response")
	// ErrEmptyResponse indicates an empty response was received.
	ErrEmptyResponse = errors.New("empty response")
	// ErrMaxRetriesExceeded indicates the maximum number of retries was exceeded.
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

// ClientConfig contains configuration for the REST client.
type ClientConfig struct {
	// BaseURL is the API base URL.
	BaseURL string
	// Timeout is the request timeout.
	Timeout time.Duration
	// Headers are default HTTP headers to include.
	Headers map[string]string
	// TLSConfig is the TLS configuration (optional).
	TLSConfig *tls.Config
	// SkipTLSVerify skips TLS certificate verification (not recommended for production).
	SkipTLSVerify bool
	// Logger is the logger instance.
	Logger *slog.Logger

	// Retry configuration
	// MaxRetries is the maximum number of retries.
	MaxRetries int
	// RetryBaseDelay is the base delay between retries.
	RetryBaseDelay time.Duration
	// RetryMaxDelay is the maximum delay between retries.
	RetryMaxDelay time.Duration
	// RetryMultiplier is the backoff multiplier.
	RetryMultiplier float64

	// Circuit breaker configuration
	// CircuitBreakerEnabled enables the circuit breaker.
	CircuitBreakerEnabled bool
	// CircuitBreakerFailureThreshold is the number of failures before opening.
	CircuitBreakerFailureThreshold int
	// CircuitBreakerSuccessThreshold is the number of successes before closing.
	CircuitBreakerSuccessThreshold int
	// CircuitBreakerTimeout is the time in open state before half-open.
	CircuitBreakerTimeout time.Duration
}

// DefaultClientConfig returns the default client configuration.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		Timeout:                        30 * time.Second,
		Headers:                        make(map[string]string),
		MaxRetries:                     3,
		RetryBaseDelay:                 1 * time.Second,
		RetryMaxDelay:                  10 * time.Second,
		RetryMultiplier:                2.0,
		CircuitBreakerEnabled:          true,
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerSuccessThreshold: 2,
		CircuitBreakerTimeout:          30 * time.Second,
	}
}

// Client is a REST client with retry and circuit breaker support.
type Client struct {
	config         ClientConfig
	httpClient     *http.Client
	circuitBreaker *CircuitBreaker
	logger         *slog.Logger
}

// NewClient creates a new REST client.
func NewClient(config ClientConfig) *Client {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	transport := &http.Transport{
		TLSClientConfig: config.TLSConfig,
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

	var cb *CircuitBreaker
	if config.CircuitBreakerEnabled {
		cb = NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: config.CircuitBreakerFailureThreshold,
			SuccessThreshold: config.CircuitBreakerSuccessThreshold,
			Timeout:          config.CircuitBreakerTimeout,
		})
	}

	return &Client{
		config:         config,
		httpClient:     httpClient,
		circuitBreaker: cb,
		logger:         config.Logger,
	}
}

// Request represents an HTTP request.
type Request struct {
	Method      string
	Path        string
	Query       url.Values
	Headers     map[string]string
	Body        interface{}
	ContentType string
}

// Response represents an HTTP response.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Do executes an HTTP request with retry and circuit breaker.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	// Check circuit breaker
	if c.circuitBreaker != nil {
		if err := c.circuitBreaker.Allow(); err != nil {
			return nil, err
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			c.logger.Debug("retrying request",
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.doRequest(ctx, req)
		if err != nil {
			lastErr = err
			if !c.isRetryable(err) {
				if c.circuitBreaker != nil {
					c.circuitBreaker.RecordFailure()
				}
				return nil, err
			}
			continue
		}

		// Check for retryable status codes
		if c.isRetryableStatus(resp.StatusCode) {
			lastErr = &provider.ProviderError{
				Code:      "HTTP_ERROR",
				Message:   fmt.Sprintf("HTTP %d", resp.StatusCode),
				Retryable: true,
			}
			continue
		}

		if c.circuitBreaker != nil {
			c.circuitBreaker.RecordSuccess()
		}

		return resp, nil
	}

	if c.circuitBreaker != nil {
		c.circuitBreaker.RecordFailure()
	}

	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
	}
	return nil, ErrMaxRetriesExceeded
}

// doRequest executes a single HTTP request.
func (c *Client) doRequest(ctx context.Context, req Request) (*Response, error) {
	startTime := time.Now()

	// Build URL
	reqURL := c.config.BaseURL + req.Path
	if len(req.Query) > 0 {
		reqURL += "?" + req.Query.Encode()
	}

	// Build body
	var bodyReader io.Reader
	if req.Body != nil {
		switch v := req.Body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = strings.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			jsonBody, err := json.Marshal(req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBody)
			if req.ContentType == "" {
				req.ContentType = "application/json"
			}
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	for key, value := range c.config.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set request headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set content type
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}

	// Log request
	c.logger.Debug("REST request",
		slog.String("method", req.Method),
		slog.String("url", reqURL),
	)

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		latency := time.Since(startTime)
		c.logger.Error("REST request failed",
			slog.String("method", req.Method),
			slog.String("url", reqURL),
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

	// Log response
	c.logger.Debug("REST response",
		slog.String("method", req.Method),
		slog.String("url", reqURL),
		slog.Int("status", httpResp.StatusCode),
		slog.Int("body_size", len(respBody)),
		slog.Duration("latency", latency),
	)

	return &Response{
		StatusCode: httpResp.StatusCode,
		Headers:    httpResp.Header,
		Body:       respBody,
	}, nil
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, query url.Values, response interface{}) error {
	resp, err := c.Do(ctx, Request{
		Method: http.MethodGet,
		Path:   path,
		Query:  query,
	})
	if err != nil {
		return err
	}

	return c.handleResponse(resp, response)
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body, response interface{}) error {
	resp, err := c.Do(ctx, Request{
		Method: http.MethodPost,
		Path:   path,
		Body:   body,
	})
	if err != nil {
		return err
	}

	return c.handleResponse(resp, response)
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body, response interface{}) error {
	resp, err := c.Do(ctx, Request{
		Method: http.MethodPut,
		Path:   path,
		Body:   body,
	})
	if err != nil {
		return err
	}

	return c.handleResponse(resp, response)
}

// Patch performs a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body, response interface{}) error {
	resp, err := c.Do(ctx, Request{
		Method: http.MethodPatch,
		Path:   path,
		Body:   body,
	})
	if err != nil {
		return err
	}

	return c.handleResponse(resp, response)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, response interface{}) error {
	resp, err := c.Do(ctx, Request{
		Method: http.MethodDelete,
		Path:   path,
	})
	if err != nil {
		return err
	}

	return c.handleResponse(resp, response)
}

// handleResponse handles the HTTP response.
func (c *Client) handleResponse(resp *Response, response interface{}) error {
	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return c.parseErrorResponse(resp)
	}

	// Parse response body
	if response != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, response); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
		}
	}

	return nil
}

// parseErrorResponse parses an error response.
func (c *Client) parseErrorResponse(resp *Response) error {
	// Try to parse as JSON error
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}

	if len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			message := errResp.Message
			if message == "" {
				message = errResp.Error
			}
			if message == "" {
				message = string(resp.Body)
			}

			return &provider.ProviderError{
				Code:            c.mapStatusToCode(resp.StatusCode),
				Message:         message,
				ProviderCode:    errResp.Code,
				ProviderMessage: message,
				Retryable:       resp.StatusCode >= 500,
			}
		}
	}

	return &provider.ProviderError{
		Code:      c.mapStatusToCode(resp.StatusCode),
		Message:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		Retryable: resp.StatusCode >= 500,
	}
}

// mapStatusToCode maps HTTP status codes to error codes.
func (c *Client) mapStatusToCode(status int) string {
	switch {
	case status == 401:
		return "PROVIDER_AUTH_FAILED"
	case status == 403:
		return "PROVIDER_FORBIDDEN"
	case status == 404:
		return "PROVIDER_NOT_FOUND"
	case status == 429:
		return "PROVIDER_RATE_LIMITED"
	case status >= 500:
		return "PROVIDER_DOWN"
	default:
		return "PROVIDER_REJECTED"
	}
}

// calculateBackoff calculates the backoff delay for a retry attempt.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	delay := float64(c.config.RetryBaseDelay) * math.Pow(c.config.RetryMultiplier, float64(attempt-1))
	if delay > float64(c.config.RetryMaxDelay) {
		delay = float64(c.config.RetryMaxDelay)
	}

	// Add jitter (±25%)
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter

	return time.Duration(delay)
}

// isRetryable checks if an error is retryable.
func (c *Client) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	var providerErr *provider.ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Retryable
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "EOF")
}

// isRetryableStatus checks if an HTTP status code is retryable.
func (c *Client) isRetryableStatus(status int) bool {
	return status == 429 || status >= 500
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

// SetBaseURL updates the client base URL.
func (c *Client) SetBaseURL(baseURL string) {
	c.config.BaseURL = baseURL
}

// SetTimeout updates the client timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.config.Timeout = timeout
	c.httpClient.Timeout = timeout
}

// SetHeader sets a default HTTP header.
func (c *Client) SetHeader(key, value string) {
	c.config.Headers[key] = value
}

// SetBearerToken sets the Authorization header with a bearer token.
func (c *Client) SetBearerToken(token string) {
	c.SetHeader("Authorization", "Bearer "+token)
}

// SetBasicAuth sets the Authorization header with basic auth.
func (c *Client) SetBasicAuth(username, password string) {
	c.SetHeader("Authorization", "Basic "+basicAuth(username, password))
}

// SetAPIKey sets an API key header.
func (c *Client) SetAPIKey(headerName, apiKey string) {
	c.SetHeader(headerName, apiKey)
}

// basicAuth encodes basic auth credentials.
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode([]byte(auth))
}

// base64Encode encodes bytes to base64.
func base64Encode(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder
	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte(base64Chars[(n>>6)&0x3F])
			result.WriteByte(base64Chars[n&0x3F])
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte(base64Chars[(n>>6)&0x3F])
			result.WriteByte('=')
		} else {
			n = uint32(data[i]) << 16
			result.WriteByte(base64Chars[(n>>18)&0x3F])
			result.WriteByte(base64Chars[(n>>12)&0x3F])
			result.WriteByte('=')
			result.WriteByte('=')
		}
	}
	return result.String()
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitBreakerClosed allows requests to pass through.
	CircuitBreakerClosed CircuitBreakerState = iota
	// CircuitBreakerOpen blocks all requests.
	CircuitBreakerOpen
	// CircuitBreakerHalfOpen allows limited requests to test recovery.
	CircuitBreakerHalfOpen
)

// String returns the string representation of the state.
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig contains configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening.
	FailureThreshold int
	// SuccessThreshold is the number of successes before closing.
	SuccessThreshold int
	// Timeout is the time in open state before half-open.
	Timeout time.Duration
}

// DefaultCircuitBreakerConfig returns the default circuit breaker configuration.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	mu              sync.RWMutex
	state           CircuitBreakerState
	failures        int
	successes       int
	lastFailureTime time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
	}
}

// Allow checks if a request is allowed.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return nil

	case CircuitBreakerOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			cb.state = CircuitBreakerHalfOpen
			cb.successes = 0
			return nil
		}
		return provider.ErrCircuitOpen

	case CircuitBreakerHalfOpen:
		return nil

	default:
		return nil
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		cb.failures = 0

	case CircuitBreakerHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = CircuitBreakerClosed
			cb.failures = 0
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitBreakerOpen
		}

	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerOpen
		cb.successes = 0
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitBreakerClosed
	cb.failures = 0
	cb.successes = 0
}

// Metrics returns the current metrics of the circuit breaker.
func (cb *CircuitBreaker) Metrics() CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitBreakerMetrics{
		State:           cb.state,
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailureTime: cb.lastFailureTime,
	}
}

// CircuitBreakerMetrics contains metrics for the circuit breaker.
type CircuitBreakerMetrics struct {
	State           CircuitBreakerState
	Failures        int
	Successes       int
	LastFailureTime time.Time
}

// RateLimiter implements a simple rate limiter using token bucket algorithm.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// Wait waits until a token is available.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1/rl.refillRate) * time.Second):
		}
	}
}

// refill refills tokens based on elapsed time.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now
}

// Tokens returns the current number of tokens.
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}
