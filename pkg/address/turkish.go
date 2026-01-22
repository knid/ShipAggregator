// Package address provides Turkish address validation and normalization utilities.
// It supports validation of Turkish postal codes, city codes, and address formatting.
//
// Turkish address structure:
// - City (İl): 81 provinces, identified by plate codes (01-81)
// - District (İlçe): Sub-divisions of cities
// - Neighborhood (Mahalle): Sub-divisions of districts
// - Street (Sokak/Cadde): Street name
// - Building (Bina No): Building number
// - Postal Code: 5-digit code
package address

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Errors
var (
	ErrInvalidPostalCode   = errors.New("address: invalid postal code")
	ErrInvalidCityCode     = errors.New("address: invalid city code")
	ErrInvalidDistrictCode = errors.New("address: invalid district code")
	ErrEmptyStreetAddress  = errors.New("address: empty street address")
	ErrEmptyContactName    = errors.New("address: empty contact name")
	ErrAddressTooLong      = errors.New("address: address too long")
	ErrInvalidCharacters   = errors.New("address: invalid characters")
)

// Address length limits
const (
	MaxStreetLineLength  = 500
	MaxContactNameLength = 255
	MaxCompanyNameLength = 255
	MaxLabelLength       = 100
	MinStreetLineLength  = 5
	MinContactNameLength = 2
)

// Regular expressions
var (
	// Turkish postal code: 5 digits, first two are city code (01-81)
	postalCodeRegex = regexp.MustCompile(`^(0[1-9]|[1-7][0-9]|8[01])\d{3}$`)

	// City code: 01-81
	cityCodeRegex = regexp.MustCompile(`^(0[1-9]|[1-7][0-9]|8[01])$`)

	// Valid address characters: letters, numbers, spaces, common punctuation
	validAddressCharsRegex = regexp.MustCompile(`^[\p{L}\p{N}\s\.,\-\/\(\)#&'"\:;]+$`)

	// Valid name characters: letters, spaces, apostrophe, hyphen
	validNameCharsRegex = regexp.MustCompile(`^[\p{L}\s'\-\.]+$`)
)

// Turkish cities by plate code
var turkishCities = map[string]string{
	"01": "Adana",
	"02": "Adıyaman",
	"03": "Afyonkarahisar",
	"04": "Ağrı",
	"05": "Amasya",
	"06": "Ankara",
	"07": "Antalya",
	"08": "Artvin",
	"09": "Aydın",
	"10": "Balıkesir",
	"11": "Bilecik",
	"12": "Bingöl",
	"13": "Bitlis",
	"14": "Bolu",
	"15": "Burdur",
	"16": "Bursa",
	"17": "Çanakkale",
	"18": "Çankırı",
	"19": "Çorum",
	"20": "Denizli",
	"21": "Diyarbakır",
	"22": "Edirne",
	"23": "Elazığ",
	"24": "Erzincan",
	"25": "Erzurum",
	"26": "Eskişehir",
	"27": "Gaziantep",
	"28": "Giresun",
	"29": "Gümüşhane",
	"30": "Hakkari",
	"31": "Hatay",
	"32": "Isparta",
	"33": "Mersin",
	"34": "İstanbul",
	"35": "İzmir",
	"36": "Kars",
	"37": "Kastamonu",
	"38": "Kayseri",
	"39": "Kırklareli",
	"40": "Kırşehir",
	"41": "Kocaeli",
	"42": "Konya",
	"43": "Kütahya",
	"44": "Malatya",
	"45": "Manisa",
	"46": "Kahramanmaraş",
	"47": "Mardin",
	"48": "Muğla",
	"49": "Muş",
	"50": "Nevşehir",
	"51": "Niğde",
	"52": "Ordu",
	"53": "Rize",
	"54": "Sakarya",
	"55": "Samsun",
	"56": "Siirt",
	"57": "Sinop",
	"58": "Sivas",
	"59": "Tekirdağ",
	"60": "Tokat",
	"61": "Trabzon",
	"62": "Tunceli",
	"63": "Şanlıurfa",
	"64": "Uşak",
	"65": "Van",
	"66": "Yozgat",
	"67": "Zonguldak",
	"68": "Aksaray",
	"69": "Bayburt",
	"70": "Karaman",
	"71": "Kırıkkale",
	"72": "Batman",
	"73": "Şırnak",
	"74": "Bartın",
	"75": "Ardahan",
	"76": "Iğdır",
	"77": "Yalova",
	"78": "Karabük",
	"79": "Kilis",
	"80": "Osmaniye",
	"81": "Düzce",
}

