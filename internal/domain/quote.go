package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// QuoteStatus represents the status of a quote.
type QuoteStatus string

const (
	// QuoteStatusPending indicates the quote is being processed.
	QuoteStatusPending QuoteStatus = "pending"
	// QuoteStatusCompleted indicates the quote has been completed with offers.
	QuoteStatusCompleted QuoteStatus = "completed"
	// QuoteStatusFailed indicates the quote failed to generate offers.
	QuoteStatusFailed QuoteStatus = "failed"
	// QuoteStatusExpired indicates the quote has expired.
	QuoteStatusExpired QuoteStatus = "expired"
)

// IsValid checks if the quote status is valid.
func (s QuoteStatus) IsValid() bool {
	switch s {
	case QuoteStatusPending, QuoteStatusCompleted, QuoteStatusFailed, QuoteStatusExpired:
		return true
	default:
		return false
	}
}

// String returns the string representation of the quote status.
func (s QuoteStatus) String() string {
	return string(s)
}

// Quote represents a rate request record.
type Quote struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// OrganizationID is the parent organization ID.
	OrganizationID uuid.UUID `json:"organization_id"`
	// IdempotencyKey is the client-provided idempotency key.
	IdempotencyKey string `json:"idempotency_key"`
	// OriginAddress is the origin address snapshot.
	OriginAddress AddressSnapshot `json:"origin_address"`
	// DestinationAddress is the destination address snapshot.
	DestinationAddress AddressSnapshot `json:"destination_address"`
	// Parcel contains the parcel details.
	Parcel Parcel `json:"parcel"`
	// CODAmountKurus is the COD amount in kuruş (if applicable).
	CODAmountKurus *int64 `json:"cod_amount_kurus,omitempty"`
	// DeclaredValueKurus is the declared value for insurance in kuruş.
	DeclaredValueKurus *int64 `json:"declared_value_kurus,omitempty"`
	// UseBYOA indicates whether to use BYOA accounts.
	UseBYOA bool `json:"use_byoa"`
	// ProviderAccountIDs is the list of specific BYOA account IDs to use.
	ProviderAccountIDs []uuid.UUID `json:"provider_account_ids,omitempty"`
	// Status is the quote status.
	Status QuoteStatus `json:"status"`
	// ExpiresAt is the quote expiration timestamp.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is the timestamp when the quote was created.
	CreatedAt time.Time `json:"created_at"`
	// Offers contains the generated offers (populated after completion).
	Offers []Offer `json:"offers,omitempty"`
}

// NewQuote creates a new Quote with the given parameters.
func NewQuote(
	organizationID uuid.UUID,
	idempotencyKey string,
	origin, destination AddressSnapshot,
	parcel Parcel,
	codAmountKurus, declaredValueKurus *int64,
	useBYOA bool,
	providerAccountIDs []uuid.UUID,
	expiresIn time.Duration,
) *Quote {
	now := time.Now().UTC()
	return &Quote{
		ID:                 uuid.New(),
		OrganizationID:     organizationID,
		IdempotencyKey:     idempotencyKey,
		OriginAddress:      origin,
		DestinationAddress: destination,
		Parcel:             parcel,
		CODAmountKurus:     codAmountKurus,
		DeclaredValueKurus: declaredValueKurus,
		UseBYOA:            useBYOA,
		ProviderAccountIDs: providerAccountIDs,
		Status:             QuoteStatusPending,
		ExpiresAt:          now.Add(expiresIn),
		CreatedAt:          now,
	}
}

// IsExpired returns true if the quote has expired.
func (q *Quote) IsExpired() bool {
	return time.Now().UTC().After(q.ExpiresAt)
}

// IsPending returns true if the quote is pending.
func (q *Quote) IsPending() bool {
	return q.Status == QuoteStatusPending
}

// IsCompleted returns true if the quote is completed.
func (q *Quote) IsCompleted() bool {
	return q.Status == QuoteStatusCompleted
}

