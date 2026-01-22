// Package money provides a type-safe money representation using integer kuruş (1 TRY = 100 kuruş).
// This avoids floating-point precision issues that are common with monetary calculations.
//
// All monetary values in the system are stored and calculated as integer kuruş.
// This follows the constitution requirement: "No floating-point for money, immutable ledger"
package money

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Kurus represents a monetary amount in Turkish kuruş (1/100 of a TRY).
// Using int64 allows for amounts up to approximately 92 quadrillion kuruş,
// which is more than sufficient for any practical use case.
type Kurus int64

// Common monetary constants
const (
	// Zero represents zero kuruş
	Zero Kurus = 0

	// OneKurus represents 1 kuruş
	OneKurus Kurus = 1

	// OneLira represents 1 TRY (100 kuruş)
	OneLira Kurus = 100

	// MaxKurus is the maximum safe value for monetary calculations
	MaxKurus Kurus = math.MaxInt64 / 2

	// MinKurus is the minimum safe value for monetary calculations
	MinKurus Kurus = math.MinInt64 / 2
)

// Currency codes
const (
	CurrencyTRY = "TRY"
)

// Errors
var (
	ErrOverflow          = errors.New("money: arithmetic overflow")
	ErrUnderflow         = errors.New("money: arithmetic underflow")
	ErrNegativeAmount    = errors.New("money: negative amount not allowed")
	ErrInvalidFormat     = errors.New("money: invalid format")
	ErrDivisionByZero    = errors.New("money: division by zero")
	ErrInvalidPercentage = errors.New("money: invalid percentage")
)

// FromLira creates a Kurus value from a TRY amount.
// For example, FromLira(10) returns 1000 kuruş.
func FromLira(lira int64) Kurus {
	return Kurus(lira * 100)
}

// FromLiraKurus creates a Kurus value from TRY and kuruş components.
// For example, FromLiraKurus(10, 50) returns 1050 kuruş (10.50 TRY).
func FromLiraKurus(lira, kurus int64) Kurus {
	return Kurus(lira*100 + kurus)
}

// FromFloat creates a Kurus value from a float64 TRY amount.
// The value is rounded to the nearest kuruş.
// Use with caution - prefer integer-based constructors when possible.
func FromFloat(amount float64) Kurus {
	return Kurus(math.Round(amount * 100))
}

// ParseTRY parses a string representation of TRY amount.
// Accepts formats like "10.50", "10,50", "1050" (kuruş), "10.5", "10".
func ParseTRY(s string) (Kurus, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidFormat
	}

	// Remove currency symbol if present
	s = strings.TrimPrefix(s, "₺")
	s = strings.TrimPrefix(s, "TRY")
	s = strings.TrimSpace(s)

	// Remove thousand separators (dots in Turkish format, commas in US format)
	// First, determine the format
	lastDot := strings.LastIndex(s, ".")
	lastComma := strings.LastIndex(s, ",")

	var intPart, decPart string

	if lastDot > lastComma {
		// Dot is decimal separator (US format: 1,234.56)
		s = strings.ReplaceAll(s, ",", "")
		parts := strings.Split(s, ".")
		if len(parts) > 2 {
			return 0, ErrInvalidFormat
		}
		intPart = parts[0]
		if len(parts) == 2 {
			decPart = parts[1]
		}
	} else if lastComma > lastDot {
		// Comma is decimal separator (Turkish format: 1.234,56)
		s = strings.ReplaceAll(s, ".", "")
		parts := strings.Split(s, ",")
		if len(parts) > 2 {
			return 0, ErrInvalidFormat
		}
		intPart = parts[0]
		if len(parts) == 2 {
			decPart = parts[1]
		}
	} else {
		// No decimal separator
		intPart = s
	}

	// Parse integer part
	lira, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, ErrInvalidFormat
	}

	// Parse decimal part
	var kurus int64
	if decPart != "" {
		// Pad or truncate to 2 digits
		if len(decPart) == 1 {
			decPart += "0"
		} else if len(decPart) > 2 {
			// Round the third digit
			if decPart[2] >= '5' {
				decPart = decPart[:2]
				k, _ := strconv.ParseInt(decPart, 10, 64)
				kurus = k + 1
				if kurus >= 100 {
					lira++
					kurus = 0
				}
			} else {
				decPart = decPart[:2]
				kurus, _ = strconv.ParseInt(decPart, 10, 64)
			}
		} else {
			kurus, err = strconv.ParseInt(decPart, 10, 64)
			if err != nil {
				return 0, ErrInvalidFormat
			}
		}
	}

	return FromLiraKurus(lira, kurus), nil
}