// Turkish character mappings for normalization
var turkishToASCII = map[rune]rune{
	'ç': 'c', 'Ç': 'C',
	'ğ': 'g', 'Ğ': 'G',
	'ı': 'i', 'I': 'I',
	'i': 'i', 'İ': 'I',
	'ö': 'o', 'Ö': 'O',
	'ş': 's', 'Ş': 'S',
	'ü': 'u', 'Ü': 'U',
}

// ValidatePostalCode validates a Turkish postal code.
// Turkish postal codes are 5 digits where the first two digits
// represent the city code (01-81).
func ValidatePostalCode(code string) error {
	code = strings.TrimSpace(code)
	if !postalCodeRegex.MatchString(code) {
		return ErrInvalidPostalCode
	}
	return nil
}

// IsValidPostalCode returns true if the postal code is valid.
func IsValidPostalCode(code string) bool {
	return ValidatePostalCode(code) == nil
}

// ValidateCityCode validates a Turkish city (plate) code.
// Valid codes are 01-81.
func ValidateCityCode(code string) error {
	code = strings.TrimSpace(code)
	if !cityCodeRegex.MatchString(code) {
		return ErrInvalidCityCode
	}
	return nil
}

// IsValidCityCode returns true if the city code is valid.
func IsValidCityCode(code string) bool {
	return ValidateCityCode(code) == nil
}

// GetCityName returns the city name for a given city code.
// Returns empty string if the code is invalid.
func GetCityName(code string) string {
	code = strings.TrimSpace(code)
	// Pad with leading zero if needed
	if len(code) == 1 {
		code = "0" + code
	}
	return turkishCities[code]
}

// GetCityCode returns the city code for a given city name.
// Returns empty string if the city is not found.
// The search is case-insensitive and handles Turkish characters.
func GetCityCode(name string) string {
	name = strings.TrimSpace(name)
	normalizedName := NormalizeText(name)

	for code, cityName := range turkishCities {
		if NormalizeText(cityName) == normalizedName {
			return code
		}
	}
	return ""
}

// GetAllCities returns a map of all Turkish cities (code -> name).
func GetAllCities() map[string]string {
	result := make(map[string]string, len(turkishCities))
	for k, v := range turkishCities {
		result[k] = v
	}
	return result
}

// CityCodeFromPostalCode extracts the city code from a postal code.
// Returns empty string if the postal code is invalid.
func CityCodeFromPostalCode(postalCode string) string {
	postalCode = strings.TrimSpace(postalCode)
	if len(postalCode) < 2 {
		return ""
	}
	cityCode := postalCode[:2]
	if IsValidCityCode(cityCode) {
		return cityCode
	}
	return ""
}

// ValidateStreetAddress validates a street address line.
func ValidateStreetAddress(address string) error {
	address = strings.TrimSpace(address)

	if address == "" {
		return ErrEmptyStreetAddress
	}

	if len(address) < MinStreetLineLength {
		return fmt.Errorf("address: street address too short (minimum %d characters)", MinStreetLineLength)
	}

	if len(address) > MaxStreetLineLength {
		return ErrAddressTooLong
	}

	if !validAddressCharsRegex.MatchString(address) {
		return ErrInvalidCharacters
	}

	return nil
}

// ValidateContactName validates a contact name.
func ValidateContactName(name string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return ErrEmptyContactName
	}

	if len(name) < MinContactNameLength {
		return fmt.Errorf("address: contact name too short (minimum %d characters)", MinContactNameLength)
	}

	if len(name) > MaxContactNameLength {
		return fmt.Errorf("address: contact name too long (maximum %d characters)", MaxContactNameLength)
	}

	if !validNameCharsRegex.MatchString(name) {
		return ErrInvalidCharacters
	}

	return nil
}