// IsFailed returns true if the quote failed.
func (q *Quote) IsFailed() bool {
	return q.Status == QuoteStatusFailed
}

// CanBeUsed returns true if the quote can be used to create a shipment.
func (q *Quote) CanBeUsed() bool {
	return q.IsCompleted() && !q.IsExpired()
}

// HasCOD returns true if the quote includes COD.
func (q *Quote) HasCOD() bool {
	return q.CODAmountKurus != nil && *q.CODAmountKurus > 0
}

// HasInsurance returns true if the quote includes insurance.
func (q *Quote) HasInsurance() bool {
	return q.DeclaredValueKurus != nil && *q.DeclaredValueKurus > 0
}

// MarkCompleted marks the quote as completed with offers.
func (q *Quote) MarkCompleted(offers []Offer) {
	q.Status = QuoteStatusCompleted
	q.Offers = offers
}

// MarkFailed marks the quote as failed.
func (q *Quote) MarkFailed() {
	q.Status = QuoteStatusFailed
}

// MarkExpired marks the quote as expired.
func (q *Quote) MarkExpired() {
	q.Status = QuoteStatusExpired
}

// Refresh refreshes the quote expiration.
func (q *Quote) Refresh(expiresIn time.Duration) {
	q.ExpiresAt = time.Now().UTC().Add(expiresIn)
}

// GetOffer returns the offer with the given ID.
func (q *Quote) GetOffer(offerID uuid.UUID) *Offer {
	for i := range q.Offers {
		if q.Offers[i].ID == offerID {
			return &q.Offers[i]
		}
	}
	return nil
}

// GetCheapestOffer returns the cheapest offer.
func (q *Quote) GetCheapestOffer() *Offer {
	if len(q.Offers) == 0 {
		return nil
	}
	cheapest := &q.Offers[0]
	for i := 1; i < len(q.Offers); i++ {
		if q.Offers[i].TotalPriceKurus < cheapest.TotalPriceKurus {
			cheapest = &q.Offers[i]
		}
	}
	return cheapest
}

// GetFastestOffer returns the fastest offer (minimum estimated days).
func (q *Quote) GetFastestOffer() *Offer {
	if len(q.Offers) == 0 {
		return nil
	}
	fastest := &q.Offers[0]
	for i := 1; i < len(q.Offers); i++ {
		if q.Offers[i].EstimatedDaysMin < fastest.EstimatedDaysMin {
			fastest = &q.Offers[i]
		}
	}
	return fastest
}

// MarshalOriginAddressJSON marshals the origin address to JSON.
func (q *Quote) MarshalOriginAddressJSON() (json.RawMessage, error) {
	return json.Marshal(q.OriginAddress)
}

// UnmarshalOriginAddressJSON unmarshals the origin address from JSON.
func (q *Quote) UnmarshalOriginAddressJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &q.OriginAddress)
}

// MarshalDestinationAddressJSON marshals the destination address to JSON.
func (q *Quote) MarshalDestinationAddressJSON() (json.RawMessage, error) {
	return json.Marshal(q.DestinationAddress)
}

// UnmarshalDestinationAddressJSON unmarshals the destination address from JSON.
func (q *Quote) UnmarshalDestinationAddressJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &q.DestinationAddress)
}

// MarshalParcelJSON marshals the parcel to JSON.
func (q *Quote) MarshalParcelJSON() (json.RawMessage, error) {
	return json.Marshal(q.Parcel)
}

// UnmarshalParcelJSON unmarshals the parcel from JSON.
func (q *Quote) UnmarshalParcelJSON(data json.RawMessage) error {
	return json.Unmarshal(data, &q.Parcel)
}

