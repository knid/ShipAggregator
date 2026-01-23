package ptt

import (
	"strings"

	"github.com/knid/kargonet/internal/domain"
)

// statusMapping maps PTT status codes to normalized tracking statuses.
// Based on the provider-capabilities-matrix.md specification.
var statusMapping = map[string]domain.TrackingStatus{
	// Pickup/Acceptance statuses
	"KABUL_EDILDI":  domain.TrackingStatusPickedUp,
	"KABUL":         domain.TrackingStatusPickedUp,
	"KARGO_KABUL":   domain.TrackingStatusPickedUp,
	"KARGO_ALINDI":  domain.TrackingStatusPickedUp,
	"TESLIM_ALINDI": domain.TrackingStatusPickedUp,
	"POSTALANDI":    domain.TrackingStatusPickedUp,

	// Transit statuses
	"YOLDA":           domain.TrackingStatusInTransit,
	"TRANSFER":        domain.TrackingStatusInTransit,
	"TRANSFER_MERKEZ": domain.TrackingStatusInTransit,
	"AKTARMA":         domain.TrackingStatusInTransit,
	"CIKIS":           domain.TrackingStatusInTransit,
	"HAREKET":         domain.TrackingStatusInTransit,
	"SEVK_EDILDI":     domain.TrackingStatusInTransit,
	"GONDERILDI":      domain.TrackingStatusInTransit,

	// At hub/branch statuses
	"DAGITIM_MERKEZINDE": domain.TrackingStatusAtHub,
	"SUBEDE":             domain.TrackingStatusAtHub,
	"DAGITIM_SUBESINDE":  domain.TrackingStatusAtHub,
	"SUBE_TESLIM":        domain.TrackingStatusAtHub,
	"SUBE_GIRIS":         domain.TrackingStatusAtHub,
	"PTT_SUBESINDE":      domain.TrackingStatusAtHub,
	"MERKEZ_MUDURLUKTE":  domain.TrackingStatusAtHub,

	// Out for delivery statuses
	"DAGITIMA_CIKTI":   domain.TrackingStatusOutForDelivery,
	"DAGITIMDA":        domain.TrackingStatusOutForDelivery,
	"KURYE_ZIMMETINDE": domain.TrackingStatusOutForDelivery,
	"DAGITIM_ARACINDA": domain.TrackingStatusOutForDelivery,
	"TESLIMATA_CIKTI":  domain.TrackingStatusOutForDelivery,

	// Delivered statuses
	"TESLIM_EDILDI":  domain.TrackingStatusDelivered,
	"TESLIM":         domain.TrackingStatusDelivered,
	"ALICIYA_TESLIM": domain.TrackingStatusDelivered,
	"TESLIM_OK":      domain.TrackingStatusDelivered,
	"TESLIM_TAMAM":   domain.TrackingStatusDelivered,

	// Delivery failed statuses
	"TESLIM_EDILEMEDI": domain.TrackingStatusDeliveryFailed,
	"TESLIM_BASARISIZ": domain.TrackingStatusDeliveryFailed,
	"ADRES_BULUNAMADI": domain.TrackingStatusDeliveryFailed,
	"ALICI_YOK":        domain.TrackingStatusDeliveryFailed,
	"TESLIM_HATASI":    domain.TrackingStatusDeliveryFailed,
	"ALICI_BULUNAMADI": domain.TrackingStatusDeliveryFailed,
	"ADRES_HATALI":     domain.TrackingStatusDeliveryFailed,

	// Return statuses
	"IADE":           domain.TrackingStatusReturning,
	"IADE_SURECINDE": domain.TrackingStatusReturning,
	"GONDERENE_IADE": domain.TrackingStatusReturning,
	"IADE_HAREKET":   domain.TrackingStatusReturning,
	"IADE_YOLDA":     domain.TrackingStatusReturning,

	// Returned to sender statuses
	"IADE_EDILDI":      domain.TrackingStatusReturnedToSender,
	"IADE_TESLIM":      domain.TrackingStatusReturnedToSender,
	"GONDERENE_TESLIM": domain.TrackingStatusReturnedToSender,
	"IADE_TAMAMLANDI":  domain.TrackingStatusReturnedToSender,

	// Cancelled statuses
	"IPTAL":        domain.TrackingStatusCancelled,
	"IPTAL_EDILDI": domain.TrackingStatusCancelled,

	// Exception statuses
	"HASAR":     domain.TrackingStatusException,
	"KAYIP":     domain.TrackingStatusException,
	"BEKLEMEDE": domain.TrackingStatusException,
	"SORUNLU":   domain.TrackingStatusException,
	"MURACAAT":  domain.TrackingStatusException,
}