// NormalizeText normalizes Turkish text for comparison.
// Converts to lowercase and replaces Turkish characters with ASCII equivalents.
func NormalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	// Replace Turkish characters
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		if ascii, ok := turkishToASCII[r]; ok {
			result.WriteRune(ascii)
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// NormalizeTextUnicode normalizes text using Unicode normalization.
// This is more thorough but slower than NormalizeText.
func NormalizeTextUnicode(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	// Use Unicode normalization to decompose characters
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)

	// Also apply Turkish-specific mappings
	var sb strings.Builder
	sb.Grow(len(result))

	for _, r := range result {
		if ascii, ok := turkishToASCII[r]; ok {
			sb.WriteRune(ascii)
		} else {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

// FormatAddress formats an address for display.
func FormatAddress(streetLine1, streetLine2, neighborhood, district, city, postalCode string) string {
	var parts []string

	if streetLine1 = strings.TrimSpace(streetLine1); streetLine1 != "" {
		parts = append(parts, streetLine1)
	}
	if streetLine2 = strings.TrimSpace(streetLine2); streetLine2 != "" {
		parts = append(parts, streetLine2)
	}
	if neighborhood = strings.TrimSpace(neighborhood); neighborhood != "" {
		parts = append(parts, neighborhood)
	}

	// District and city on same line
	var locationParts []string
	if district = strings.TrimSpace(district); district != "" {
		locationParts = append(locationParts, district)
	}
	if city = strings.TrimSpace(city); city != "" {
		locationParts = append(locationParts, city)
	}
	if len(locationParts) > 0 {
		parts = append(parts, strings.Join(locationParts, ", "))
	}

	if postalCode = strings.TrimSpace(postalCode); postalCode != "" {
		parts = append(parts, postalCode)
	}

	return strings.Join(parts, "\n")
}

// FormatAddressSingleLine formats an address as a single line.
func FormatAddressSingleLine(streetLine1, streetLine2, neighborhood, district, city, postalCode string) string {
	var parts []string

	if streetLine1 = strings.TrimSpace(streetLine1); streetLine1 != "" {
		parts = append(parts, streetLine1)
	}
	if streetLine2 = strings.TrimSpace(streetLine2); streetLine2 != "" {
		parts = append(parts, streetLine2)
	}
	if neighborhood = strings.TrimSpace(neighborhood); neighborhood != "" {
		parts = append(parts, neighborhood)
	}
	if district = strings.TrimSpace(district); district != "" {
		parts = append(parts, district)
	}
	if city = strings.TrimSpace(city); city != "" {
		parts = append(parts, city)
	}
	if postalCode = strings.TrimSpace(postalCode); postalCode != "" {
		parts = append(parts, postalCode)
	}

	return strings.Join(parts, ", ")
}

// Address represents a Turkish address.
type Address struct {
	ContactName      string  `json:"contact_name"`
	CompanyName      string  `json:"company_name,omitempty"`
	Phone            string  `json:"phone"`
	Email            string  `json:"email,omitempty"`
	StreetLine1      string  `json:"street_line_1"`
	StreetLine2      string  `json:"street_line_2,omitempty"`
	NeighborhoodCode string  `json:"neighborhood_code"`
	NeighborhoodName string  `json:"neighborhood_name,omitempty"`
	DistrictCode     string  `json:"district_code"`
	DistrictName     string  `json:"district_name,omitempty"`
	CityCode         string  `json:"city_code"`
	CityName         string  `json:"city_name,omitempty"`
	PostalCode       string  `json:"postal_code,omitempty"`
	Latitude         float64 `json:"latitude,omitempty"`
	Longitude        float64 `json:"longitude,omitempty"`
}

// Validate validates the address.
func (a *Address) Validate() error {
	var errs []error

	// Validate contact name
	if err := ValidateContactName(a.ContactName); err != nil {
		errs = append(errs, fmt.Errorf("contact_name: %w", err))
	}

	// Validate company name if provided
	if a.CompanyName != "" && len(a.CompanyName) > MaxCompanyNameLength {
		errs = append(errs, fmt.Errorf("company_name: too long (maximum %d characters)", MaxCompanyNameLength))
	}

	// Validate street address
	if err := ValidateStreetAddress(a.StreetLine1); err != nil {
		errs = append(errs, fmt.Errorf("street_line_1: %w", err))
	}

	// Validate street line 2 if provided
	if a.StreetLine2 != "" {
		if len(a.StreetLine2) > MaxStreetLineLength {
			errs = append(errs, fmt.Errorf("street_line_2: too long (maximum %d characters)", MaxStreetLineLength))
		}
	}

	// Validate city code
	if err := ValidateCityCode(a.CityCode); err != nil {
		errs = append(errs, fmt.Errorf("city_code: %w", err))
	}

	// Validate postal code if provided
	if a.PostalCode != "" {
		if err := ValidatePostalCode(a.PostalCode); err != nil {
			errs = append(errs, fmt.Errorf("postal_code: %w", err))
		}

		// Check postal code matches city code
		if a.CityCode != "" {
			postalCityCode := CityCodeFromPostalCode(a.PostalCode)
			if postalCityCode != "" && postalCityCode != a.CityCode {
				errs = append(errs, fmt.Errorf("postal_code: does not match city_code (expected %s, got %s)", a.CityCode, postalCityCode))
			}
		}
	}

	// Validate coordinates if provided
	if a.Latitude != 0 || a.Longitude != 0 {
		if !IsValidTurkeyCoordinates(a.Latitude, a.Longitude) {
			errs = append(errs, errors.New("coordinates: outside Turkey boundaries"))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Format returns the formatted address for display.
func (a *Address) Format() string {
	city := a.CityName
	if city == "" {
		city = GetCityName(a.CityCode)
	}

	district := a.DistrictName
	neighborhood := a.NeighborhoodName

	return FormatAddress(a.StreetLine1, a.StreetLine2, neighborhood, district, city, a.PostalCode)
}

// FormatSingleLine returns the address as a single line.
func (a *Address) FormatSingleLine() string {
	city := a.CityName
	if city == "" {
		city = GetCityName(a.CityCode)
	}

	district := a.DistrictName
	neighborhood := a.NeighborhoodName

	return FormatAddressSingleLine(a.StreetLine1, a.StreetLine2, neighborhood, district, city, a.PostalCode)
}

// MaskedStreetLine returns a masked version of the street address for logging.
func (a *Address) MaskedStreetLine() string {
	if len(a.StreetLine1) <= 6 {
		return "***"
	}
	return a.StreetLine1[:3] + "***"
}

// MaskedContactName returns a masked version of the contact name for logging.
func (a *Address) MaskedContactName() string {
	if len(a.ContactName) <= 3 {
		return "***"
	}
	return a.ContactName[:3] + "***"
}

// Turkey geographic boundaries
const (
	TurkeyMinLatitude  = 35.8
	TurkeyMaxLatitude  = 42.1
	TurkeyMinLongitude = 26.0
	TurkeyMaxLongitude = 44.8
)

// IsValidTurkeyCoordinates checks if coordinates are within Turkey's boundaries.
func IsValidTurkeyCoordinates(lat, lng float64) bool {
	return lat >= TurkeyMinLatitude && lat <= TurkeyMaxLatitude &&
		lng >= TurkeyMinLongitude && lng <= TurkeyMaxLongitude
}

// ValidateCoordinates validates that coordinates are within Turkey.
func ValidateCoordinates(lat, lng float64) error {
	if !IsValidTurkeyCoordinates(lat, lng) {
		return fmt.Errorf("coordinates (%.6f, %.6f) are outside Turkey boundaries", lat, lng)
	}
	return nil
}

// DistrictCodeFormat represents the format of district codes.
// Format: {city_code}-{district_number} e.g., "34-01" for Adalar, Istanbul
type DistrictCodeFormat struct {
	CityCode       string
	DistrictNumber string
}

// ParseDistrictCode parses a district code into its components.
func ParseDistrictCode(code string) (*DistrictCodeFormat, error) {
	parts := strings.Split(code, "-")
	if len(parts) != 2 {
		return nil, ErrInvalidDistrictCode
	}

	if err := ValidateCityCode(parts[0]); err != nil {
		return nil, ErrInvalidDistrictCode
	}

	return &DistrictCodeFormat{
		CityCode:       parts[0],
		DistrictNumber: parts[1],
	}, nil
}

// String returns the district code as a string.
func (d *DistrictCodeFormat) String() string {
	return d.CityCode + "-" + d.DistrictNumber
}

// NeighborhoodCodeFormat represents the format of neighborhood codes.
// Format: {district_code}-{neighborhood_number} e.g., "34-01-001"
type NeighborhoodCodeFormat struct {
	CityCode           string
	DistrictNumber     string
	NeighborhoodNumber string
}

// ParseNeighborhoodCode parses a neighborhood code into its components.
func ParseNeighborhoodCode(code string) (*NeighborhoodCodeFormat, error) {
	parts := strings.Split(code, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("address: invalid neighborhood code format")
	}

	if err := ValidateCityCode(parts[0]); err != nil {
		return nil, fmt.Errorf("address: invalid city code in neighborhood code")
	}

	return &NeighborhoodCodeFormat{
		CityCode:           parts[0],
		DistrictNumber:     parts[1],
		NeighborhoodNumber: parts[2],
	}, nil
}

// String returns the neighborhood code as a string.
func (n *NeighborhoodCodeFormat) String() string {
	return n.CityCode + "-" + n.DistrictNumber + "-" + n.NeighborhoodNumber
}

// DistrictCode returns the district code portion.
func (n *NeighborhoodCodeFormat) DistrictCode() string {
	return n.CityCode + "-" + n.DistrictNumber
}
