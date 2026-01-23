package surat

import (
	"strings"

	"github.com/e-port/shipping-aggregator/internal/domain"
)

// statusMapping maps Sürat status codes to normalized tracking statuses.
// Based on the provider-capabilities-matrix.md specification.
var statusMapping = map[string]domain.TrackingStatus{
	// Pickup/Acceptance statuses
	"KABUL":         domain.TrackingStatusPickedUp,
	"KARGO_KABUL":   domain.TrackingStatusPickedUp,
	"KARGO_ALINDI":  domain.TrackingStatusPickedUp,
	"TESLIM_ALINDI": domain.TrackingStatusPickedUp,

	// Transit statuses
	"TRANSFER":        domain.TrackingStatusInTransit,
	"TRANSFER_MERKEZ": domain.TrackingStatusInTransit,
	"AKTARMA":         domain.TrackingStatusInTransit,
	"YOLDA":           domain.TrackingStatusInTransit,
	"CIKIS":           domain.TrackingStatusInTransit,
	"HAREKET":         domain.TrackingStatusInTransit,

	// At hub/branch statuses
	"SUBEDE":            domain.TrackingStatusAtHub,
	"DAGITIM_SUBESINDE": domain.TrackingStatusAtHub,
	"SUBE_TESLIM":       domain.TrackingStatusAtHub,
	"DAGITIM_MERKEZI":   domain.TrackingStatusAtHub,
	"SUBE_GIRIS":        domain.TrackingStatusAtHub,

	// Out for delivery statuses
	"DAGITIMDA":        domain.TrackingStatusOutForDelivery,
	"DAGITIMA_CIKTI":   domain.TrackingStatusOutForDelivery,
	"KURYE_ZIMMETINDE": domain.TrackingStatusOutForDelivery,
	"DAGITIM_ARACINDA": domain.TrackingStatusOutForDelivery,

	// Delivered statuses
	"TESLIM":         domain.TrackingStatusDelivered,
	"TESLIM_EDILDI":  domain.TrackingStatusDelivered,
	"ALICIYA_TESLIM": domain.TrackingStatusDelivered,
	"TESLIM_OK":      domain.TrackingStatusDelivered,

	// Delivery failed statuses
	"TESLIM_EDILEMEDI": domain.TrackingStatusDeliveryFailed,
	"TESLIM_BASARISIZ": domain.TrackingStatusDeliveryFailed,
	"ADRES_BULUNAMADI": domain.TrackingStatusDeliveryFailed,
	"ALICI_YOK":        domain.TrackingStatusDeliveryFailed,
	"TESLIM_HATASI":    domain.TrackingStatusDeliveryFailed,

	// Return statuses
	"IADE":           domain.TrackingStatusReturning,
	"IADE_SURECINDE": domain.TrackingStatusReturning,
	"GONDERENE_IADE": domain.TrackingStatusReturning,
	"IADE_HAREKET":   domain.TrackingStatusReturning,

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
	"SORUNLU":   domain.TrackingStatusException,
}

