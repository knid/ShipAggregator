package yurtici

import (
	"fmt"
	"strings"

	"github.com/knid/kargonet/internal/domain"
)

// operationStatusMapping maps Yurtici operationStatus codes to normalized tracking statuses.
var operationStatusMapping = map[string]domain.TrackingStatus{
	"NOP": domain.TrackingStatusLabelCreated,     // Not processed
	"IND": domain.TrackingStatusInTransit,        // In delivery
	"ISR": domain.TrackingStatusPickedUp,         // Processed, not yet invoiced
	"CNL": domain.TrackingStatusCancelled,        // Exit blocked / cancelled
	"ISC": domain.TrackingStatusCancelled,        // Previously cancelled
	"DLV": domain.TrackingStatusDelivered,        // Delivered
	"BI":  domain.TrackingStatusCancelled,        // Invoice cancelled by branch
}

// operationCodeMapping maps Yurtici operationCode (numeric) to normalized tracking statuses.
var operationCodeMapping = map[string]domain.TrackingStatus{
	"0": domain.TrackingStatusLabelCreated,     // NOP - not processed
	"1": domain.TrackingStatusInTransit,        // IND - in delivery
	"2": domain.TrackingStatusPickedUp,         // ISR - processed
	"3": domain.TrackingStatusCancelled,        // CNL - exit blocked
	"4": domain.TrackingStatusCancelled,        // ISC - previously cancelled
	"5": domain.TrackingStatusDelivered,        // DLV - delivered
	"6": domain.TrackingStatusCancelled,        // BI - branch cancelled
}

// eventIdMapping maps Yurtici eventId codes from tracking history to normalized statuses.
var eventIdMapping = map[string]domain.TrackingStatus{
	"IN": domain.TrackingStatusAtHub,           // Kargo Indirildi (cargo unloaded)
	"YK": domain.TrackingStatusInTransit,       // Kargo Yuklendi (cargo loaded)
	"CK": domain.TrackingStatusInTransit,       // Cikis (departure)
	"TM": domain.TrackingStatusDelivered,       // Teslim (delivered)
	"DG": domain.TrackingStatusOutForDelivery,  // Dagitimda (out for delivery)
}

// MapOperationStatus maps a Yurtici operationStatus to a normalized tracking status.
func MapOperationStatus(status string) domain.TrackingStatus {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	if s, ok := operationStatusMapping[normalized]; ok {
		return s
	}
	return domain.TrackingStatusUnknown
}

// MapOperationCode maps a Yurtici operationCode (numeric) to a normalized tracking status.
func MapOperationCode(code string) domain.TrackingStatus {
	normalized := strings.TrimSpace(code)
	if s, ok := operationCodeMapping[normalized]; ok {
		return s
	}
	return domain.TrackingStatusUnknown
}

// MapEventId maps a Yurtici eventId from tracking history to a normalized tracking status.
func MapEventId(eventId string) domain.TrackingStatus {
	normalized := strings.ToUpper(strings.TrimSpace(eventId))
	if s, ok := eventIdMapping[normalized]; ok {
		return s
	}
	// Fallback: treat unknown events as in-transit
	return domain.TrackingStatusInTransit
}

// operationStatusDescriptions maps operationStatus to Turkish descriptions.
var operationStatusDescriptions = map[string]string{
	"NOP": "Henüz işleme alınmadı",
	"IND": "Kargo taşıma sürecinde",
	"ISR": "Kargo işleme alındı",
	"CNL": "Kargo çıkışı engellendi",
	"ISC": "Kargo daha önce iptal edilmiş",
	"DLV": "Kargo teslim edildi",
	"BI":  "Fatura şube tarafından iptal edildi",
}

// MapOperationStatusDescription returns a human-readable description for an operationStatus.
func MapOperationStatusDescription(status string) string {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	if desc, ok := operationStatusDescriptions[normalized]; ok {
		return desc
	}
	return status
}

// eventDescriptions maps eventId to Turkish descriptions.
var eventDescriptions = map[string]string{
	"IN": "Kargo indirildi",
	"YK": "Kargo yüklendi",
	"CK": "Çıkış yapıldı",
	"TM": "Teslim edildi",
	"DG": "Dağıtıma çıktı",
}

// MapEventDescription returns a description for an eventId.
func MapEventDescription(eventId, eventName string) string {
	normalized := strings.ToUpper(strings.TrimSpace(eventId))
	if desc, ok := eventDescriptions[normalized]; ok {
		return desc
	}
	if eventName != "" {
		return eventName
	}
	return eventId
}

