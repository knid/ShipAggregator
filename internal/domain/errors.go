package domain

import (
	"errors"
	"fmt"
)

// Domain errors are sentinel errors that represent business logic failures.
// These errors should be used throughout the domain layer and can be
// wrapped with additional context as needed.

// General errors
var (
	// ErrNotFound indicates a resource was not found.
	ErrNotFound = errors.New("resource not found")
	// ErrAlreadyExists indicates a resource already exists.
	ErrAlreadyExists = errors.New("resource already exists")
	// ErrInvalidInput indicates invalid input was provided.
	ErrInvalidInput = errors.New("invalid input")
	// ErrUnauthorized indicates the request is not authorized.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden indicates the request is forbidden.
	ErrForbidden = errors.New("forbidden")
	// ErrConflict indicates a conflict with the current state.
	ErrConflict = errors.New("conflict")
	// ErrInternal indicates an internal error occurred.
	ErrInternal = errors.New("internal error")
)

// Organization errors
var (
	// ErrOrganizationNotFound indicates the organization was not found.
	ErrOrganizationNotFound = errors.New("organization not found")
	// ErrOrganizationSuspended indicates the organization is suspended.
	ErrOrganizationSuspended = errors.New("organization is suspended")
	// ErrOrganizationClosed indicates the organization is closed.
	ErrOrganizationClosed = errors.New("organization is closed")
	// ErrInvalidOrganizationID indicates an invalid organization ID.
	ErrInvalidOrganizationID = errors.New("invalid organization ID")
	// ErrOrganizationSlugTaken indicates the organization slug is already taken.
	ErrOrganizationSlugTaken = errors.New("organization slug is already taken")
)

// Member errors
var (
	// ErrMemberNotFound indicates the member was not found.
	ErrMemberNotFound = errors.New("member not found")
	// ErrMemberSuspended indicates the member is suspended.
	ErrMemberSuspended = errors.New("member is suspended")
	// ErrMemberRemoved indicates the member has been removed.
	ErrMemberRemoved = errors.New("member has been removed")
	// ErrMemberEmailTaken indicates the email is already taken.
	ErrMemberEmailTaken = errors.New("member email is already taken")
	// ErrInvalidCredentials indicates invalid login credentials.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrMemberNotActivated indicates the member has not activated their account.
	ErrMemberNotActivated = errors.New("member has not activated their account")
)

// Role errors
var (
	// ErrRoleNotFound indicates the role was not found.
	ErrRoleNotFound = errors.New("role not found")
	// ErrSystemRoleImmutable indicates a system role cannot be modified.
	ErrSystemRoleImmutable = errors.New("system role cannot be modified")
	// ErrRoleInUse indicates the role is in use and cannot be deleted.
	ErrRoleInUse = errors.New("role is in use")
)

// API Key errors
var (
	// ErrAPIKeyNotFound indicates the API key was not found.
	ErrAPIKeyNotFound = errors.New("API key not found")
	// ErrAPIKeyExpired indicates the API key has expired.
	ErrAPIKeyExpired = errors.New("API key has expired")
	// ErrAPIKeyRevoked indicates the API key has been revoked.
	ErrAPIKeyRevoked = errors.New("API key has been revoked")
	// ErrAPIKeyInactive indicates the API key is inactive.
	ErrAPIKeyInactive = errors.New("API key is inactive")
	// ErrAPIKeyInvalidScope indicates the API key does not have the required scope.
	ErrAPIKeyInvalidScope = errors.New("API key does not have the required scope")
	// ErrAPIKeyIPNotAllowed indicates the IP address is not allowed.
	ErrAPIKeyIPNotAllowed = errors.New("IP address is not allowed")
	// ErrAPIKeyRateLimitExceeded indicates the rate limit has been exceeded.
	ErrAPIKeyRateLimitExceeded = errors.New("rate limit exceeded")
)

// Session errors
var (
	// ErrSessionNotFound indicates the session was not found.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session has expired")
	// ErrInvalidSessionToken indicates an invalid session token.
	ErrInvalidSessionToken = errors.New("invalid session token")
)