// MapStatus maps a PTT status code to a normalized tracking status.
// Returns TrackingStatusException for unknown status codes.
func MapStatus(pttStatus string) domain.TrackingStatus {
	// Normalize the status code (uppercase, trim spaces)
	normalized := strings.ToUpper(strings.TrimSpace(pttStatus))

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
	if strings.Contains(normalized, "TRANSFER") || strings.Contains(normalized, "AKTARMA") || strings.Contains(normalized, "YOLDA") {
		return domain.TrackingStatusInTransit
	}
	if strings.Contains(normalized, "IADE") {
		if strings.Contains(normalized, "TESLIM") || strings.Contains(normalized, "EDILDI") || strings.Contains(normalized, "TAMAMLANDI") {
			return domain.TrackingStatusReturnedToSender
		}
		return domain.TrackingStatusReturning
	}
	if strings.Contains(normalized, "IPTAL") {
		return domain.TrackingStatusCancelled
	}
	if strings.Contains(normalized, "KABUL") || strings.Contains(normalized, "ALINDI") {
		return domain.TrackingStatusPickedUp
	}
	if strings.Contains(normalized, "SUBE") || strings.Contains(normalized, "MERKEZ") {
		return domain.TrackingStatusAtHub
	}

	// Default to exception for unknown statuses
	return domain.TrackingStatusException
}

// MapStatusDescription returns a human-readable description for a PTT status.
func MapStatusDescription(pttStatus string) string {
	normalized := strings.ToUpper(strings.TrimSpace(pttStatus))

	descriptions := map[string]string{
		"KABUL_EDILDI":       "Kabul edildi",
		"KABUL":              "Kabul edildi",
		"KARGO_KABUL":        "Kargo kabul edildi",
		"KARGO_ALINDI":       "Kargo alındı",
		"TESLIM_ALINDI":      "Teslim alındı",
		"POSTALANDI":         "Postalandı",
		"YOLDA":              "Yolda",
		"TRANSFER":           "Transfer aşamasında",
		"TRANSFER_MERKEZ":    "Transfer merkezinde",
		"AKTARMA":            "Aktarma merkezinde",
		"CIKIS":              "Çıkış yapıldı",
		"HAREKET":            "Hareket halinde",
		"SEVK_EDILDI":        "Sevk edildi",
		"GONDERILDI":         "Gönderildi",
		"DAGITIM_MERKEZINDE": "Dağıtım merkezinde",
		"SUBEDE":             "Şubede",
		"DAGITIM_SUBESINDE":  "Dağıtım şubesinde",
		"SUBE_TESLIM":        "Şubeye teslim edildi",
		"SUBE_GIRIS":         "Şubeye giriş yapıldı",
		"PTT_SUBESINDE":      "PTT şubesinde",
		"MERKEZ_MUDURLUKTE":  "Merkez müdürlükte",
		"DAGITIMA_CIKTI":     "Dağıtıma çıktı",
		"DAGITIMDA":          "Dağıtımda",
		"KURYE_ZIMMETINDE":   "Kurye zimmetinde",
		"DAGITIM_ARACINDA":   "Dağıtım aracında",
		"TESLIMATA_CIKTI":    "Teslimata çıktı",
		"TESLIM_EDILDI":      "Teslim edildi",
		"TESLIM":             "Teslim edildi",
		"ALICIYA_TESLIM":     "Alıcıya teslim edildi",
		"TESLIM_OK":          "Teslim başarılı",
		"TESLIM_TAMAM":       "Teslim tamamlandı",
		"TESLIM_EDILEMEDI":   "Teslim edilemedi",
		"TESLIM_BASARISIZ":   "Teslim başarısız",
		"ADRES_BULUNAMADI":   "Adres bulunamadı",
		"ALICI_YOK":          "Alıcı bulunamadı",
		"TESLIM_HATASI":      "Teslim hatası",
		"ALICI_BULUNAMADI":   "Alıcı bulunamadı",
		"ADRES_HATALI":       "Adres hatalı",
		"IADE":               "İade sürecinde",
		"IADE_SURECINDE":     "İade sürecinde",
		"GONDERENE_IADE":     "Gönderene iade ediliyor",
		"IADE_HAREKET":       "İade hareket halinde",
		"IADE_YOLDA":         "İade yolda",
		"IADE_EDILDI":        "İade edildi",
		"IADE_TESLIM":        "İade teslim edildi",
		"GONDERENE_TESLIM":   "Gönderene teslim edildi",
		"IADE_TAMAMLANDI":    "İade tamamlandı",
		"IPTAL":              "İptal edildi",
		"IPTAL_EDILDI":       "İptal edildi",
		"HASAR":              "Hasarlı",
		"KAYIP":              "Kayıp",
		"BEKLEMEDE":          "Beklemede",
		"SORUNLU":            "Sorunlu gönderi",
		"MURACAAT":           "Müracaat bekleniyor",
	}

	if desc, ok := descriptions[normalized]; ok {
		return desc
	}

	// Return the original status if no description found
	return pttStatus
}