// Lira returns the whole TRY amount (truncated).
func (k Kurus) Lira() int64 {
	return int64(k) / 100
}

// KurusPart returns the kuruş part (0-99).
func (k Kurus) KurusPart() int64 {
	kurus := int64(k) % 100
	if kurus < 0 {
		kurus = -kurus
	}
	return kurus
}

// Float64 returns the amount as a float64 TRY value.
// Use with caution for display purposes only.
func (k Kurus) Float64() float64 {
	return float64(k) / 100.0
}

// String returns a string representation in TRY format (e.g., "10.50").
func (k Kurus) String() string {
	sign := ""
	val := int64(k)
	if val < 0 {
		sign = "-"
		val = -val
	}
	return fmt.Sprintf("%s%d.%02d", sign, val/100, val%100)
}

// StringTR returns a string representation in Turkish format (e.g., "10,50 ₺").
func (k Kurus) StringTR() string {
	sign := ""
	val := int64(k)
	if val < 0 {
		sign = "-"
		val = -val
	}
	return fmt.Sprintf("%s%d,%02d ₺", sign, val/100, val%100)
}

// StringWithCurrency returns a string with currency code (e.g., "10.50 TRY").
func (k Kurus) StringWithCurrency() string {
	return fmt.Sprintf("%s TRY", k.String())
}

// IsZero returns true if the amount is zero.
func (k Kurus) IsZero() bool {
	return k == 0
}

// IsPositive returns true if the amount is greater than zero.
func (k Kurus) IsPositive() bool {
	return k > 0
}

// IsNegative returns true if the amount is less than zero.
func (k Kurus) IsNegative() bool {
	return k < 0
}

// Abs returns the absolute value.
func (k Kurus) Abs() Kurus {
	if k < 0 {
		return -k
	}
	return k
}

// Negate returns the negated value.
func (k Kurus) Negate() Kurus {
	return -k
}

// Add adds two amounts and returns the result.
// Returns an error if the result would overflow.
func (k Kurus) Add(other Kurus) (Kurus, error) {
	result := k + other
	// Check for overflow
	if (other > 0 && result < k) || (other < 0 && result > k) {
		return 0, ErrOverflow
	}
	return result, nil
}