// Address errors
var (
	// ErrAddressNotFound indicates the address was not found.
	ErrAddressNotFound = errors.New("address not found")
	// ErrInvalidAddress indicates an invalid address.
	ErrInvalidAddress = errors.New("invalid address")
	// ErrInvalidPhoneNumber indicates an invalid phone number.
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
	// ErrInvalidPostalCode indicates an invalid postal code.
	ErrInvalidPostalCode = errors.New("invalid postal code")
	// ErrCityNotFound indicates the city was not found.
	ErrCityNotFound = errors.New("city not found")
	// ErrDistrictNotFound indicates the district was not found.
	ErrDistrictNotFound = errors.New("district not found")
	// ErrNeighborhoodNotFound indicates the neighborhood was not found.
	ErrNeighborhoodNotFound = errors.New("neighborhood not found")
)

// Provider errors
var (
	// ErrProviderNotFound indicates the provider was not found.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrProviderInactive indicates the provider is inactive.
	ErrProviderInactive = errors.New("provider is inactive")
	// ErrProviderServiceNotFound indicates the provider service was not found.
	ErrProviderServiceNotFound = errors.New("provider service not found")
	// ErrProviderServiceInactive indicates the provider service is inactive.
	ErrProviderServiceInactive = errors.New("provider service is inactive")
	// ErrProviderUnsupportedFeature indicates the provider does not support the feature.
	ErrProviderUnsupportedFeature = errors.New("provider does not support this feature")
	// ErrProviderAPIError indicates an error from the provider API.
	ErrProviderAPIError = errors.New("provider API error")
	// ErrProviderTimeout indicates a timeout from the provider.
	ErrProviderTimeout = errors.New("provider timeout")
)

// Provider Account errors
var (
	// ErrProviderAccountNotFound indicates the provider account was not found.
	ErrProviderAccountNotFound = errors.New("provider account not found")
	// ErrProviderAccountNotVerified indicates the provider account is not verified.
	ErrProviderAccountNotVerified = errors.New("provider account is not verified")
	// ErrProviderAccountSuspended indicates the provider account is suspended.
	ErrProviderAccountSuspended = errors.New("provider account is suspended")
	// ErrProviderAccountVerificationFailed indicates verification failed.
	ErrProviderAccountVerificationFailed = errors.New("provider account verification failed")
	// ErrInvalidProviderCredentials indicates invalid provider credentials.
	ErrInvalidProviderCredentials = errors.New("invalid provider credentials")
)

// Quote errors
var (
	// ErrQuoteNotFound indicates the quote was not found.
	ErrQuoteNotFound = errors.New("quote not found")
	// ErrQuoteExpired indicates the quote has expired.
	ErrQuoteExpired = errors.New("quote has expired")
	// ErrQuoteFailed indicates the quote failed to generate.
	ErrQuoteFailed = errors.New("quote failed to generate")
	// ErrNoOffersAvailable indicates no offers are available.
	ErrNoOffersAvailable = errors.New("no offers available")
)

// Offer errors
var (
	// ErrOfferNotFound indicates the offer was not found.
	ErrOfferNotFound = errors.New("offer not found")
	// ErrOfferExpired indicates the offer has expired.
	ErrOfferExpired = errors.New("offer has expired")
)

// Shipment errors
var (
	// ErrShipmentNotFound indicates the shipment was not found.
	ErrShipmentNotFound = errors.New("shipment not found")
	// ErrShipmentAlreadyPurchased indicates the shipment label was already purchased.
	ErrShipmentAlreadyPurchased = errors.New("shipment label already purchased")
	// ErrShipmentCannotBeCancelled indicates the shipment cannot be cancelled.
	ErrShipmentCannotBeCancelled = errors.New("shipment cannot be cancelled")
	// ErrShipmentCannotSchedulePickup indicates pickup cannot be scheduled.
	ErrShipmentCannotSchedulePickup = errors.New("cannot schedule pickup for this shipment")
	// ErrShipmentCannotCreateReturn indicates a return cannot be created.
	ErrShipmentCannotCreateReturn = errors.New("cannot create return for this shipment")
	// ErrShipmentAlreadyHasReturn indicates the shipment already has a return.
	ErrShipmentAlreadyHasReturn = errors.New("shipment already has a return")
	// ErrInvalidShipmentStatus indicates an invalid shipment status transition.
	ErrInvalidShipmentStatus = errors.New("invalid shipment status")
	// ErrParcelTooHeavy indicates the parcel exceeds the weight limit.
	ErrParcelTooHeavy = errors.New("parcel exceeds weight limit")
	// ErrParcelTooLarge indicates the parcel exceeds the size limit.
	ErrParcelTooLarge = errors.New("parcel exceeds size limit")
)

// Tracking errors
var (
	// ErrTrackingNotFound indicates tracking information was not found.
	ErrTrackingNotFound = errors.New("tracking information not found")
	// ErrInvalidTrackingStatus indicates an invalid tracking status.
	ErrInvalidTrackingStatus = errors.New("invalid tracking status")
)

