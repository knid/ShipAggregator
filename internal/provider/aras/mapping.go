package aras

import (
	"strings"

	"github.com/knid/kargonet/internal/domain"
)

// statusMapping maps Aras status codes to normalized tracking statuses.
// Based on the provider-capabilities-matrix.md specification.
var statusMapping = map[string]domain.TrackingStatus{
	// Pickup/Acceptance statuses
	"KARGO_KABUL":   domain.TrackingStatusPickedUp,
	"KABUL":         domain.TrackingStatusPickedUp,
	"KARGO_ALINDI":  domain.TrackingStatusPickedUp,
	"TESLIM_ALINDI": domain.TrackingStatusPickedUp,

	// Transit statuses
	"TRANSFER_MERKEZ": domain.TrackingStatusInTransit,
	"TRANSFER":        domain.TrackingStatusInTransit,
	"AKTARMA":         domain.TrackingStatusInTransit,
	"YOLDA":           domain.TrackingStatusInTransit,
	"CIKIS":           domain.TrackingStatusInTransit,

	// At hub/branch statuses
	"DAGITIM_SUBESINDE": domain.TrackingStatusAtHub,
	"SUBEDE":            domain.TrackingStatusAtHub,
	"SUBE_TESLIM":       domain.TrackingStatusAtHub,
	"DAGITIM_MERKEZI":   domain.TrackingStatusAtHub,

	// Out for delivery statuses
	"DAGITIMDA":        domain.TrackingStatusOutForDelivery,
	"DAGITIMA_CIKTI":   domain.TrackingStatusOutForDelivery,
	"KURYE_ZIMMETINDE": domain.TrackingStatusOutForDelivery,

	// Delivered statuses
	"TESLIM_EDILDI":  domain.TrackingStatusDelivered,
	"TESLIM":         domain.TrackingStatusDelivered,
	"ALICIYA_TESLIM": domain.TrackingStatusDelivered,

	// Delivery failed statuses
	"TESLIM_EDILEMEDI": domain.TrackingStatusDeliveryFailed,
	"TESLIM_BASARISIZ": domain.TrackingStatusDeliveryFailed,
	"ADRES_BULUNAMADI": domain.TrackingStatusDeliveryFailed,
	"ALICI_YOK":        domain.TrackingStatusDeliveryFailed,

	// Return statuses
	"IADE":           domain.TrackingStatusReturning,
	"IADE_SURECINDE": domain.TrackingStatusReturning,
	"GONDERENE_IADE": domain.TrackingStatusReturning,

	// Returned to sender statuses
	"IADE_TESLIM":      domain.TrackingStatusReturnedToSender,
	"IADE_EDILDI":      domain.TrackingStatusReturnedToSender,
	"GONDERENE_TESLIM": domain.TrackingStatusReturnedToSender,

	// Cancelled statuses
	"IPTAL":        domain.TrackingStatusCancelled,
	"IPTAL_EDILDI": domain.TrackingStatusCancelled,

	// Exception statuses
	"HASAR":     domain.TrackingStatusException,
	"KAYIP":     domain.TrackingStatusException,
	"BEKLEMEDE": domain.TrackingStatusException,
}

// MapStatus maps an Aras status code to a normalized tracking status.
// Returns TrackingStatusException for unknown status codes.
func MapStatus(arasStatus string) domain.TrackingStatus {
	// Normalize the status code (uppercase, trim spaces)
	normalized := strings.ToUpper(strings.TrimSpace(arasStatus))

	// Replace common variations
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	if status, ok := statusMapping[normalized]; ok {
		return status
	}

	// Try partial matching for common patterns
	if strings.Contains(normalized, "TESLIM_EDILDI") || strings.Contains(normalized, "TESLIM_ALINDI") {
		return domain.TrackingStatusDelivered
	}
	if strings.Contains(normalized, "DAGITIM") && strings.Contains(normalized, "CIKTI") {
		return domain.TrackingStatusOutForDelivery
	}
	if strings.Contains(normalized, "TRANSFER") || strings.Contains(normalized, "AKTARMA") {
		return domain.TrackingStatusInTransit
	}
	if strings.Contains(normalized, "IADE") {
		if strings.Contains(normalized, "TESLIM") {
			return domain.TrackingStatusReturnedToSender
		}
		return domain.TrackingStatusReturning
	}
	if strings.Contains(normalized, "IPTAL") {
		return domain.TrackingStatusCancelled
	}

	// Default to exception for unknown statuses
	return domain.TrackingStatusException
}

