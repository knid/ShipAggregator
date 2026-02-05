package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HMAC constants
const (
	// SignaturePrefix is the version prefix for webhook signatures
	SignaturePrefix = "v1="
	// DefaultTimestampTolerance is the default tolerance for timestamp validation (5 minutes)
	DefaultTimestampTolerance = 5 * time.Minute
)

// HMAC errors
var (
	// ErrInvalidSignature is returned when signature verification fails
	ErrInvalidSignature = errors.New("crypto: invalid signature")
	// ErrTimestampExpired is returned when the timestamp is outside the tolerance window
	ErrTimestampExpired = errors.New("crypto: timestamp expired or too far in future")
	// ErrInvalidSignatureFormat is returned when the signature format is invalid
	ErrInvalidSignatureFormat = errors.New("crypto: invalid signature format")
	// ErrEmptySecret is returned when the secret is empty
	ErrEmptySecret = errors.New("crypto: secret cannot be empty")
)

// HMACSHA256 computes HMAC-SHA256 of the message using the given key.
// Returns the raw bytes of the HMAC.
func HMACSHA256(key, message []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}

// HMACSHA256Hex computes HMAC-SHA256 and returns the hex-encoded result.
func HMACSHA256Hex(key, message []byte) string {
	return hex.EncodeToString(HMACSHA256(key, message))
}

// WebhookSigner provides webhook signing and verification operations.
type WebhookSigner struct {
	secret             []byte
	timestampTolerance time.Duration
}

// NewWebhookSigner creates a new webhook signer with the given secret.
func NewWebhookSigner(secret string) (*WebhookSigner, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &WebhookSigner{
		secret:             []byte(secret),
		timestampTolerance: DefaultTimestampTolerance,
	}, nil
}

// NewWebhookSignerWithTolerance creates a new webhook signer with custom timestamp tolerance.
func NewWebhookSignerWithTolerance(secret string, tolerance time.Duration) (*WebhookSigner, error) {
	signer, err := NewWebhookSigner(secret)
	if err != nil {
		return nil, err
	}
	signer.timestampTolerance = tolerance
	return signer, nil
}

// Sign generates a webhook signature for the given payload and timestamp.
// The signature format is: v1=<hex-encoded-hmac>
// The signed message is: <timestamp>.<payload>
func (s *WebhookSigner) Sign(payload []byte, timestamp int64) string {
	prefix := []byte(fmt.Sprintf("%d.", timestamp))
	message := append(prefix, payload...)
	signature := HMACSHA256Hex(s.secret, message)
	return SignaturePrefix + signature
}

// SignWithCurrentTime generates a webhook signature using the current Unix timestamp.
// Returns the signature and the timestamp used.
func (s *WebhookSigner) SignWithCurrentTime(payload []byte) (signature string, timestamp int64) {
	timestamp = time.Now().Unix()
	signature = s.Sign(payload, timestamp)
	return signature, timestamp
}

// Verify verifies a webhook signature against the payload and timestamp.
// Returns nil if the signature is valid, or an error describing the failure.
func (s *WebhookSigner) Verify(payload []byte, signatureHeader string, timestamp int64) error {
	// Check timestamp is within tolerance
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(s.timestampTolerance.Seconds()) {
		return ErrTimestampExpired
	}

	// Parse signature header
	if !strings.HasPrefix(signatureHeader, SignaturePrefix) {
		return ErrInvalidSignatureFormat
	}
	providedSignature := signatureHeader[len(SignaturePrefix):]

	// Compute expected signature
	expectedSignature := s.Sign(payload, timestamp)[len(SignaturePrefix):]

	// Constant-time comparison to prevent timing attacks
	if !constantTimeCompare(expectedSignature, providedSignature) {
		return ErrInvalidSignature
	}

	return nil
}

// VerifyWithHeaders verifies a webhook using the standard header format.
// signatureHeader is the X-Webhook-Signature header value.
// timestampHeader is the X-Webhook-Timestamp header value.
func (s *WebhookSigner) VerifyWithHeaders(payload []byte, signatureHeader, timestampHeader string) error {
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("crypto: invalid timestamp header: %w", err)
	}
	return s.Verify(payload, signatureHeader, timestamp)
}

// constantTimeCompare performs a constant-time string comparison.
func constantTimeCompare(a, b string) bool {
	// Decode hex strings to bytes for comparison
	aBytes, err := hex.DecodeString(a)
	if err != nil {
		return false
	}
	bBytes, err := hex.DecodeString(b)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(aBytes, bBytes) == 1
}

// SignWebhook is a convenience function for signing webhooks.
// It creates a signature in the format: v1=<hex-encoded-hmac>
// The signed message is: <timestamp>.<payload>
func SignWebhook(payload []byte, secret string, timestamp int64) string {
	prefix := []byte(fmt.Sprintf("%d.", timestamp))
	message := append(prefix, payload...)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(message)
	return fmt.Sprintf("v1=%x", mac.Sum(nil))
}

// VerifyWebhookSignature verifies a webhook signature.
// This is a convenience function that creates a temporary signer.
// For repeated verifications, use WebhookSigner directly.
func VerifyWebhookSignature(
	payload []byte,
	signatureHeader string,
	timestampHeader string,
	secret string,
	toleranceSeconds int,
) error {
	tolerance := time.Duration(toleranceSeconds) * time.Second
	signer, err := NewWebhookSignerWithTolerance(secret, tolerance)
	if err != nil {
		return err
	}
	return signer.VerifyWithHeaders(payload, signatureHeader, timestampHeader)
}

// WebhookHeaders represents the standard webhook headers.
type WebhookHeaders struct {
	// ID is the unique identifier for the webhook event (X-Webhook-Id)
	ID string
	// Timestamp is the Unix timestamp when the webhook was sent (X-Webhook-Timestamp)
	Timestamp int64
	// Signature is the HMAC-SHA256 signature (X-Webhook-Signature)
	Signature string
}

// GenerateWebhookHeaders generates all required webhook headers for a payload.
func (s *WebhookSigner) GenerateWebhookHeaders(webhookID string, payload []byte) WebhookHeaders {
	timestamp := time.Now().Unix()
	signature := s.Sign(payload, timestamp)

	return WebhookHeaders{
		ID:        webhookID,
		Timestamp: timestamp,
		Signature: signature,
	}
}

// ComputeRequestHash computes a SHA-256 hash of the request body.
// This is used for idempotency key validation.
func ComputeRequestHash(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// HashAPIKey computes the SHA-256 hash of an API key for storage.
// API keys should never be stored in plaintext.
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// CompareAPIKeyHash compares an API key against its stored hash.
// Uses constant-time comparison to prevent timing attacks.
func CompareAPIKeyHash(apiKey, storedHash string) bool {
	computedHash := HashAPIKey(apiKey)
	return constantTimeCompareStrings(computedHash, storedHash)
}

// constantTimeCompareStrings performs constant-time comparison of two strings.
func constantTimeCompareStrings(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