// Webhook errors
var (
	// ErrWebhookEndpointNotFound indicates the webhook endpoint was not found.
	ErrWebhookEndpointNotFound = errors.New("webhook endpoint not found")
	// ErrWebhookSubscriptionNotFound indicates the webhook subscription was not found.
	ErrWebhookSubscriptionNotFound = errors.New("webhook subscription not found")
	// ErrWebhookDeliveryNotFound indicates the webhook delivery was not found.
	ErrWebhookDeliveryNotFound = errors.New("webhook delivery not found")
	// ErrWebhookDeliveryFailed indicates the webhook delivery failed.
	ErrWebhookDeliveryFailed = errors.New("webhook delivery failed")
	// ErrInvalidWebhookURL indicates an invalid webhook URL.
	ErrInvalidWebhookURL = errors.New("invalid webhook URL")
	// ErrInvalidWebhookEventType indicates an invalid webhook event type.
	ErrInvalidWebhookEventType = errors.New("invalid webhook event type")
)

// Billing errors
var (
	// ErrInsufficientBalance indicates insufficient wallet balance.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrInvalidAmount indicates an invalid amount.
	ErrInvalidAmount = errors.New("invalid amount")
	// ErrAmountBelowMinimum indicates the amount is below the minimum.
	ErrAmountBelowMinimum = errors.New("amount is below minimum")
	// ErrAmountAboveMaximum indicates the amount is above the maximum.
	ErrAmountAboveMaximum = errors.New("amount is above maximum")
	// ErrLedgerEntryNotFound indicates the ledger entry was not found.
	ErrLedgerEntryNotFound = errors.New("ledger entry not found")
	// ErrLedgerImmutable indicates ledger entries cannot be modified.
	ErrLedgerImmutable = errors.New("ledger entries are immutable")
)

// Topup errors
var (
	// ErrTopupNotFound indicates the top-up was not found.
	ErrTopupNotFound = errors.New("top-up not found")
	// ErrTopupExpired indicates the top-up has expired.
	ErrTopupExpired = errors.New("top-up has expired")
	// ErrTopupAlreadyCompleted indicates the top-up is already completed.
	ErrTopupAlreadyCompleted = errors.New("top-up is already completed")
	// ErrTopupAlreadyFailed indicates the top-up has already failed.
	ErrTopupAlreadyFailed = errors.New("top-up has already failed")
	// ErrTopupCannotBeCancelled indicates the top-up cannot be cancelled.
	ErrTopupCannotBeCancelled = errors.New("top-up cannot be cancelled")
	// ErrPaymentFailed indicates the payment failed.
	ErrPaymentFailed = errors.New("payment failed")
	// ErrReturnURLRequired indicates a return URL is required.
	ErrReturnURLRequired = errors.New("return URL is required")
)

// Refund errors
var (
	// ErrRefundNotFound indicates the refund was not found.
	ErrRefundNotFound = errors.New("refund not found")
	// ErrRefundAlreadyProcessed indicates the refund is already processed.
	ErrRefundAlreadyProcessed = errors.New("refund is already processed")
	// ErrRefundAmountExceedsOriginal indicates the refund amount exceeds the original.
	ErrRefundAmountExceedsOriginal = errors.New("refund amount exceeds original charge")
)

// COD Settlement errors
var (
	// ErrCODSettlementNotFound indicates the COD settlement was not found.
	ErrCODSettlementNotFound = errors.New("COD settlement not found")
	// ErrCODSettlementAlreadySettled indicates the COD is already settled.
	ErrCODSettlementAlreadySettled = errors.New("COD settlement is already settled")
)

// Idempotency errors
var (
	// ErrIdempotencyKeyRequired indicates an idempotency key is required.
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required")
	// ErrIdempotencyKeyConflict indicates the idempotency key was used with different parameters.
	ErrIdempotencyKeyConflict = errors.New("idempotency key was used with different parameters")
	// ErrIdempotencyKeyInProgress indicates the request is already in progress.
	ErrIdempotencyKeyInProgress = errors.New("request with this idempotency key is already in progress")
)

// Document errors
var (
	// ErrDocumentNotFound indicates the document was not found.
	ErrDocumentNotFound = errors.New("document not found")
	// ErrInvalidDocumentType indicates an invalid document type.
	ErrInvalidDocumentType = errors.New("invalid document type")
	// ErrDocumentTooLarge indicates the document is too large.
	ErrDocumentTooLarge = errors.New("document is too large")
)

