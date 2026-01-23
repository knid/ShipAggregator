// Package handler provides HTTP handlers for the shipping aggregator API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/domain"
)

// ProblemDetails represents an RFC 7807 Problem Details response.
// See: https://tools.ietf.org/html/rfc7807
type ProblemDetails struct {
	// Type is a URI reference that identifies the problem type.
	// When dereferenced, it should provide human-readable documentation.
	Type string `json:"type"`

	// Title is a short, human-readable summary of the problem type.
	// It should not change from occurrence to occurrence.
	Title string `json:"title"`

	// Status is the HTTP status code for this occurrence of the problem.
	Status int `json:"status"`

	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`

	// Instance is a URI reference that identifies the specific occurrence.
	Instance string `json:"instance,omitempty"`

	// Code is an application-specific error code.
	Code string `json:"code,omitempty"`

	// TraceID is the trace ID for debugging.
	TraceID string `json:"trace_id,omitempty"`

	// RequestID is the request ID for debugging.
	RequestID string `json:"request_id,omitempty"`

	// Errors contains validation errors or multiple error details.
	Errors []ErrorDetail `json:"errors,omitempty"`

	// Extensions contains additional problem-specific data.
	Extensions map[string]interface{} `json:"-"`
}

// ErrorDetail represents a single error detail, typically for validation errors.
type ErrorDetail struct {
	// Field is the field that caused the error (for validation errors).
	Field string `json:"field,omitempty"`

	// Code is the error code for this specific error.
	Code string `json:"code"`

	// Message is a human-readable description of the error.
	Message string `json:"message"`

	// Value is the invalid value that caused the error.
	Value interface{} `json:"value,omitempty"`
}

// MarshalJSON implements custom JSON marshaling to include extensions.
func (p *ProblemDetails) MarshalJSON() ([]byte, error) {
	// Create a map with all standard fields
	m := map[string]interface{}{
		"type":   p.Type,
		"title":  p.Title,
		"status": p.Status,
	}

	if p.Detail != "" {
		m["detail"] = p.Detail
	}
	if p.Instance != "" {
		m["instance"] = p.Instance
	}
	if p.Code != "" {
		m["code"] = p.Code
	}
	if p.TraceID != "" {
		m["trace_id"] = p.TraceID
	}
	if p.RequestID != "" {
		m["request_id"] = p.RequestID
	}
	if len(p.Errors) > 0 {
		m["errors"] = p.Errors
	}

	// Add extensions
	for k, v := range p.Extensions {
		m[k] = v
	}

	return json.Marshal(m)
}

// Error implements the error interface.
func (p *ProblemDetails) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// Common error types as URI references.
const (
	ErrorTypeBase = "https://api.kargonet.com/errors/"

	// Authentication and authorization errors
	ErrorTypeUnauthorized    = ErrorTypeBase + "unauthorized"
	ErrorTypeForbidden       = ErrorTypeBase + "forbidden"
	ErrorTypeInvalidAPIKey   = ErrorTypeBase + "invalid-api-key"
	ErrorTypeExpiredAPIKey   = ErrorTypeBase + "expired-api-key"
	ErrorTypeRevokedAPIKey   = ErrorTypeBase + "revoked-api-key"
	ErrorTypeSessionExpired  = ErrorTypeBase + "session-expired"
	ErrorTypeIPNotAllowed    = ErrorTypeBase + "ip-not-allowed"
	ErrorTypeMemberSuspended = ErrorTypeBase + "member-suspended"
	ErrorTypeOrgSuspended    = ErrorTypeBase + "organization-suspended"

	// Resource errors
	ErrorTypeNotFound = ErrorTypeBase + "not-found"
	ErrorTypeConflict = ErrorTypeBase + "conflict"
	ErrorTypeGone     = ErrorTypeBase + "gone"

	// Validation errors
	ErrorTypeValidation     = ErrorTypeBase + "validation-error"
	ErrorTypeInvalidRequest = ErrorTypeBase + "invalid-request"
	ErrorTypeMissingField   = ErrorTypeBase + "missing-field"
	ErrorTypeInvalidField   = ErrorTypeBase + "invalid-field"

	// Rate limiting errors
	ErrorTypeRateLimitExceeded = ErrorTypeBase + "rate-limit-exceeded"

	// Idempotency errors
	ErrorTypeIdempotencyKeyReused  = ErrorTypeBase + "idempotency-key-reused"
	ErrorTypeIdempotencyKeyInUse   = ErrorTypeBase + "idempotency-key-in-use"
	ErrorTypeInvalidIdempotencyKey = ErrorTypeBase + "invalid-idempotency-key"

	// Server errors
	ErrorTypeInternalError      = ErrorTypeBase + "internal-error"
	ErrorTypeServiceUnavailable = ErrorTypeBase + "service-unavailable"
	ErrorTypeTimeout            = ErrorTypeBase + "timeout"

	// Business logic errors
	ErrorTypeInsufficientBalance = ErrorTypeBase + "insufficient-balance"
	ErrorTypeQuoteExpired        = ErrorTypeBase + "quote-expired"
	ErrorTypeShipmentCancelled   = ErrorTypeBase + "shipment-cancelled"
	ErrorTypeProviderError       = ErrorTypeBase + "provider-error"
)

// Common error codes.
const (
	CodeUnauthorized        = "AUTH_INVALID"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
	CodeInsufficientBalance = "INSUFFICIENT_BALANCE"
	CodeQuoteExpired        = "QUOTE_EXPIRED"
	CodeProviderError       = "PROVIDER_ERROR"
	CodeMethodNotAllowed    = "METHOD_NOT_ALLOWED"
)

// WriteError writes a ProblemDetails error response.
func WriteError(w http.ResponseWriter, r *http.Request, problem *ProblemDetails) {
	// Add trace and request IDs
	if problem.TraceID == "" {
		problem.TraceID = middleware.GetTraceID(r.Context())
	}
	if problem.RequestID == "" {
		problem.RequestID = middleware.GetRequestID(r.Context())
	}

	// Set instance to the request path
	if problem.Instance == "" {
		problem.Instance = r.URL.Path
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

// WriteErrorFromDomain converts a domain error to a ProblemDetails response.
func WriteErrorFromDomain(w http.ResponseWriter, r *http.Request, err error) {
	problem := DomainErrorToProblem(err)
	WriteError(w, r, problem)
}

// DomainErrorToProblem converts a domain error to a ProblemDetails.
func DomainErrorToProblem(err error) *ProblemDetails {
	// Check for specific domain errors
	switch {
	// Authentication errors
	case errors.Is(err, domain.ErrUnauthorized):
		return &ProblemDetails{
			Type:   ErrorTypeUnauthorized,
			Title:  "Unauthorized",
			Status: http.StatusUnauthorized,
			Detail: "Authentication is required to access this resource",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrAPIKeyNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeInvalidAPIKey,
			Title:  "Invalid API Key",
			Status: http.StatusUnauthorized,
			Detail: "The provided API key is invalid",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrAPIKeyExpired):
		return &ProblemDetails{
			Type:   ErrorTypeExpiredAPIKey,
			Title:  "API Key Expired",
			Status: http.StatusUnauthorized,
			Detail: "The API key has expired",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrAPIKeyRevoked):
		return &ProblemDetails{
			Type:   ErrorTypeRevokedAPIKey,
			Title:  "API Key Revoked",
			Status: http.StatusUnauthorized,
			Detail: "The API key has been revoked",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrAPIKeyInactive):
		return &ProblemDetails{
			Type:   ErrorTypeInvalidAPIKey,
			Title:  "API Key Inactive",
			Status: http.StatusUnauthorized,
			Detail: "The API key is inactive",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrAPIKeyIPNotAllowed):
		return &ProblemDetails{
			Type:   ErrorTypeIPNotAllowed,
			Title:  "IP Not Allowed",
			Status: http.StatusForbidden,
			Detail: "Your IP address is not allowed to use this API key",
			Code:   CodeForbidden,
		}
	case errors.Is(err, domain.ErrSessionNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeUnauthorized,
			Title:  "Session Not Found",
			Status: http.StatusUnauthorized,
			Detail: "The session is invalid or has been terminated",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrSessionExpired):
		return &ProblemDetails{
			Type:   ErrorTypeSessionExpired,
			Title:  "Session Expired",
			Status: http.StatusUnauthorized,
			Detail: "Your session has expired. Please log in again.",
			Code:   CodeUnauthorized,
		}
	case errors.Is(err, domain.ErrMemberSuspended):
		return &ProblemDetails{
			Type:   ErrorTypeMemberSuspended,
			Title:  "Account Suspended",
			Status: http.StatusForbidden,
			Detail: "Your account has been suspended",
			Code:   CodeForbidden,
		}
	case errors.Is(err, domain.ErrMemberRemoved):
		return &ProblemDetails{
			Type:   ErrorTypeForbidden,
			Title:  "Account Removed",
			Status: http.StatusForbidden,
			Detail: "Your account has been removed from the organization",
			Code:   CodeForbidden,
		}

	// Resource errors
	case errors.Is(err, domain.ErrOrganizationNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeNotFound,
			Title:  "Organization Not Found",
			Status: http.StatusNotFound,
			Detail: "The organization was not found",
			Code:   CodeNotFound,
		}
	case errors.Is(err, domain.ErrShipmentNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeNotFound,
			Title:  "Shipment Not Found",
			Status: http.StatusNotFound,
			Detail: "The shipment was not found",
			Code:   CodeNotFound,
		}
	case errors.Is(err, domain.ErrQuoteNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeNotFound,
			Title:  "Quote Not Found",
			Status: http.StatusNotFound,
			Detail: "The quote was not found",
			Code:   CodeNotFound,
		}
	case errors.Is(err, domain.ErrAddressNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeNotFound,
			Title:  "Address Not Found",
			Status: http.StatusNotFound,
			Detail: "The address was not found",
			Code:   CodeNotFound,
		}
	case errors.Is(err, domain.ErrWebhookNotFound):
		return &ProblemDetails{
			Type:   ErrorTypeNotFound,
			Title:  "Webhook Not Found",
			Status: http.StatusNotFound,
			Detail: "The webhook endpoint was not found",
			Code:   CodeNotFound,
		}

	// Business logic errors
	case errors.Is(err, domain.ErrQuoteExpired):
		return &ProblemDetails{
			Type:   ErrorTypeQuoteExpired,
			Title:  "Quote Expired",
			Status: http.StatusGone,
			Detail: "The quote has expired. Please request a new quote.",
			Code:   CodeQuoteExpired,
		}
	case errors.Is(err, domain.ErrInsufficientBalance):
		return &ProblemDetails{
			Type:   ErrorTypeInsufficientBalance,
			Title:  "Insufficient Balance",
			Status: http.StatusPaymentRequired,
			Detail: "Your account balance is insufficient for this operation",
			Code:   CodeInsufficientBalance,
		}
	case errors.Is(err, domain.ErrShipmentAlreadyCancelled):
		return &ProblemDetails{
			Type:   ErrorTypeShipmentCancelled,
			Title:  "Shipment Already Cancelled",
			Status: http.StatusConflict,
			Detail: "The shipment has already been cancelled",
			Code:   CodeConflict,
		}
	case errors.Is(err, domain.ErrShipmentNotCancellable):
		return &ProblemDetails{
			Type:   ErrorTypeConflict,
			Title:  "Shipment Not Cancellable",
			Status: http.StatusConflict,
			Detail: "The shipment cannot be cancelled in its current state",
			Code:   CodeConflict,
		}

	// Validation errors
	case errors.Is(err, domain.ErrValidation):
		return &ProblemDetails{
			Type:   ErrorTypeValidation,
			Title:  "Validation Error",
			Status: http.StatusBadRequest,
			Detail: err.Error(),
			Code:   CodeValidationError,
		}

	// Provider errors
	case errors.Is(err, domain.ErrProviderUnavailable):
		return &ProblemDetails{
			Type:   ErrorTypeProviderError,
			Title:  "Provider Unavailable",
			Status: http.StatusServiceUnavailable,
			Detail: "The shipping provider is currently unavailable",
			Code:   CodeProviderError,
		}
	case errors.Is(err, domain.ErrProviderError):
		return &ProblemDetails{
			Type:   ErrorTypeProviderError,
			Title:  "Provider Error",
			Status: http.StatusBadGateway,
			Detail: "An error occurred while communicating with the shipping provider",
			Code:   CodeProviderError,
		}

	// Default to internal error
	default:
		return &ProblemDetails{
			Type:   ErrorTypeInternalError,
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "An unexpected error occurred",
			Code:   CodeInternalError,
		}
	}
}

// NewValidationError creates a validation error with field details.
func NewValidationError(message string, errors []ErrorDetail) *ProblemDetails {
	return &ProblemDetails{
		Type:   ErrorTypeValidation,
		Title:  "Validation Error",
		Status: http.StatusBadRequest,
		Detail: message,
		Code:   CodeValidationError,
		Errors: errors,
	}
}

// NewFieldError creates an error detail for a single field.
func NewFieldError(field, code, message string) ErrorDetail {
	return ErrorDetail{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

// NewFieldErrorWithValue creates an error detail for a single field with the invalid value.
func NewFieldErrorWithValue(field, code, message string, value interface{}) ErrorDetail {
	return ErrorDetail{
		Field:   field,
		Code:    code,
		Message: message,
		Value:   value,
	}
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(resource string) *ProblemDetails {
	return &ProblemDetails{
		Type:   ErrorTypeNotFound,
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: resource + " was not found",
		Code:   CodeNotFound,
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(message string) *ProblemDetails {
	return &ProblemDetails{
		Type:   ErrorTypeConflict,
		Title:  "Conflict",
		Status: http.StatusConflict,
		Detail: message,
		Code:   CodeConflict,
	}
}

// NewUnauthorizedError creates an unauthorized error.
func NewUnauthorizedError(message string) *ProblemDetails {
	if message == "" {
		message = "Authentication is required"
	}
	return &ProblemDetails{
		Type:   ErrorTypeUnauthorized,
		Title:  "Unauthorized",
		Status: http.StatusUnauthorized,
		Detail: message,
		Code:   CodeUnauthorized,
	}
}

// NewForbiddenError creates a forbidden error.
func NewForbiddenError(message string) *ProblemDetails {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	return &ProblemDetails{
		Type:   ErrorTypeForbidden,
		Title:  "Forbidden",
		Status: http.StatusForbidden,
		Detail: message,
		Code:   CodeForbidden,
	}
}

// NewInternalError creates an internal server error.
func NewInternalError(message string) *ProblemDetails {
	if message == "" {
		message = "An unexpected error occurred"
	}
	return &ProblemDetails{
		Type:   ErrorTypeInternalError,
		Title:  "Internal Server Error",
		Status: http.StatusInternalServerError,
		Detail: message,
		Code:   CodeInternalError,
	}
}

// NewBadRequestError creates a bad request error.
func NewBadRequestError(message string) *ProblemDetails {
	return &ProblemDetails{
		Type:   ErrorTypeInvalidRequest,
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: message,
		Code:   CodeInvalidRequest,
	}
}

// NewRateLimitError creates a rate limit exceeded error.
func NewRateLimitError(limit int, retryAfter int) *ProblemDetails {
	return &ProblemDetails{
		Type:   ErrorTypeRateLimitExceeded,
		Title:  "Rate Limit Exceeded",
		Status: http.StatusTooManyRequests,
		Detail: "You have exceeded the rate limit. Please try again later.",
		Code:   CodeRateLimitExceeded,
		Extensions: map[string]interface{}{
			"limit":       limit,
			"retry_after": retryAfter,
		},
	}
}

// ErrorCodeFromString converts an error code string to a standardized format.
func ErrorCodeFromString(code string) string {
	// Convert to uppercase and replace hyphens with underscores
	return strings.ToUpper(strings.ReplaceAll(code, "-", "_"))
}

// ErrorTypeFromCode generates an error type URI from an error code.
func ErrorTypeFromCode(code string) string {
	// Convert to lowercase and replace underscores with hyphens
	slug := strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	return ErrorTypeBase + slug
}
