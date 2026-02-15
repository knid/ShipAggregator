package aras

import (
	"fmt"
	"strings"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/provider"
)

// ---------------------------------------------------------------------------
// Status code mapping — real Aras numeric DURUM_KODU from GetCargoTransaction
// ---------------------------------------------------------------------------

// statusCodeMapping maps Aras numeric status codes (DURUM_KODU) to normalized
// tracking statuses.
var statusCodeMapping = map[string]domain.TrackingStatus{
	"1": domain.TrackingStatusPickedUp,       // Kabul (at origin)
	"2": domain.TrackingStatusInTransit,       // Transfer (in transit)
	"3": domain.TrackingStatusAtHub,           // Dağıtım şubesinde (at delivery branch)
	"4": domain.TrackingStatusOutForDelivery,  // Dağıtımda (out for delivery)
	"5": domain.TrackingStatusDeliveryFailed,  // Kısmi teslim / Teslim edilemedi
	"6": domain.TrackingStatusDelivered,       // Teslim edildi
	"7": domain.TrackingStatusReturning,       // Yönlendirildi / İade
}

// transferCodeMapping maps Aras transfer codes (DEVIR_KODU) to tracking statuses
// for more specific status determination when DURUM_KODU alone isn't enough.
var transferCodeMapping = map[string]domain.TrackingStatus{
	"AY": domain.TrackingStatusDeliveryFailed,  // 10 - Adres yanlış/yetersiz
	"NT": domain.TrackingStatusDeliveryFailed,  // 11 - Uğrama notu bırakıldı
	"MD": domain.TrackingStatusOutForDelivery,  // 12 - Mobil dağıtım gününde teslimat
	"TL": domain.TrackingStatusDeliveryFailed,  // 14 - Tatil dolayısıyla teslimat yapılamadı
	"ŞA": domain.TrackingStatusAtHub,           // 15 - Müşteri kargosunu şubeden alacak
	"AD": domain.TrackingStatusDeliveryFailed,  // 17 - Alıcı adresi dağıtım alanı dışında
	"TG": domain.TrackingStatusDeliveryFailed,  // 18 - Teslimat gün içerisinde yapılamadı
	"AA": domain.TrackingStatusDeliveryFailed,  // 20 - Alıcı adreste tanınmıyor
	"HT": domain.TrackingStatusException,       // 22 - Hasarlı/tazminlik/sorunlu kargo
	"KG": domain.TrackingStatusInTransit,       // 26 - Kargo gümrükte
	"PR": domain.TrackingStatusAtHub,           // 32 - Pratik 7/24
	"ÜR": domain.TrackingStatusDeliveryFailed,  // 33 - Alıcı taşıma ücretini ödemeyi reddetti
	"KE": domain.TrackingStatusDeliveryFailed,  // 34 - Alıcı kargoyu kabul etmiyor
	"TT": domain.TrackingStatusOutForDelivery,  // 45 - Tercihli teslimat
	"PS": domain.TrackingStatusException,       // 47 - Parça sorunlu/teslim edilemedi
	"TO": domain.TrackingStatusDeliveryFailed,  // 51 - Toplumsal olaylar
}

// MapStatus maps an Aras status code to a normalized tracking status.
// Accepts numeric DURUM_KODU from GetCargoTransaction response.
func MapStatus(statusCode string) domain.TrackingStatus {
	code := strings.TrimSpace(statusCode)
	if status, ok := statusCodeMapping[code]; ok {
		return status
	}
	return domain.TrackingStatusException
}

// MapStatusWithTransferCode maps status using both DURUM_KODU and DEVIR_KODU
// for more precise determination.
func MapStatusWithTransferCode(statusCode, transferCode string) domain.TrackingStatus {
	tc := strings.TrimSpace(strings.ToUpper(transferCode))
	if tc != "" {
		if status, ok := transferCodeMapping[tc]; ok {
			return status
		}
	}
	return MapStatus(statusCode)
}

// MapStatusDescription returns a human-readable description for an Aras status code.
func MapStatusDescription(statusCode string) string {
	descriptions := map[string]string{
		"1": "Kargo kabul edildi",
		"2": "Transfer aşamasında",
		"3": "Dağıtım şubesinde",
		"4": "Dağıtımda",
		"5": "Teslim edilemedi",
		"6": "Teslim edildi",
		"7": "İade sürecinde",
	}

	code := strings.TrimSpace(statusCode)
	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return fmt.Sprintf("Durum kodu: %s", statusCode)
}

