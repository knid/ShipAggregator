// Package phone provides Turkish phone number validation and formatting.
// It supports E.164 format (+905XXXXXXXXX) as required by the shipping aggregator.
//
// Turkish mobile phone numbers follow the format:
// - Country code: +90
// - Mobile prefix: 5XX (where XX is the operator code)
// - Subscriber number: 7 digits
// - Total: 12 digits including country code
//
// Turkish landline numbers follow the format:
// - Country code: +90
// - Area code: 2XX, 3XX, 4XX (where XX varies by region)
// - Subscriber number: 7 digits
// - Total: 12 digits including country code
package phone

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Turkish phone number constants
const (
	// CountryCode is the Turkish country code
	CountryCode = "90"

	// CountryCodeWithPlus is the Turkish country code with plus sign
	CountryCodeWithPlus = "+90"

	// MinLength is the minimum length of a Turkish phone number (without country code)
	MinLength = 10

	// MaxLength is the maximum length of a Turkish phone number (without country code)
	MaxLength = 10

	// E164Length is the length of a Turkish phone number in E.164 format
	E164Length = 12
)

// Errors
var (
	ErrInvalidPhone       = errors.New("phone: invalid phone number")
	ErrInvalidFormat      = errors.New("phone: invalid format")
	ErrInvalidCountryCode = errors.New("phone: invalid country code (must be +90)")
	ErrInvalidPrefix      = errors.New("phone: invalid prefix")
	ErrInvalidLength      = errors.New("phone: invalid length")
	ErrEmptyPhone         = errors.New("phone: empty phone number")
)

// Turkish mobile operator prefixes (5XX)
var mobileOperatorPrefixes = map[string]string{
	"530": "Turkcell",
	"531": "Turkcell",
	"532": "Turkcell",
	"533": "Turkcell",
	"534": "Turkcell",
	"535": "Turkcell",
	"536": "Turkcell",
	"537": "Turkcell",
	"538": "Turkcell",
	"539": "Turkcell",
	"540": "Turkcell", // M2M
	"541": "Turkcell", // M2M
	"542": "Vodafone",
	"543": "Vodafone",
	"544": "Vodafone",
	"545": "Vodafone",
	"546": "Vodafone",
	"547": "Vodafone",
	"548": "Vodafone",
	"549": "Vodafone",
	"550": "Türk Telekom",
	"551": "Türk Telekom",
	"552": "Türk Telekom",
	"553": "Türk Telekom",
	"554": "Türk Telekom",
	"555": "Türk Telekom",
	"556": "Türk Telekom",
	"557": "Türk Telekom",
	"558": "Türk Telekom",
	"559": "Türk Telekom",
	"501": "Turkcell", // MVNO
	"505": "Vodafone",
	"506": "Vodafone",
	"507": "Vodafone",
	"508": "Vodafone",
}

// Turkish landline area codes (first digit after 0)
var landlineAreaCodes = map[string]bool{
	"2": true, // Marmara, Thrace
	"3": true, // Central Anatolia, Black Sea
	"4": true, // Eastern Anatolia, Southeastern Anatolia
}

// Regular expressions for validation
var (
	// Matches digits only
	digitsOnlyRegex = regexp.MustCompile(`^\d+$`)

	// Matches E.164 format: +905XXXXXXXXX
	e164Regex = regexp.MustCompile(`^\+90[2-5]\d{9}$`)

	// Matches Turkish mobile: starts with 5
	mobileRegex = regexp.MustCompile(`^5\d{9}$`)

	// Matches Turkish landline: starts with 2, 3, or 4
	landlineRegex = regexp.MustCompile(`^[234]\d{9}$`)
)

// Phone represents a validated Turkish phone number in E.164 format.
type Phone string