// MapStatusDescription returns a human-readable description for an Aras status.
func MapStatusDescription(arasStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(arasStatus))

	descriptions := map[string]string{
		"KARGO_KABUL":       "Kargo kabul edildi",
		"KABUL":             "Kabul edildi",
		"KARGO_ALINDI":      "Kargo alındı",
		"TESLIM_ALINDI":     "Teslim alındı",
		"TRANSFER_MERKEZ":   "Transfer merkezinde",
		"TRANSFER":          "Transfer aşamasında",
		"AKTARMA":           "Aktarma merkezinde",
		"YOLDA":             "Yolda",
		"CIKIS":             "Çıkış yapıldı",
		"DAGITIM_SUBESINDE": "Dağıtım şubesinde",
		"SUBEDE":            "Şubede",
		"SUBE_TESLIM":       "Şubeye teslim edildi",
		"DAGITIM_MERKEZI":   "Dağıtım merkezinde",
		"DAGITIMDA":         "Dağıtımda",
		"DAGITIMA_CIKTI":    "Dağıtıma çıktı",
		"KURYE_ZIMMETINDE":  "Kurye zimmetinde",
		"TESLIM_EDILDI":     "Teslim edildi",
		"TESLIM":            "Teslim edildi",
		"ALICIYA_TESLIM":    "Alıcıya teslim edildi",
		"TESLIM_EDILEMEDI":  "Teslim edilemedi",
		"TESLIM_BASARISIZ":  "Teslim başarısız",
		"ADRES_BULUNAMADI":  "Adres bulunamadı",
		"ALICI_YOK":         "Alıcı bulunamadı",
		"IADE":              "İade sürecinde",
		"IADE_SURECINDE":    "İade sürecinde",
		"GONDERENE_IADE":    "Gönderene iade ediliyor",
		"IADE_TESLIM":       "İade teslim edildi",
		"IADE_EDILDI":       "İade edildi",
		"GONDERENE_TESLIM":  "Gönderene teslim edildi",
		"IPTAL":             "İptal edildi",
		"IPTAL_EDILDI":      "İptal edildi",
		"HASAR":             "Hasarlı",
		"KAYIP":             "Kayıp",
		"BEKLEMEDE":         "Beklemede",
	}

	if desc, ok := descriptions[normalized]; ok {
		return desc
	}

	// Return the original status if no description found
	return arasStatus
}

// MapServiceCode maps internal service codes to Aras service codes.
func MapServiceCode(serviceCode string) string {
	switch strings.ToLower(serviceCode) {
	case "standard", "standart":
		return "STANDART"
	case "express", "hizli":
		return "HIZLI"
	default:
		return strings.ToUpper(serviceCode)
	}
}

// MapLabelFormat maps internal label formats to Aras label formats.
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

// MapLabelFormatFromAras maps Aras label formats to internal label formats.
func MapLabelFormatFromAras(format string) domain.LabelFormat {
	switch strings.ToUpper(format) {
	case "PDF":
		return domain.LabelFormatPDF
	case "ZPL":
		return domain.LabelFormatZPL
	default:
		return domain.LabelFormatPDF
	}
}

// MapContentType returns the MIME content type for an Aras label format.
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

// ErrorCodeMapping maps Aras error codes to provider error codes.
var ErrorCodeMapping = map[string]string{
	"AUTH_FAILED":         "PROVIDER_AUTH_FAILED",
	"INVALID_CREDENTIALS": "PROVIDER_AUTH_FAILED",
	"INVALID_ADDRESS":     "PROVIDER_REJECTED",
	"INVALID_PARCEL":      "PROVIDER_REJECTED",
	"SERVICE_UNAVAILABLE": "PROVIDER_DOWN",
	"RATE_LIMITED":        "PROVIDER_RATE_LIMITED",
	"SHIPMENT_NOT_FOUND":  "PROVIDER_NOT_FOUND",
	"ALREADY_CANCELLED":   "PROVIDER_REJECTED",
	"CANNOT_CANCEL":       "PROVIDER_REJECTED",
	"INVALID_QUOTE":       "PROVIDER_REJECTED",
	"QUOTE_EXPIRED":       "PROVIDER_REJECTED",
}

// MapErrorCode maps an Aras error code to a normalized error code.
func MapErrorCode(arasErrorCode string) string {
	if code, ok := ErrorCodeMapping[arasErrorCode]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if an Aras error code indicates a retryable error.
func IsRetryableError(arasErrorCode string) bool {
	retryable := map[string]bool{
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMITED":        true,
		"TIMEOUT":             true,
		"CONNECTION_ERROR":    true,
	}
	return retryable[arasErrorCode]
}