// MapStatus maps a Yurtici status code to a normalized tracking status.
// This handles both operationStatus (NOP, IND, etc.) and operationCode (0, 1, etc.).
func MapStatus(yurticiStatus string) domain.TrackingStatus {
	normalized := strings.ToUpper(strings.TrimSpace(yurticiStatus))

	// Try operationStatus mapping first
	if s, ok := operationStatusMapping[normalized]; ok {
		return s
	}

	// Try operationCode (numeric) mapping
	if s, ok := operationCodeMapping[normalized]; ok {
		return s
	}

	// Try eventId mapping
	if s, ok := eventIdMapping[normalized]; ok {
		return s
	}

	return domain.TrackingStatusUnknown
}

// MapStatusDescription returns a human-readable description for a Yurtici status.
func MapStatusDescription(yurticiStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(yurticiStatus))

	if desc, ok := operationStatusDescriptions[normalized]; ok {
		return desc
	}
	if desc, ok := eventDescriptions[normalized]; ok {
		return desc
	}

	return yurticiStatus
}

// ErrorCodeMapping maps Yurtici numeric error codes to normalized provider error codes.
var ErrorCodeMapping = map[string]string{
	"0":     "SUCCESS",
	"936":   "PROVIDER_ERROR",         // Unexpected error (retryable)
	"60020": "PROVIDER_REJECTED",      // Cargo key already exists
	"80859": "PROVIDER_REJECTED",      // Cargo key param not found
	"82500": "PROVIDER_REJECTED",      // Cargo key length exceeded
	"82502": "PROVIDER_REJECTED",      // Receiver address length error
	"82503": "PROVIDER_REJECTED",      // Receiver name length exceeded
	"82505": "PROVIDER_REJECTED",      // COD invoice amount missing
	"82515": "PROVIDER_REJECTED",      // Invalid email
	"82516": "PROVIDER_REJECTED",      // Invalid phone
	"82519": "PROVIDER_NOT_FOUND",     // Cargo key not found for user
	"82520": "PROVIDER_REJECTED",      // Cargo key already cancelled
}

// MapErrorCode maps a Yurtici error code to a normalized error code.
func MapErrorCode(yurticiErrorCode string) string {
	if code, ok := ErrorCodeMapping[yurticiErrorCode]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if a Yurtici error code indicates a retryable error.
func IsRetryableError(yurticiErrorCode string) bool {
	return yurticiErrorCode == "936"
}

// MapErrorMessage returns a human-readable error message for a Yurtici error code.
func MapErrorMessage(errCode, errMessage string) string {
	if errMessage != "" {
		return errMessage
	}
	messages := map[string]string{
		"936":   "Beklenmeyen hata oluştu",
		"60020": "Bu kargo anahtarı zaten kayıtlı",
		"80859": "Kargo anahtarı parametresi bulunamadı",
		"82500": "Kargo anahtarı uzunluk sınırını aştı",
		"82502": "Alıcı adres bilgisi hatalı",
		"82503": "Alıcı ad bilgisi çok uzun",
		"82505": "Tahsilatlı gönderi fatura tutarı eksik",
		"82515": "Geçersiz e-posta adresi",
		"82516": "Geçersiz telefon numarası",
		"82519": "Bu kargo anahtarı kullanıcı için bulunamadı",
		"82520": "Bu kargo anahtarı zaten iptal edilmiş",
	}
	if msg, ok := messages[errCode]; ok {
		return msg
	}
	return fmt.Sprintf("Yurtiçi Kargo hatası: %s", errCode)
}

// CancelOperationCode constants for cancelShipment response.
const (
	CancelOpNOP = "0" // Not processed
	CancelOpIND = "1" // Return request sent (for in-transit)
	CancelOpISR = "2" // Processed successfully
	CancelOpCNL = "3" // Cancellation confirmed
	CancelOpISC = "4" // Already cancelled
	CancelOpDLV = "5" // Cannot cancel, delivered
)

// IsCancelSuccess returns true if the cancel operationCode indicates success.
func IsCancelSuccess(operationCode string) bool {
	switch operationCode {
	case CancelOpNOP, CancelOpISR, CancelOpCNL:
		return true
	case CancelOpIND:
		// Return request sent for in-transit package - partial success
		return true
	default:
		return false
	}
}

// IsCancelAlreadyCancelled returns true if the shipment was already cancelled.
func IsCancelAlreadyCancelled(operationCode string) bool {
	return operationCode == CancelOpISC
}
