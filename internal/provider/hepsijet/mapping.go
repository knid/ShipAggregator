package hepsijet

import (
	"strings"

	"github.com/knid/kargonet/internal/domain"
)

// statusMapping maps Hepsijet status codes to normalized tracking statuses.
// Based on the provider-capabilities-matrix.md specification.
var statusMapping = map[string]domain.TrackingStatus{
	// Pickup/Acceptance statuses
	"PICKED_UP":        domain.TrackingStatusPickedUp,
	"ACCEPTED":         domain.TrackingStatusPickedUp,
	"COLLECTED":        domain.TrackingStatusPickedUp,
	"RECEIVED":         domain.TrackingStatusPickedUp,
	"SHIPMENT_CREATED": domain.TrackingStatusPickedUp,

	// Transit statuses
	"IN_TRANSIT": domain.TrackingStatusInTransit,
	"TRANSIT":    domain.TrackingStatusInTransit,
	"ON_THE_WAY": domain.TrackingStatusInTransit,
	"DEPARTED":   domain.TrackingStatusInTransit,
	"ARRIVED":    domain.TrackingStatusInTransit,
	"TRANSFER":   domain.TrackingStatusInTransit,

	// At hub/branch statuses
	"AT_HUB":                 domain.TrackingStatusAtHub,
	"AT_BRANCH":              domain.TrackingStatusAtHub,
	"AT_DISTRIBUTION_CENTER": domain.TrackingStatusAtHub,
	"SORTING":                domain.TrackingStatusAtHub,
	"PROCESSED":              domain.TrackingStatusAtHub,

	// Out for delivery statuses
	"OUT_FOR_DELIVERY": domain.TrackingStatusOutForDelivery,
	"WITH_COURIER":     domain.TrackingStatusOutForDelivery,
	"DELIVERY_ATTEMPT": domain.TrackingStatusOutForDelivery,
	"ON_VEHICLE":       domain.TrackingStatusOutForDelivery,

	// Delivered statuses
	"DELIVERED": domain.TrackingStatusDelivered,
	"COMPLETED": domain.TrackingStatusDelivered,
	"SIGNED":    domain.TrackingStatusDelivered,

	// Delivery failed statuses
	"DELIVERY_FAILED":         domain.TrackingStatusDeliveryFailed,
	"FAILED":                  domain.TrackingStatusDeliveryFailed,
	"ADDRESS_NOT_FOUND":       domain.TrackingStatusDeliveryFailed,
	"RECIPIENT_NOT_AVAILABLE": domain.TrackingStatusDeliveryFailed,
	"REFUSED":                 domain.TrackingStatusDeliveryFailed,

	// Return statuses
	"RETURNING":          domain.TrackingStatusReturning,
	"RETURN_IN_PROGRESS": domain.TrackingStatusReturning,
	"RETURN_REQUESTED":   domain.TrackingStatusReturning,

	// Returned to sender statuses
	"RETURNED":           domain.TrackingStatusReturnedToSender,
	"RETURNED_TO_SENDER": domain.TrackingStatusReturnedToSender,

	// Cancelled statuses
	"CANCELLED": domain.TrackingStatusCancelled,
	"CANCELED":  domain.TrackingStatusCancelled,
	"VOIDED":    domain.TrackingStatusCancelled,

	// Exception statuses
	"EXCEPTION": domain.TrackingStatusException,
	"DAMAGED":   domain.TrackingStatusException,
	"LOST":      domain.TrackingStatusException,
	"ON_HOLD":   domain.TrackingStatusException,
	"PENDING":   domain.TrackingStatusException,
}

// MapStatus maps a Hepsijet status code to a normalized tracking status.
// Returns TrackingStatusException for unknown status codes.
func MapStatus(hepsijetStatus string) domain.TrackingStatus {
	// Normalize the status code (uppercase, trim spaces)
	normalized := strings.ToUpper(strings.TrimSpace(hepsijetStatus))

	// Replace common variations
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	if status, ok := statusMapping[normalized]; ok {
		return status
	}

	// Try partial matching for common patterns
	if strings.Contains(normalized, "DELIVER") && strings.Contains(normalized, "FAIL") {
		return domain.TrackingStatusDeliveryFailed
	}
	if strings.Contains(normalized, "DELIVER") {
		return domain.TrackingStatusDelivered
	}
	if strings.Contains(normalized, "TRANSIT") {
		return domain.TrackingStatusInTransit
	}
	if strings.Contains(normalized, "RETURN") {
		if strings.Contains(normalized, "SENDER") || strings.Contains(normalized, "COMPLETE") {
			return domain.TrackingStatusReturnedToSender
		}
		return domain.TrackingStatusReturning
	}
	if strings.Contains(normalized, "CANCEL") {
		return domain.TrackingStatusCancelled
	}

	// Default to exception for unknown statuses
	return domain.TrackingStatusException
}