// DomainError wraps a domain error with additional context.
type DomainError struct {
	// Err is the underlying error.
	Err error
	// Message is the error message.
	Message string
	// Code is the error code for API responses.
	Code string
	// Details contains additional error details.
	Details map[string]interface{}
}

// Error returns the error message.
func (e *DomainError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *DomainError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target.
func (e *DomainError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewDomainError creates a new DomainError.
func NewDomainError(err error, message, code string) *DomainError {
	return &DomainError{
		Err:     err,
		Message: message,
		Code:    code,
		Details: make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the error.
func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
	e.Details[key] = value
	return e
}

// WrapError wraps an error with a message.
func WrapError(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

// IsNotFoundError checks if the error is a not found error.
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrOrganizationNotFound) ||
		errors.Is(err, ErrMemberNotFound) ||
		errors.Is(err, ErrRoleNotFound) ||
		errors.Is(err, ErrAPIKeyNotFound) ||
		errors.Is(err, ErrSessionNotFound) ||
		errors.Is(err, ErrAddressNotFound) ||
		errors.Is(err, ErrProviderNotFound) ||
		errors.Is(err, ErrProviderServiceNotFound) ||
		errors.Is(err, ErrProviderAccountNotFound) ||
		errors.Is(err, ErrQuoteNotFound) ||
		errors.Is(err, ErrOfferNotFound) ||
		errors.Is(err, ErrShipmentNotFound) ||
		errors.Is(err, ErrTrackingNotFound) ||
		errors.Is(err, ErrWebhookEndpointNotFound) ||
		errors.Is(err, ErrWebhookSubscriptionNotFound) ||
		errors.Is(err, ErrWebhookDeliveryNotFound) ||
		errors.Is(err, ErrLedgerEntryNotFound) ||
		errors.Is(err, ErrTopupNotFound) ||
		errors.Is(err, ErrRefundNotFound) ||
		errors.Is(err, ErrCODSettlementNotFound) ||
		errors.Is(err, ErrDocumentNotFound) ||
		errors.Is(err, ErrCityNotFound) ||
		errors.Is(err, ErrDistrictNotFound) ||
		errors.Is(err, ErrNeighborhoodNotFound)
}

// IsAuthenticationError checks if the error is an authentication error.
func IsAuthenticationError(err error) bool {
	return errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrInvalidCredentials) ||
		errors.Is(err, ErrAPIKeyExpired) ||
		errors.Is(err, ErrAPIKeyRevoked) ||
		errors.Is(err, ErrAPIKeyInactive) ||
		errors.Is(err, ErrSessionExpired) ||
		errors.Is(err, ErrInvalidSessionToken)
}

// IsAuthorizationError checks if the error is an authorization error.
func IsAuthorizationError(err error) bool {
	return errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrAPIKeyInvalidScope) ||
		errors.Is(err, ErrAPIKeyIPNotAllowed)
}

// IsValidationError checks if the error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrInvalidAddress) ||
		errors.Is(err, ErrInvalidPhoneNumber) ||
		errors.Is(err, ErrInvalidPostalCode) ||
		errors.Is(err, ErrInvalidAmount) ||
		errors.Is(err, ErrAmountBelowMinimum) ||
		errors.Is(err, ErrAmountAboveMaximum) ||
		errors.Is(err, ErrInvalidWebhookURL) ||
		errors.Is(err, ErrInvalidWebhookEventType) ||
		errors.Is(err, ErrInvalidDocumentType)
}

// IsConflictError checks if the error is a conflict error.
func IsConflictError(err error) bool {
	return errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrAlreadyExists) ||
		errors.Is(err, ErrOrganizationSlugTaken) ||
		errors.Is(err, ErrMemberEmailTaken) ||
		errors.Is(err, ErrIdempotencyKeyConflict) ||
		errors.Is(err, ErrIdempotencyKeyInProgress)
}

// IsRateLimitError checks if the error is a rate limit error.
func IsRateLimitError(err error) bool {
	return errors.Is(err, ErrAPIKeyRateLimitExceeded)
}

// IsProviderError checks if the error is a provider error.
func IsProviderError(err error) bool {
	return errors.Is(err, ErrProviderAPIError) ||
		errors.Is(err, ErrProviderTimeout) ||
		errors.Is(err, ErrProviderInactive) ||
		errors.Is(err, ErrProviderUnsupportedFeature)
}