// MapStatus maps a Sürat status code to a normalized tracking status.
// Returns TrackingStatusException for unknown status codes.
func MapStatus(suratStatus string) domain.TrackingStatus {
	// Normalize the status code (uppercase, trim spaces)
	normalized := strings.ToUpper(strings.TrimSpace(suratStatus))

	// Replace common variations
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	if status, ok := statusMapping[normalized]; ok {
		return status
	}

	// Try partial matching for common patterns
	if strings.Contains(normalized, "TESLIM_EDILDI") || strings.Contains(normalized, "TESLIM_OK") {
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

// MapStatusDescription returns a human-readable description for a Sürat status.
func MapStatusDescription(suratStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(suratStatus))

	descriptions := map[string]string{
		"KABUL":             "Kabul edildi",
		"KARGO_KABUL":       "Kargo kabul edildi",
		"KARGO_ALINDI":      "Kargo alındı",
		"TESLIM_ALINDI":     "Teslim alındı",
		"TRANSFER":          "Transfer aşamasında",
		"TRANSFER_MERKEZ":   "Transfer merkezinde",
		"AKTARMA":           "Aktarma merkezinde",
		"YOLDA":             "Yolda",
		"CIKIS":             "Çıkış yapıldı",
		"HAREKET":           "Hareket halinde",
		"SUBEDE":            "Şubede",
		"DAGITIM_SUBESINDE": "Dağıtım şubesinde",
		"SUBE_TESLIM":       "Şubeye teslim edildi",
		"DAGITIM_MERKEZI":   "Dağıtım merkezinde",
		"SUBE_GIRIS":        "Şubeye giriş yapıldı",
		"DAGITIMDA":         "Dağıtımda",
		"DAGITIMA_CIKTI":    "Dağıtıma çıktı",
		"KURYE_ZIMMETINDE":  "Kurye zimmetinde",
		"DAGITIM_ARACINDA":  "Dağıtım aracında",
		"TESLIM":            "Teslim edildi",
		"TESLIM_EDILDI":     "Teslim edildi",
		"ALICIYA_TESLIM":    "Alıcıya teslim edildi",
		"TESLIM_OK":         "Teslim başarılı",
		"TESLIM_EDILEMEDI":  "Teslim edilemedi",
		"TESLIM_BASARISIZ":  "Teslim başarısız",
		"ADRES_BULUNAMADI":  "Adres bulunamadı",
		"ALICI_YOK":         "Alıcı bulunamadı",
		"TESLIM_HATASI":     "Teslim hatası",
		"IADE":              "İade sürecinde",
		"IADE_SURECINDE":    "İade sürecinde",
		"GONDERENE_IADE":    "Gönderene iade ediliyor",
		"IADE_HAREKET":      "İade hareket halinde",
		"IADE_TESLIM":       "İade teslim edildi",
		"IADE_EDILDI":       "İade edildi",
		"GONDERENE_TESLIM":  "Gönderene teslim edildi",
		"IPTAL":             "İptal edildi",
		"IPTAL_EDILDI":      "İptal edildi",
		"HASAR":             "Hasarlı",
		"KAYIP":             "Kayıp",
		"BEKLEMEDE":         "Beklemede",
		"SORUNLU":           "Sorunlu gönderi",
	}

	if desc, ok := descriptions[normalized]; ok {
		return desc
	}

	// Return the original status if no description found
	return suratStatus
}

// MapServiceCode maps internal service codes to Sürat service codes.
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

// MapLabelFormat maps internal label formats to Sürat label formats.
// Note: Sürat only supports PDF format.
func MapLabelFormat(format domain.LabelFormat) string {
	// Sürat only supports PDF
	return LabelFormatPDF
}

// MapLabelFormatFromSurat maps Sürat label formats to internal label formats.
func MapLabelFormatFromSurat(format string) domain.LabelFormat {
	// Sürat only supports PDF
	return domain.LabelFormatPDF
}

// MapContentType returns the MIME content type for a Sürat label format.
func MapContentType(format string) string {
	// Sürat only supports PDF
	return "application/pdf"
}

// ErrorCodeMapping maps Sürat error codes to provider error codes.
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
	"INVALID_CITY":        "PROVIDER_REJECTED",
	"INVALID_DISTRICT":    "PROVIDER_REJECTED",
	"WEIGHT_EXCEEDED":     "PROVIDER_REJECTED",
	"DESI_EXCEEDED":       "PROVIDER_REJECTED",
	"COD_LIMIT_EXCEEDED":  "PROVIDER_REJECTED",
}

// MapErrorCode maps a Sürat error code to a normalized error code.
func MapErrorCode(suratErrorCode string) string {
	if code, ok := ErrorCodeMapping[suratErrorCode]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if a Sürat error code indicates a retryable error.
func IsRetryableError(suratErrorCode string) bool {
	retryable := map[string]bool{
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMITED":        true,
		"TIMEOUT":             true,
		"CONNECTION_ERROR":    true,
	}
	return retryable[suratErrorCode]
}