// MapServiceCode maps internal service codes to PTT service codes.
func MapServiceCode(serviceCode string) string {
	switch strings.ToLower(serviceCode) {
	case "standard", "standart", "normal":
		return "NORMAL"
	case "registered", "taahhutlu":
		return "TAAHHUTLU"
	default:
		return strings.ToUpper(serviceCode)
	}
}

// MapLabelFormat maps internal label formats to PTT label formats.
// Note: PTT only supports PDF format.
func MapLabelFormat(format domain.LabelFormat) string {
	// PTT only supports PDF
	return LabelFormatPDF
}

// MapLabelFormatFromPTT maps PTT label formats to internal label formats.
func MapLabelFormatFromPTT(format string) domain.LabelFormat {
	// PTT only supports PDF
	return domain.LabelFormatPDF
}

// MapContentType returns the MIME content type for a PTT label format.
func MapContentType(format string) string {
	// PTT only supports PDF
	return "application/pdf"
}

// ErrorCodeMapping maps PTT error codes to provider error codes.
var ErrorCodeMapping = map[string]string{
	"AUTH_FAILED":             "PROVIDER_AUTH_FAILED",
	"INVALID_CREDENTIALS":     "PROVIDER_AUTH_FAILED",
	"INVALID_SENDER_CODE":     "PROVIDER_AUTH_FAILED",
	"INVALID_ADDRESS":         "PROVIDER_REJECTED",
	"INVALID_PARCEL":          "PROVIDER_REJECTED",
	"SERVICE_UNAVAILABLE":     "PROVIDER_DOWN",
	"RATE_LIMITED":            "PROVIDER_RATE_LIMITED",
	"SHIPMENT_NOT_FOUND":      "PROVIDER_NOT_FOUND",
	"ALREADY_CANCELLED":       "PROVIDER_REJECTED",
	"CANNOT_CANCEL":           "PROVIDER_REJECTED",
	"INVALID_QUOTE":           "PROVIDER_REJECTED",
	"QUOTE_EXPIRED":           "PROVIDER_REJECTED",
	"INVALID_CITY":            "PROVIDER_REJECTED",
	"INVALID_DISTRICT":        "PROVIDER_REJECTED",
	"WEIGHT_EXCEEDED":         "PROVIDER_REJECTED",
	"DESI_EXCEEDED":           "PROVIDER_REJECTED",
	"COD_LIMIT_EXCEEDED":      "PROVIDER_REJECTED",
	"INSURANCE_NOT_SUPPORTED": "PROVIDER_REJECTED",
	"PICKUP_NOT_SUPPORTED":    "PROVIDER_REJECTED",
}

// MapErrorCode maps a PTT error code to a normalized error code.
func MapErrorCode(pttErrorCode string) string {
	if code, ok := ErrorCodeMapping[pttErrorCode]; ok {
		return code
	}
	return "PROVIDER_ERROR"
}

// IsRetryableError checks if a PTT error code indicates a retryable error.
func IsRetryableError(pttErrorCode string) bool {
	retryable := map[string]bool{
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMITED":        true,
		"TIMEOUT":             true,
		"CONNECTION_ERROR":    true,
	}
	return retryable[pttErrorCode]
}