// Parse parses and validates a Turkish phone number.
// Accepts various formats:
// - E.164: +905321234567
// - With country code: 905321234567
// - National: 05321234567
// - Without leading zero: 5321234567
// - With spaces/dashes: +90 532 123 45 67, 0532-123-4567
func Parse(input string) (Phone, error) {
	if input == "" {
		return "", ErrEmptyPhone
	}

	// Remove all non-digit characters except leading +
	cleaned := cleanPhoneNumber(input)

	// Handle different formats
	var normalized string

	switch {
	case strings.HasPrefix(cleaned, "+90"):
		// E.164 format: +905321234567
		normalized = cleaned[1:] // Remove +, keep 90...
	case strings.HasPrefix(cleaned, "0090"):
		// International format with 00: 00905321234567
		normalized = cleaned[2:] // Remove 00, keep 90...
	case strings.HasPrefix(cleaned, "90") && len(cleaned) >= 12:
		// With country code: 905321234567
		normalized = cleaned
	case strings.HasPrefix(cleaned, "0") && len(cleaned) == 11:
		// National format: 05321234567
		normalized = "90" + cleaned[1:]
	case len(cleaned) == 10 && (cleaned[0] == '5' || cleaned[0] == '2' || cleaned[0] == '3' || cleaned[0] == '4'):
		// Without leading zero: 5321234567 or 2121234567
		normalized = "90" + cleaned
	default:
		return "", ErrInvalidFormat
	}

	// Validate the normalized number
	if len(normalized) != 12 {
		return "", ErrInvalidLength
	}

	if !strings.HasPrefix(normalized, "90") {
		return "", ErrInvalidCountryCode
	}

	// Get the number without country code
	nationalNumber := normalized[2:]

	// Validate prefix (must start with 2, 3, 4, or 5)
	firstDigit := nationalNumber[0]
	if firstDigit != '2' && firstDigit != '3' && firstDigit != '4' && firstDigit != '5' {
		return "", ErrInvalidPrefix
	}

	// Create E.164 format
	e164 := "+" + normalized

	// Final validation with regex
	if !e164Regex.MatchString(e164) {
		return "", ErrInvalidPhone
	}

	return Phone(e164), nil
}

// MustParse parses a phone number and panics on error.
func MustParse(input string) Phone {
	p, err := Parse(input)
	if err != nil {
		panic(err)
	}
	return p
}

// cleanPhoneNumber removes all non-digit characters except leading +.
func cleanPhoneNumber(input string) string {
	var result strings.Builder
	result.Grow(len(input))

	for i, r := range input {
		if r == '+' && i == 0 {
			result.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// String returns the phone number in E.164 format.
func (p Phone) String() string {
	return string(p)
}

// E164 returns the phone number in E.164 format (same as String).
func (p Phone) E164() string {
	return string(p)
}

// National returns the phone number in national format (05321234567).
func (p Phone) National() string {
	if len(p) != 13 { // +90 + 10 digits
		return string(p)
	}
	return "0" + string(p[3:])
}

// NationalFormatted returns the phone number in formatted national format (0532 123 45 67).
func (p Phone) NationalFormatted() string {
	if len(p) != 13 {
		return string(p)
	}
	n := string(p[3:]) // Remove +90
	return fmt.Sprintf("0%s %s %s %s", n[0:3], n[3:6], n[6:8], n[8:10])
}

// International returns the phone number in international format (+90 532 123 45 67).
func (p Phone) International() string {
	if len(p) != 13 {
		return string(p)
	}
	n := string(p[3:]) // Remove +90
	return fmt.Sprintf("+90 %s %s %s %s", n[0:3], n[3:6], n[6:8], n[8:10])
}

// CountryCode returns the country code (90).
func (p Phone) CountryCode() string {
	return CountryCode
}

// NationalNumber returns the national number without country code.
func (p Phone) NationalNumber() string {
	if len(p) < 4 {
		return ""
	}
	return string(p[3:])
}

// Prefix returns the operator/area prefix (e.g., 532, 212).
func (p Phone) Prefix() string {
	if len(p) < 7 {
		return ""
	}
	return string(p[3:6])
}

// IsMobile returns true if this is a mobile phone number.
func (p Phone) IsMobile() bool {
	if len(p) < 4 {
		return false
	}
	return p[3] == '5'
}

// IsLandline returns true if this is a landline phone number.
func (p Phone) IsLandline() bool {
	if len(p) < 4 {
		return false
	}
	c := p[3]
	return c == '2' || c == '3' || c == '4'
}

// Operator returns the mobile operator name, or empty string for landlines.
func (p Phone) Operator() string {
	if !p.IsMobile() {
		return ""
	}
	prefix := p.Prefix()
	if op, ok := mobileOperatorPrefixes[prefix]; ok {
		return op
	}
	return "Unknown"
}

// IsValid returns true if the phone number is valid.
func (p Phone) IsValid() bool {
	return e164Regex.MatchString(string(p))
}

// IsEmpty returns true if the phone number is empty.
func (p Phone) IsEmpty() bool {
	return p == ""
}

// Equal returns true if two phone numbers are equal.
func (p Phone) Equal(other Phone) bool {
	return p == other
}

// Masked returns a masked version of the phone number for logging.
// Example: +9053****1234
func (p Phone) Masked() string {
	if len(p) < 8 {
		return "****"
	}
	return string(p[:5]) + "****" + string(p[len(p)-4:])
}

// MarshalJSON implements json.Marshaler.
func (p Phone) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(p))
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Phone) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if s == "" {
		*p = ""
		return nil
	}

	parsed, err := Parse(s)
	if err != nil {
		return err
	}

	*p = parsed
	return nil
}

