package yurtici

import (
	"strings"

	"github.com/e-port/shipping-aggregator/internal/domain"
)

// statusMapping maps Yurtiçi status codes to normalized tracking statuses.
// Based on the provider-capabilities-matrix.md specification.
var statusMapping = map[string]domain.TrackingStatus{
	// Pickup/Acceptance statuses
	"KARGO_KABUL":    domain.TrackingStatusPickedUp,
	"KABUL":          domain.TrackingStatusPickedUp,
	"KARGO_ALINDI":   domain.TrackingStatusPickedUp,
	"TESLIM_ALINDI":  domain.TrackingStatusPickedUp,
	"MUSTERI_TESLIM": domain.TrackingStatusPickedUp,

	// Transit statuses
	"TRANSFER_MERKEZ": domain.TrackingStatusInTransit,
	"TRANSFER":        domain.TrackingStatusInTransit,
	"AKTARMA":         domain.TrackingStatusInTransit,
	"YOLDA":           domain.TrackingStatusInTransit,
	"CIKIS":           domain.TrackingStatusInTransit,
	"ARAC_YUKLEME":    domain.TrackingStatusInTransit,
	"HAT_CIKIS":       domain.TrackingStatusInTransit,

	// At hub/branch statuses
	"DAGITIM_SUBESINDE": domain.TrackingStatusAtHub,
	"SUBEDE":            domain.TrackingStatusAtHub,
	"SUBE_TESLIM":       domain.TrackingStatusAtHub,
	"DAGITIM_MERKEZI":   domain.TrackingStatusAtHub,
	"BIRIM_GIRIS":       domain.TrackingStatusAtHub,

	// Out for delivery statuses
	"DAGITIMDA":        domain.TrackingStatusOutForDelivery,
	"DAGITIMA_CIKTI":   domain.TrackingStatusOutForDelivery,
	"KURYE_ZIMMETINDE": domain.TrackingStatusOutForDelivery,
	"DAGITIM_ARACINDA": domain.TrackingStatusOutForDelivery,

	// Delivered statuses
	"TESLIM_EDILDI":  domain.TrackingStatusDelivered,
	"TESLIM":         domain.TrackingStatusDelivered,
	"ALICIYA_TESLIM": domain.TrackingStatusDelivered,
	"TESLIM_TAMAM":   domain.TrackingStatusDelivered,

	// Delivery failed statuses
	"TESLIM_EDILEMEDI": domain.TrackingStatusDeliveryFailed,
	"TESLIM_BASARISIZ": domain.TrackingStatusDeliveryFailed,
	"ADRES_BULUNAMADI": domain.TrackingStatusDeliveryFailed,
	"ALICI_YOK":        domain.TrackingStatusDeliveryFailed,
	"TESLIM_ENGEL":     domain.TrackingStatusDeliveryFailed,

	// Return statuses
	"IADE":           domain.TrackingStatusReturning,
	"IADE_SURECINDE": domain.TrackingStatusReturning,
	"GONDERENE_IADE": domain.TrackingStatusReturning,
	"IADE_TALEP":     domain.TrackingStatusReturning,

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

// MapStatus maps a Yurtiçi status code to a normalized tracking status.
// Returns TrackingStatusException for unknown status codes.
func MapStatus(yurticiStatus string) domain.TrackingStatus {
	// Normalize the status code (uppercase, trim spaces)
	normalized := strings.ToUpper(strings.TrimSpace(yurticiStatus))

	// Replace common variations
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	if status, ok := statusMapping[normalized]; ok {
		return status
	}

	// Try partial matching for common patterns
	if strings.Contains(normalized, "TESLIM_EDILDI") || strings.Contains(normalized, "TESLIM_TAMAM") {
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

// MapStatusDescription returns a human-readable description for a Yurtiçi status.
func MapStatusDescription(yurticiStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(yurticiStatus))

	descriptions := map[string]string{
		"KARGO_KABUL":       "Kargo kabul edildi",
		"KABUL":             "Kabul edildi",
		"KARGO_ALINDI":      "Kargo alındı",
		"TESLIM_ALINDI":     "Teslim alındı",
		"MUSTERI_TESLIM":    "Müşteriden teslim alındı",
		"TRANSFER_MERKEZ":   "Transfer merkezinde",
		"TRANSFER":          "Transfer aşamasında",
		"AKTARMA":           "Aktarma merkezinde",
		"YOLDA":             "Yolda",
		"CIKIS":             "Çıkış yapıldı",
		"ARAC_YUKLEME":      "Araca yüklendi",
		"HAT_CIKIS":         "Hat çıkışı yapıldı",
		"DAGITIM_SUBESINDE": "Dağıtım şubesinde",
		"SUBEDE":            "Şubede",
		"SUBE_TESLIM":       "Şubeye teslim edildi",
		"DAGITIM_MERKEZI":   "Dağıtım merkezinde",
		"BIRIM_GIRIS":       "Birime giriş yapıldı",
		"DAGITIMDA":         "Dağıtımda",
		"DAGITIMA_CIKTI":    "Dağıtıma çıktı",
		"KURYE_ZIMMETINDE":  "Kurye zimmetinde",
		"DAGITIM_ARACINDA":  "Dağıtım aracında",
		"TESLIM_EDILDI":     "Teslim edildi",
		"TESLIM":            "Teslim edildi",
		"ALICIYA_TESLIM":    "Alıcıya teslim edildi",
		"TESLIM_TAMAM":      "Teslim tamamlandı",
		"TESLIM_EDILEMEDI":  "Teslim edilemedi",
		"TESLIM_BASARISIZ":  "Teslim başarısız",
		"ADRES_BULUNAMADI":  "Adres bulunamadı",
		"ALICI_YOK":         "Alıcı bulunamadı",
		"TESLIM_ENGEL":      "Teslim engeli",
		"IADE":              "İade sürecinde",
		"IADE_SURECINDE":    "İade sürecinde",
		"GONDERENE_IADE":    "Gönderene iade ediliyor",
		"IADE_TALEP":        "İade talebi alındı",
		"IADE_TESLIM":       "İade teslim edildi",
		"IADE_EDILDI":       "İade edildi",
		"GONDERENE_TESLIM":  "Gönderene teslim edildi",
		"IPTAL":             "İptal edildi",
		"IPTAL_EDILDI":      "İptal edildi",
		"HASAR":             "Hasarlı",
		"KAYIP":             "Kayıp",
		"BEKLEMEDE":         "Beklemede",
		"SORUNLU":           "Sorunlu kargo",
	}

	if desc, ok := descriptions[normalized]; ok {
		return desc
	}

	// Return the original status if no description found
	return yurticiStatus
}

// MapProductCode maps internal service codes to Yurtiçi product codes.
func MapProductCode(serviceCode string) string {
	switch strings.ToLower(serviceCode) {
	case "standard", "standart":
		return ProductCodeStandard
	case "express", "ekspres", "hizli":
		return ProductCodeExpress
	case "same_day", "ayni_gun":
		return ProductCodeSameDay
	default:
		return strings.ToUpper(serviceCode)
	}
}

// MapLabelFormat maps internal label formats to Yurtiçi label formats.
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

// MapLabelFormatFromYurtici maps Yurtiçi label formats to internal label formats.
func MapLabelFormatFromYurtici(format string) domain.LabelFormat {
	switch strings.ToUpper(format) {
	case "PDF":
		return domain.LabelFormatPDF
	case "ZPL":
		return domain.LabelFormatZPL
	default:
		return domain.LabelFormatPDF
	}
}

// MapContentType returns the MIME content type for a Yurtiçi label format.
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

// MapCargoType maps parcel type to Yurtiçi cargo type.
func MapCargoType(isDocument bool) string {
	if isDocument {
		return CargoTypeDocument
	}
	return CargoTypeParcel
}

// ErrorCodeMapping maps Yurtiçi error codes to provider error codes.
var ErrorCodeMapping = map[string]string{
	"AUTH_FAILED":         "PROVIDER_AUTH_FAILED",
	"INVALID_CREDENTIALS": "PROVIDER_AUTH_FAILED",
	"INVALID_ADDRESS":     "PROVIDER_REJECTED",
	"INVALID_CARGO":       "PROVIDER_REJECTED",
	"SERVICE_UNAVAILABLE": "PROVIDER_DOWN",
	"RATE_LIMITED":        "PROVIDER_RATE_LIMITED",
	"CARGO_NOT_FOUND":     "PROVIDER_NOT_FOUND",
	"ALREADY_CANCELLED":   "PROVIDER_REJECTED",
	"CANNOT_CANCEL":       "PROVIDER_REJECTED",
	"INVALID_QUOTE":       "PROVIDER_REJECTED",
	"QUOTE_EXPIRED":       "PROVIDER_REJECTED",
	"INVALID_PRODUCT":     "PROVIDER_REJECTED",
}

// MapErrorCode maps a Yurtiçi error code to a normalized error code.
func MapErrorCode(yurticiErrorCode string) string {
	if code, ok := ErrorCodeMapping[yurticiErrorCode]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if a Yurtiçi error code indicates a retryable error.
func IsRetryableError(yurticiErrorCode string) bool {
	retryable := map[string]bool{
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMITED":        true,
		"TIMEOUT":             true,
		"CONNECTION_ERROR":    true,
	}
	return retryable[yurticiErrorCode]
}