// Offer represents a price quote from a provider.
type Offer struct {
	// ID is the unique identifier.
	ID uuid.UUID `json:"id"`
	// QuoteID is the parent quote ID.
	QuoteID uuid.UUID `json:"quote_id"`
	// ProviderCode is the carrier provider code.
	ProviderCode ProviderCode `json:"provider_code"`
	// ServiceCode is the service code.
	ServiceCode string `json:"service_code"`
	// ProviderAccountID is the BYOA account ID (if applicable).
	ProviderAccountID *uuid.UUID `json:"provider_account_id,omitempty"`
	// IsMarketplace indicates if this is marketplace pricing.
	IsMarketplace bool `json:"is_marketplace"`
	// BasePriceKurus is the base shipping price in kuruş.
	BasePriceKurus int64 `json:"base_price_kurus"`
	// FuelSurchargeKurus is the fuel surcharge in kuruş.
	FuelSurchargeKurus int64 `json:"fuel_surcharge_kurus"`
	// CODFeeKurus is the COD handling fee in kuruş.
	CODFeeKurus int64 `json:"cod_fee_kurus"`
	// InsuranceFeeKurus is the insurance fee in kuruş.
	InsuranceFeeKurus int64 `json:"insurance_fee_kurus"`
	// OtherFeesKurus is other fees in kuruş.
	OtherFeesKurus int64 `json:"other_fees_kurus"`
	// SubtotalKurus is the subtotal before VAT in kuruş.
	SubtotalKurus int64 `json:"subtotal_kurus"`
	// VATKurus is the VAT amount in kuruş.
	VATKurus int64 `json:"vat_kurus"`
	// TotalPriceKurus is the total price including VAT in kuruş.
	TotalPriceKurus int64 `json:"total_price_kurus"`
	// CostPriceKurus is our cost (marketplace only) in kuruş.
	CostPriceKurus *int64 `json:"cost_price_kurus,omitempty"`
	// MarginKurus is the margin (marketplace only) in kuruş.
	MarginKurus *int64 `json:"margin_kurus,omitempty"`
	// EstimatedDaysMin is the minimum delivery days.
	EstimatedDaysMin int `json:"estimated_days_min"`
	// EstimatedDaysMax is the maximum delivery days.
	EstimatedDaysMax int `json:"estimated_days_max"`
	// ProviderQuoteID is the provider's quote reference ID.
	ProviderQuoteID *string `json:"provider_quote_id,omitempty"`
	// ExpiresAt is the offer expiration timestamp.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is the timestamp when the offer was created.
	CreatedAt time.Time `json:"created_at"`
}

// NewOffer creates a new Offer with the given parameters.
func NewOffer(
	quoteID uuid.UUID,
	providerCode ProviderCode,
	serviceCode string,
	providerAccountID *uuid.UUID,
	isMarketplace bool,
	breakdown PriceBreakdown,
	costPriceKurus *int64,
	estimatedDaysMin, estimatedDaysMax int,
	providerQuoteID *string,
	expiresAt time.Time,
) *Offer {
	now := time.Now().UTC()

	var marginKurus *int64
	if isMarketplace && costPriceKurus != nil {
		margin := breakdown.TotalPriceKurus - *costPriceKurus
		marginKurus = &margin
	}

	return &Offer{
		ID:                 uuid.New(),
		QuoteID:            quoteID,
		ProviderCode:       providerCode,
		ServiceCode:        serviceCode,
		ProviderAccountID:  providerAccountID,
		IsMarketplace:      isMarketplace,
		BasePriceKurus:     breakdown.BasePriceKurus,
		FuelSurchargeKurus: breakdown.FuelSurchargeKurus,
		CODFeeKurus:        breakdown.CODFeeKurus,
		InsuranceFeeKurus:  breakdown.InsuranceFeeKurus,
		OtherFeesKurus:     breakdown.OtherFeesKurus,
		SubtotalKurus:      breakdown.SubtotalKurus,
		VATKurus:           breakdown.VATKurus,
		TotalPriceKurus:    breakdown.TotalPriceKurus,
		CostPriceKurus:     costPriceKurus,
		MarginKurus:        marginKurus,
		EstimatedDaysMin:   estimatedDaysMin,
		EstimatedDaysMax:   estimatedDaysMax,
		ProviderQuoteID:    providerQuoteID,
		ExpiresAt:          expiresAt,
		CreatedAt:          now,
	}
}