// Scan implements sql.Scanner.
func (p *Phone) Scan(src interface{}) error {
	switch v := src.(type) {
	case string:
		if v == "" {
			*p = ""
			return nil
		}
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*p = parsed
		return nil
	case []byte:
		if len(v) == 0 {
			*p = ""
			return nil
		}
		parsed, err := Parse(string(v))
		if err != nil {
			return err
		}
		*p = parsed
		return nil
	case nil:
		*p = ""
		return nil
	default:
		return fmt.Errorf("phone: cannot scan type %T into Phone", src)
	}
}

// Value implements driver.Valuer.
func (p Phone) Value() (driver.Value, error) {
	if p == "" {
		return nil, nil
	}
	return string(p), nil
}

// Validate validates a phone number string without parsing.
// Returns nil if valid, error otherwise.
func Validate(input string) error {
	_, err := Parse(input)
	return err
}

// IsValidTurkishMobile checks if the input is a valid Turkish mobile number.
func IsValidTurkishMobile(input string) bool {
	p, err := Parse(input)
	if err != nil {
		return false
	}
	return p.IsMobile()
}

// IsValidTurkishLandline checks if the input is a valid Turkish landline number.
func IsValidTurkishLandline(input string) bool {
	p, err := Parse(input)
	if err != nil {
		return false
	}
	return p.IsLandline()
}

// FormatE164 formats a phone number to E.164 format.
// Returns empty string if invalid.
func FormatE164(input string) string {
	p, err := Parse(input)
	if err != nil {
		return ""
	}
	return p.E164()
}

// FormatNational formats a phone number to national format.
// Returns empty string if invalid.
func FormatNational(input string) string {
	p, err := Parse(input)
	if err != nil {
		return ""
	}
	return p.National()
}

// FormatInternational formats a phone number to international format.
// Returns empty string if invalid.
func FormatInternational(input string) string {
	p, err := Parse(input)
	if err != nil {
		return ""
	}
	return p.International()
}

// NullPhone represents a nullable phone number.
type NullPhone struct {
	Phone Phone
	Valid bool
}

// NewNullPhone creates a new NullPhone from a string.
func NewNullPhone(input string) NullPhone {
	if input == "" {
		return NullPhone{Valid: false}
	}
	p, err := Parse(input)
	if err != nil {
		return NullPhone{Valid: false}
	}
	return NullPhone{Phone: p, Valid: true}
}

// Scan implements sql.Scanner.
func (np *NullPhone) Scan(src interface{}) error {
	if src == nil {
		np.Phone = ""
		np.Valid = false
		return nil
	}

	var p Phone
	if err := p.Scan(src); err != nil {
		return err
	}

	np.Phone = p
	np.Valid = p != ""
	return nil
}

// Value implements driver.Valuer.
func (np NullPhone) Value() (driver.Value, error) {
	if !np.Valid {
		return nil, nil
	}
	return np.Phone.Value()
}

// MarshalJSON implements json.Marshaler.
func (np NullPhone) MarshalJSON() ([]byte, error) {
	if !np.Valid {
		return []byte("null"), nil
	}
	return np.Phone.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (np *NullPhone) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		np.Valid = false
		np.Phone = ""
		return nil
	}

	var p Phone
	if err := p.UnmarshalJSON(data); err != nil {
		return err
	}

	np.Phone = p
	np.Valid = p != ""
	return nil
}