// ---------------------------------------------------------------------------
// Service code mapping
// ---------------------------------------------------------------------------

// MapServiceCode maps internal service codes to Aras service IDs.
// Aras uses numeric service IDs (e.g. "1" = normal, "2" = express).
func MapServiceCode(serviceCode string) string {
	switch strings.ToLower(serviceCode) {
	case "standard", "standart", "normal":
		return "1"
	case "express", "hizli":
		return "2"
	default:
		return serviceCode
	}
}

// ---------------------------------------------------------------------------
// Label format mapping
// ---------------------------------------------------------------------------

// MapLabelFormat maps internal label formats to Aras label formats.
func MapLabelFormat(format domain.LabelFormat) string {
	switch format {
	case domain.LabelFormatPDF:
		return "PDF"
	case domain.LabelFormatZPL:
		return "ZPL"
	case domain.LabelFormatPNG:
		return "PNG"
	default:
		return "PDF"
	}
}

// MapLabelFormatFromAras maps Aras label formats to internal label formats.
func MapLabelFormatFromAras(format string) domain.LabelFormat {
	switch strings.ToUpper(format) {
	case "PDF":
		return domain.LabelFormatPDF
	case "ZPL":
		return domain.LabelFormatZPL
	case "PNG":
		return domain.LabelFormatPNG
	default:
		return domain.LabelFormatPDF
	}
}

// MapContentType returns the MIME content type for a label format.
func MapContentType(format string) string {
	switch strings.ToUpper(format) {
	case "PDF":
		return "application/pdf"
	case "ZPL":
		return "application/x-zpl"
	case "PNG":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

// ---------------------------------------------------------------------------
// Error code mapping — real Aras result codes
// ---------------------------------------------------------------------------

// MapErrorCode maps an Aras result code to a normalized error code.
func MapErrorCode(resultCode string) string {
	switch resultCode {
	case "0":
		return "" // success
	case "-1", "-2":
		return "PROVIDER_AUTH_FAILED"
	case "-99":
		return "PROVIDER_ERROR"
	default:
		return "PROVIDER_REJECTED"
	}
}

// IsRetryableError checks if an Aras error indicates a retryable condition.
func IsRetryableError(resultCode string) bool {
	switch resultCode {
	case "-99": // generic server error
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// SetOrder result code mapping
// ---------------------------------------------------------------------------

// MapSetOrderResultCode maps a SetOrder ResultCode to an error.
// Returns nil for success (ResultCode "0").
func MapSetOrderResultCode(code, message string) error {
	code = strings.TrimSpace(code)
	if code == "" || code == "0" {
		return nil
	}
	return &provider.ProviderError{
		Code:            MapErrorCode(code),
		Message:         message,
		ProviderCode:    code,
		ProviderMessage: message,
		Retryable:       IsRetryableError(code),
	}
}

// ---------------------------------------------------------------------------
// CancelDispatch result code mapping
// ---------------------------------------------------------------------------

// MapCancelDispatchResultCode maps a CancelDispatch ResultCode to an error.
// Returns nil for success (ResultCode "0" or "1").
// Result codes per Aras docs:
//
//	0  = Success
//	1  = "{orderCode} successfully deleted"
//	-1 = Record not found
//	-2 = Invalid username/password
//	999 = Cannot cancel — waybill already issued
//	936 = Error
func MapCancelDispatchResultCode(code, message string) error {
	code = strings.TrimSpace(code)
	switch code {
	case "0", "1":
		return nil // success
	case "-2":
		return &provider.ProviderError{
			Code:            "PROVIDER_AUTH_FAILED",
			Message:         message,
			ProviderCode:    code,
			ProviderMessage: message,
			Retryable:       false,
		}
	case "999":
		return &provider.ProviderError{
			Code:            "PROVIDER_REJECTED",
			Message:         "cannot cancel — waybill already issued at branch",
			ProviderCode:    code,
			ProviderMessage: message,
			Retryable:       false,
		}
	case "-1":
		return &provider.ProviderError{
			Code:            "PROVIDER_REJECTED",
			Message:         "shipment not found",
			ProviderCode:    code,
			ProviderMessage: message,
			Retryable:       false,
		}
	default:
		return &provider.ProviderError{
			Code:            MapErrorCode(code),
			Message:         message,
			ProviderCode:    code,
			ProviderMessage: message,
			Retryable:       IsRetryableError(code),
		}
	}
}