// MapStatusDescription returns a human-readable description for a Hepsijet status.
func MapStatusDescription(hepsijetStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(hepsijetStatus))

	descriptions := map[string]string{
		"PICKED_UP":               "Shipment picked up",
		"ACCEPTED":                "Shipment accepted",
		"COLLECTED":               "Shipment collected",
		"RECEIVED":                "Shipment received",
		"SHIPMENT_CREATED":        "Shipment created",
		"IN_TRANSIT":              "In transit",
		"TRANSIT":                 "In transit",
		"ON_THE_WAY":              "On the way",
		"DEPARTED":                "Departed from facility",
		"ARRIVED":                 "Arrived at facility",
		"TRANSFER":                "In transfer",
		"AT_HUB":                  "At hub",
		"AT_BRANCH":               "At branch",
		"AT_DISTRIBUTION_CENTER":  "At distribution center",
		"SORTING":                 "Being sorted",
		"PROCESSED":               "Processed",
		"OUT_FOR_DELIVERY":        "Out for delivery",
		"WITH_COURIER":            "With courier",
		"DELIVERY_ATTEMPT":        "Delivery attempt",
		"ON_VEHICLE":              "On delivery vehicle",
		"DELIVERED":               "Delivered",
		"COMPLETED":               "Delivery completed",
		"SIGNED":                  "Signed for",
		"DELIVERY_FAILED":         "Delivery failed",
		"FAILED":                  "Failed",
		"ADDRESS_NOT_FOUND":       "Address not found",
		"RECIPIENT_NOT_AVAILABLE": "Recipient not available",
		"REFUSED":                 "Delivery refused",
		"RETURNING":               "Returning to sender",
		"RETURN_IN_PROGRESS":      "Return in progress",
		"RETURN_REQUESTED":        "Return requested",
		"RETURNED":                "Returned to sender",
		"RETURNED_TO_SENDER":      "Returned to sender",
		"CANCELLED":               "Cancelled",
		"CANCELED":                "Cancelled",
		"VOIDED":                  "Voided",
		"EXCEPTION":               "Exception",
		"DAMAGED":                 "Damaged",
		"LOST":                    "Lost",
		"ON_HOLD":                 "On hold",
		"PENDING":                 "Pending",
	}

	if desc, ok := descriptions[normalized]; ok {
		return desc
	}

	// Return the original status if no description found
	return hepsijetStatus
}

// MapServiceCode maps internal service codes to Hepsijet service types.
func MapServiceCode(serviceCode string) string {
	switch strings.ToLower(serviceCode) {
	case "standard", "standart":
		return ServiceTypeStandard
	case "express", "ekspres", "hizli":
		return ServiceTypeExpress
	case "same_day", "ayni_gun", "sameday":
		return ServiceTypeSameDay
	default:
		return strings.ToUpper(serviceCode)
	}
}

// MapLabelFormat maps internal label formats to Hepsijet label formats.
func MapLabelFormat(format domain.LabelFormat) string {
	switch format {
	case domain.LabelFormatPDF:
		return LabelFormatPDF
	case domain.LabelFormatZPL:
		return LabelFormatZPL
	default:
		return LabelFormatPDF
	}
}

// MapLabelFormatFromHepsijet maps Hepsijet label formats to internal label formats.
func MapLabelFormatFromHepsijet(format string) domain.LabelFormat {
	switch strings.ToUpper(format) {
	case "PDF":
		return domain.LabelFormatPDF
	case "ZPL":
		return domain.LabelFormatZPL
	default:
		return domain.LabelFormatPDF
	}
}

// MapContentType returns the MIME content type for a Hepsijet label format.
func MapContentType(format string) string {
	switch strings.ToUpper(format) {
	case "PDF":
		return "application/pdf"
	case "ZPL":
		return "application/x-zpl"
	default:
		return "application/octet-stream"
	}
}

// ErrorCodeMapping maps Hepsijet error codes to provider error codes.
var ErrorCodeMapping = map[string]string{
	"AUTH_FAILED":         "PROVIDER_AUTH_FAILED",
	"INVALID_CREDENTIALS": "PROVIDER_AUTH_FAILED",
	"UNAUTHORIZED":        "PROVIDER_AUTH_FAILED",
	"INVALID_ADDRESS":     "PROVIDER_REJECTED",
	"INVALID_PARCEL":      "PROVIDER_REJECTED",
	"SERVICE_UNAVAILABLE": "PROVIDER_DOWN",
	"RATE_LIMITED":        "PROVIDER_RATE_LIMITED",
	"TOO_MANY_REQUESTS":   "PROVIDER_RATE_LIMITED",
	"NOT_FOUND":           "PROVIDER_NOT_FOUND",
	"SHIPMENT_NOT_FOUND":  "PROVIDER_NOT_FOUND",
	"ALREADY_CANCELLED":   "PROVIDER_REJECTED",
	"CANNOT_CANCEL":       "PROVIDER_REJECTED",
	"INVALID_QUOTE":       "PROVIDER_REJECTED",
	"QUOTE_EXPIRED":       "PROVIDER_REJECTED",
	"INVALID_SERVICE":     "PROVIDER_REJECTED",
	"VALIDATION_ERROR":    "PROVIDER_REJECTED",
}

// MapErrorCode maps a Hepsijet error code to a normalized error code.
func MapErrorCode(hepsijetErrorCode string) string {
	normalized := strings.ToUpper(strings.TrimSpace(hepsijetErrorCode))
	if code, ok := ErrorCodeMapping[normalized]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if a Hepsijet error code indicates a retryable error.
func IsRetryableError(hepsijetErrorCode string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(hepsijetErrorCode))
	retryable := map[string]bool{
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMITED":        true,
		"TOO_MANY_REQUESTS":   true,
		"TIMEOUT":             true,
		"CONNECTION_ERROR":    true,
		"INTERNAL_ERROR":      true,
	}
	return retryable[normalized]
}