// IsExpired returns true if the offer has expired.
func (o *Offer) IsExpired() bool {
	return time.Now().UTC().After(o.ExpiresAt)
}

// CanBeUsed returns true if the offer can be used to create a shipment.
func (o *Offer) CanBeUsed() bool {
	return !o.IsExpired()
}

// GetPriceBreakdown returns the price breakdown for the offer.
func (o *Offer) GetPriceBreakdown() PriceBreakdown {
	return PriceBreakdown{
		BasePriceKurus:     o.BasePriceKurus,
		FuelSurchargeKurus: o.FuelSurchargeKurus,
		CODFeeKurus:        o.CODFeeKurus,
		InsuranceFeeKurus:  o.InsuranceFeeKurus,
		OtherFeesKurus:     o.OtherFeesKurus,
		SubtotalKurus:      o.SubtotalKurus,
		VATKurus:           o.VATKurus,
		TotalPriceKurus:    o.TotalPriceKurus,
	}
}

// EstimatedDeliveryRange returns the estimated delivery date range.
func (o *Offer) EstimatedDeliveryRange(from time.Time) (min, max time.Time) {
	min = from.AddDate(0, 0, o.EstimatedDaysMin)
	max = from.AddDate(0, 0, o.EstimatedDaysMax)
	return
}

// Parcel represents parcel details for a shipment.
type Parcel struct {
	// WeightGrams is the weight in grams.
	WeightGrams int `json:"weight_grams"`
	// LengthCm is the length in centimeters.
	LengthCm int `json:"length_cm"`
	// WidthCm is the width in centimeters.
	WidthCm int `json:"width_cm"`
	// HeightCm is the height in centimeters.
	HeightCm int `json:"height_cm"`
	// Description is the parcel content description.
	Description *string `json:"description,omitempty"`
	// Reference is the customer reference number.
	Reference *string `json:"reference,omitempty"`
}

// NewParcel creates a new Parcel with the given dimensions.
func NewParcel(weightGrams, lengthCm, widthCm, heightCm int) Parcel {
	return Parcel{
		WeightGrams: weightGrams,
		LengthCm:    lengthCm,
		WidthCm:     widthCm,
		HeightCm:    heightCm,
	}
}

// VolumeCC returns the volume in cubic centimeters.
func (p *Parcel) VolumeCC() int {
	return p.LengthCm * p.WidthCm * p.HeightCm
}

// Desi returns the volumetric weight (desi) for Turkish carriers.
// Desi = (L × W × H) / 3000
func (p *Parcel) Desi() float64 {
	return float64(p.VolumeCC()) / 3000.0
}

// ChargeableWeight returns the chargeable weight in grams.
// This is the higher of actual weight or volumetric weight.
func (p *Parcel) ChargeableWeight() int {
	desiGrams := int(p.Desi() * 1000) // Convert desi to grams (1 desi = 1 kg)
	if desiGrams > p.WeightGrams {
		return desiGrams
	}
	return p.WeightGrams
}

// IsValid checks if the parcel dimensions are valid.
func (p *Parcel) IsValid() bool {
	return p.WeightGrams > 0 && p.LengthCm > 0 && p.WidthCm > 0 && p.HeightCm > 0
}

// FitsInService checks if the parcel fits within the service limits.
func (p *Parcel) FitsInService(maxWeightGrams, maxLengthCm, maxWidthCm, maxHeightCm int) bool {
	return p.WeightGrams <= maxWeightGrams &&
		p.LengthCm <= maxLengthCm &&
		p.WidthCm <= maxWidthCm &&
		p.HeightCm <= maxHeightCm
}