// MustAdd adds two amounts and panics on overflow.
func (k Kurus) MustAdd(other Kurus) Kurus {
	result, err := k.Add(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Sub subtracts an amount and returns the result.
// Returns an error if the result would underflow.
func (k Kurus) Sub(other Kurus) (Kurus, error) {
	result := k - other
	// Check for underflow
	if (other > 0 && result > k) || (other < 0 && result < k) {
		return 0, ErrUnderflow
	}
	return result, nil
}

// MustSub subtracts an amount and panics on underflow.
func (k Kurus) MustSub(other Kurus) Kurus {
	result, err := k.Sub(other)
	if err != nil {
		panic(err)
	}
	return result
}

// Mul multiplies by an integer factor.
// Returns an error if the result would overflow.
func (k Kurus) Mul(factor int64) (Kurus, error) {
	if factor == 0 {
		return 0, nil
	}
	result := Kurus(int64(k) * factor)
	// Check for overflow
	if result/Kurus(factor) != k {
		return 0, ErrOverflow
	}
	return result, nil
}

// MustMul multiplies by an integer factor and panics on overflow.
func (k Kurus) MustMul(factor int64) Kurus {
	result, err := k.Mul(factor)
	if err != nil {
		panic(err)
	}
	return result
}

// Div divides by an integer divisor (truncates toward zero).
// Returns an error if divisor is zero.
func (k Kurus) Div(divisor int64) (Kurus, error) {
	if divisor == 0 {
		return 0, ErrDivisionByZero
	}
	return Kurus(int64(k) / divisor), nil
}

// MustDiv divides by an integer divisor and panics on division by zero.
func (k Kurus) MustDiv(divisor int64) Kurus {
	result, err := k.Div(divisor)
	if err != nil {
		panic(err)
	}
	return result
}

// DivRound divides by an integer divisor with rounding.
// Returns an error if divisor is zero.
func (k Kurus) DivRound(divisor int64) (Kurus, error) {
	if divisor == 0 {
		return 0, ErrDivisionByZero
	}
	val := int64(k)
	// Round half away from zero
	if (val >= 0 && divisor > 0) || (val < 0 && divisor < 0) {
		return Kurus((val + divisor/2) / divisor), nil
	}
	return Kurus((val - divisor/2) / divisor), nil
}

// Percentage calculates a percentage of the amount.
// For example, k.Percentage(20) returns 20% of k.
// The percentage is specified as a whole number (20 for 20%, not 0.20).
// Returns an error if percentage is negative.
func (k Kurus) Percentage(percent int64) (Kurus, error) {
	if percent < 0 {
		return 0, ErrInvalidPercentage
	}
	// Calculate with rounding: (k * percent + 50) / 100
	val := int64(k) * percent
	if val >= 0 {
		return Kurus((val + 50) / 100), nil
	}
	return Kurus((val - 50) / 100), nil
}

// MustPercentage calculates a percentage and panics on error.
func (k Kurus) MustPercentage(percent int64) Kurus {
	result, err := k.Percentage(percent)
	if err != nil {
		panic(err)
	}
	return result
}

// PercentageDecimal calculates a percentage using decimal representation.
// For example, k.PercentageDecimal(2050) returns 20.50% of k.
// The percentage is specified as hundredths (2050 for 20.50%).
func (k Kurus) PercentageDecimal(percentHundredths int64) (Kurus, error) {
	if percentHundredths < 0 {
		return 0, ErrInvalidPercentage
	}
	// Calculate with rounding: (k * percentHundredths + 5000) / 10000
	val := int64(k) * percentHundredths
	if val >= 0 {
		return Kurus((val + 5000) / 10000), nil
	}
	return Kurus((val - 5000) / 10000), nil
}

// AddPercentage adds a percentage to the amount.
// For example, k.AddPercentage(20) returns k + 20% of k.
func (k Kurus) AddPercentage(percent int64) (Kurus, error) {
	addition, err := k.Percentage(percent)
	if err != nil {
		return 0, err
	}
	return k.Add(addition)
}

// SubPercentage subtracts a percentage from the amount.
// For example, k.SubPercentage(20) returns k - 20% of k.
func (k Kurus) SubPercentage(percent int64) (Kurus, error) {
	subtraction, err := k.Percentage(percent)
	if err != nil {
		return 0, err
	}
	return k.Sub(subtraction)
}

// Split divides the amount into n equal parts.
// Returns the parts and any remainder that couldn't be evenly divided.
// The remainder is always less than n kuruş.
func (k Kurus) Split(n int) (part Kurus, remainder Kurus, err error) {
	if n <= 0 {
		return 0, 0, ErrDivisionByZero
	}
	part = Kurus(int64(k) / int64(n))
	remainder = Kurus(int64(k) % int64(n))
	return part, remainder, nil
}

// Allocate distributes the amount according to the given ratios.
// For example, k.Allocate([]int64{1, 2, 3}) distributes k in ratio 1:2:3.
// The sum of allocated amounts always equals the original amount.
func (k Kurus) Allocate(ratios []int64) ([]Kurus, error) {
	if len(ratios) == 0 {
		return nil, errors.New("money: empty ratios")
	}

	var total int64
	for _, r := range ratios {
		if r < 0 {
			return nil, errors.New("money: negative ratio")
		}
		total += r
	}

	if total == 0 {
		return nil, errors.New("money: zero total ratio")
	}

	results := make([]Kurus, len(ratios))
	var allocated Kurus

	for i, ratio := range ratios {
		// Calculate share with rounding
		share := Kurus((int64(k) * ratio) / total)
		results[i] = share
		allocated += share
	}

	// Distribute remainder to first non-zero ratio
	remainder := k - allocated
	for i := range results {
		if ratios[i] > 0 && remainder != 0 {
			if remainder > 0 {
				results[i]++
				remainder--
			} else {
				results[i]--
				remainder++
			}
		}
		if remainder == 0 {
			break
		}
	}

	return results, nil
}

// Compare compares two amounts.
// Returns -1 if k < other, 0 if k == other, 1 if k > other.
func (k Kurus) Compare(other Kurus) int {
	switch {
	case k < other:
		return -1
	case k > other:
		return 1
	default:
		return 0
	}
}

// Equal returns true if the amounts are equal.
func (k Kurus) Equal(other Kurus) bool {
	return k == other
}

// LessThan returns true if k < other.
func (k Kurus) LessThan(other Kurus) bool {
	return k < other
}

// LessThanOrEqual returns true if k <= other.
func (k Kurus) LessThanOrEqual(other Kurus) bool {
	return k <= other
}

// GreaterThan returns true if k > other.
func (k Kurus) GreaterThan(other Kurus) bool {
	return k > other
}

// GreaterThanOrEqual returns true if k >= other.
func (k Kurus) GreaterThanOrEqual(other Kurus) bool {
	return k >= other
}

// Min returns the smaller of two amounts.
func Min(a, b Kurus) Kurus {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of two amounts.
func Max(a, b Kurus) Kurus {
	if a > b {
		return a
	}
	return b
}

// Sum calculates the sum of multiple amounts.
// Returns an error if the sum would overflow.
func Sum(amounts ...Kurus) (Kurus, error) {
	var total Kurus
	for _, amount := range amounts {
		var err error
		total, err = total.Add(amount)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

// MustSum calculates the sum and panics on overflow.
func MustSum(amounts ...Kurus) Kurus {
	result, err := Sum(amounts...)
	if err != nil {
		panic(err)
	}
	return result
}

// JSON marshaling

// MarshalJSON implements json.Marshaler.
// Marshals as an integer (kuruş value).
func (k Kurus) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(k))
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts integer (kuruş) or string (TRY format).
func (k *Kurus) UnmarshalJSON(data []byte) error {
	// Try integer first
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*k = Kurus(i)
		return nil
	}

	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parsed, err := ParseTRY(s)
		if err != nil {
			return err
		}
		*k = parsed
		return nil
	}

	return ErrInvalidFormat
}

// SQL scanning

// Scan implements sql.Scanner.
func (k *Kurus) Scan(src interface{}) error {
	switch v := src.(type) {
	case int64:
		*k = Kurus(v)
		return nil
	case int32:
		*k = Kurus(v)
		return nil
	case int:
		*k = Kurus(v)
		return nil
	case float64:
		*k = FromFloat(v)
		return nil
	case []byte:
		parsed, err := ParseTRY(string(v))
		if err != nil {
			return err
		}
		*k = parsed
		return nil
	case string:
		parsed, err := ParseTRY(v)
		if err != nil {
			return err
		}
		*k = parsed
		return nil
	case nil:
		*k = 0
		return nil
	default:
		return fmt.Errorf("money: cannot scan type %T into Kurus", src)
	}
}

// Value implements driver.Valuer.
func (k Kurus) Value() (driver.Value, error) {
	return int64(k), nil
}

// PriceBreakdown represents a detailed price breakdown.
type PriceBreakdown struct {
	BasePrice     Kurus `json:"base_price_kurus"`
	FuelSurcharge Kurus `json:"fuel_surcharge_kurus"`
	CODFee        Kurus `json:"cod_fee_kurus"`
	InsuranceFee  Kurus `json:"insurance_fee_kurus"`
	OtherFees     Kurus `json:"other_fees_kurus"`
	Subtotal      Kurus `json:"subtotal_kurus"`
	VATAmount     Kurus `json:"vat_kurus"`
	Total         Kurus `json:"total_kurus"`
}

// Calculate computes the subtotal and total from the breakdown components.
// VATPercent should be specified as a whole number (e.g., 20 for 20%).
func (p *PriceBreakdown) Calculate(vatPercent int64) error {
	subtotal, err := Sum(p.BasePrice, p.FuelSurcharge, p.CODFee, p.InsuranceFee, p.OtherFees)
	if err != nil {
		return err
	}
	p.Subtotal = subtotal

	vat, err := subtotal.Percentage(vatPercent)
	if err != nil {
		return err
	}
	p.VATAmount = vat

	total, err := subtotal.Add(vat)
	if err != nil {
		return err
	}
	p.Total = total

	return nil
}

// Validate checks that the breakdown is internally consistent.
func (p *PriceBreakdown) Validate() error {
	expectedSubtotal, err := Sum(p.BasePrice, p.FuelSurcharge, p.CODFee, p.InsuranceFee, p.OtherFees)
	if err != nil {
		return err
	}
	if p.Subtotal != expectedSubtotal {
		return fmt.Errorf("money: subtotal mismatch: expected %s, got %s", expectedSubtotal, p.Subtotal)
	}

	expectedTotal, err := p.Subtotal.Add(p.VATAmount)
	if err != nil {
		return err
	}
	if p.Total != expectedTotal {
		return fmt.Errorf("money: total mismatch: expected %s, got %s", expectedTotal, p.Total)
	}

	return nil
}
