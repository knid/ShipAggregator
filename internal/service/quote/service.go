// Package quote provides quote and offer management services.
package quote

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/provider"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	providerservice "github.com/e-port/shipping-aggregator/internal/service/provider"
)

// QuoteExpiry is the default expiry time for quotes (30 minutes).
const QuoteExpiry = 30 * time.Minute

// VATPercent is the default VAT percentage for Turkey.
const VATPercent = 20.0

// Service provides quote and offer management operations.
type Service struct {
	queries        *db.Queries
	adapterPool    *provider.AdapterPool
	accountService *providerservice.AccountService
	logger         *observability.Logger
}

// NewService creates a new quote service.
func NewService(queries *db.Queries, logger *observability.Logger) *Service {
	return &Service{
		queries: queries,
		logger:  logger,
	}
}

// NewServiceWithBYOA creates a new quote service with BYOA support.
func NewServiceWithBYOA(
	queries *db.Queries,
	adapterPool *provider.AdapterPool,
	accountService *providerservice.AccountService,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:        queries,
		adapterPool:    adapterPool,
		accountService: accountService,
		logger:         logger,
	}
}

// CreateQuoteRequest represents a request to create a quote.
type CreateQuoteRequest struct {
	OrganizationID     uuid.UUID
	IdempotencyKey     string
	OriginAddress      *domain.Address
	DestinationAddress *domain.Address
	Parcel             *domain.Parcel
	CODAmountKurus     *int32
	DeclaredValueKurus *int32
	UseBYOA            bool
	ProviderAccountIDs []uuid.UUID
}

// QuoteResult represents the result of a quote creation.
type QuoteResult struct {
	Quote  *db.Quote
	Offers []*db.Offer
}

// CreateQuote creates a new quote and generates offers from providers.
func (s *Service) CreateQuote(ctx context.Context, req *CreateQuoteRequest) (*QuoteResult, error) {
	// Check for existing quote with same idempotency key
	existingQuote, err := s.queries.GetQuoteByIdempotencyKey(ctx, db.GetQuoteByIdempotencyKeyParams{
		OrganizationID: req.OrganizationID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err == nil {
		// Return existing quote with offers
		offers, err := s.queries.ListOffersByQuote(ctx, existingQuote.ID)
		if err != nil {
			return nil, err
		}
		return &QuoteResult{
			Quote:  existingQuote,
			Offers: offers,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Serialize addresses and parcel
	originJSON, err := json.Marshal(req.OriginAddress)
	if err != nil {
		return nil, err
	}
	destJSON, err := json.Marshal(req.DestinationAddress)
	if err != nil {
		return nil, err
	}
	parcelJSON, err := json.Marshal(req.Parcel)
	if err != nil {
		return nil, err
	}

	// Serialize provider account IDs
	var providerAccountIDs []byte
	if len(req.ProviderAccountIDs) > 0 {
		providerAccountIDs, err = json.Marshal(req.ProviderAccountIDs)
		if err != nil {
			return nil, err
		}
	}

	// Create quote
	expiresAt := time.Now().Add(QuoteExpiry)
	quote, err := s.queries.CreateQuote(ctx, db.CreateQuoteParams{
		OrganizationID:     req.OrganizationID,
		IdempotencyKey:     req.IdempotencyKey,
		OriginAddress:      originJSON,
		DestinationAddress: destJSON,
		Parcel:             parcelJSON,
		CodAmountKurus:     req.CODAmountKurus,
		DeclaredValueKurus: req.DeclaredValueKurus,
		UseByoa:            req.UseBYOA,
		ProviderAccountIds: providerAccountIDs,
		Status:             "pending",
		ExpiresAt:          pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	// Generate offers
	offers, err := s.generateOffers(ctx, quote, req)
	if err != nil {
		s.logger.Error("Failed to generate offers", "error", err, "quote_id", quote.ID)
		// Update quote status to failed
		_, _ = s.queries.UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
			ID:     quote.ID,
			Status: "failed",
		})
		return nil, err
	}

	// Update quote status to completed
	quote, err = s.queries.UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
		ID:     quote.ID,
		Status: "completed",
	})
	if err != nil {
		return nil, err
	}

	return &QuoteResult{
		Quote:  quote,
		Offers: offers,
	}, nil
}

// generateOffers generates offers from all active providers.
func (s *Service) generateOffers(ctx context.Context, quote *db.Quote, req *CreateQuoteRequest) ([]*db.Offer, error) {
	var offers []*db.Offer

	// If BYOA mode is enabled and we have provider accounts, generate BYOA offers
	if req.UseBYOA && len(req.ProviderAccountIDs) > 0 && s.adapterPool != nil && s.accountService != nil {
		byoaOffers, err := s.generateBYOAOffers(ctx, quote, req)
		if err != nil {
			s.logger.Error("Failed to generate BYOA offers", "error", err, "quote_id", quote.ID)
			// Continue with marketplace offers if BYOA fails
		} else {
			offers = append(offers, byoaOffers...)
		}
	}

	// Generate marketplace offers (always available)
	marketplaceOffers, err := s.generateMarketplaceOffers(ctx, quote, req)
	if err != nil {
		s.logger.Error("Failed to generate marketplace offers", "error", err, "quote_id", quote.ID)
	} else {
		offers = append(offers, marketplaceOffers...)
	}

	return offers, nil
}

// generateBYOAOffers generates offers using customer's own provider accounts.
func (s *Service) generateBYOAOffers(ctx context.Context, quote *db.Quote, req *CreateQuoteRequest) ([]*db.Offer, error) {
	var offers []*db.Offer

	// Get verified provider accounts
	accounts, err := s.accountService.GetAccountsByIDs(ctx, req.ProviderAccountIDs)
	if err != nil {
		return nil, err
	}

	// Build address snapshots for provider API calls
	// Note: For BYOA, we need to convert Address to AddressSnapshot
	// The names (City, District, Neighborhood) should be resolved from reference data
	// For now, we use the codes as placeholders - in production, these would be resolved
	originSnapshot := domain.AddressSnapshot{
		ContactName:      req.OriginAddress.ContactName,
		CompanyName:      req.OriginAddress.CompanyName,
		Phone:            req.OriginAddress.Phone,
		Email:            req.OriginAddress.Email,
		StreetLine1:      req.OriginAddress.StreetLine1,
		StreetLine2:      req.OriginAddress.StreetLine2,
		Neighborhood:     req.OriginAddress.NeighborhoodCode, // Would be resolved from ref data
		NeighborhoodCode: req.OriginAddress.NeighborhoodCode,
		District:         req.OriginAddress.DistrictCode, // Would be resolved from ref data
		DistrictCode:     req.OriginAddress.DistrictCode,
		City:             req.OriginAddress.CityCode, // Would be resolved from ref data
		CityCode:         req.OriginAddress.CityCode,
		PostalCode:       req.OriginAddress.PostalCode,
	}

	destSnapshot := domain.AddressSnapshot{
		ContactName:      req.DestinationAddress.ContactName,
		CompanyName:      req.DestinationAddress.CompanyName,
		Phone:            req.DestinationAddress.Phone,
		Email:            req.DestinationAddress.Email,
		StreetLine1:      req.DestinationAddress.StreetLine1,
		StreetLine2:      req.DestinationAddress.StreetLine2,
		Neighborhood:     req.DestinationAddress.NeighborhoodCode, // Would be resolved from ref data
		NeighborhoodCode: req.DestinationAddress.NeighborhoodCode,
		District:         req.DestinationAddress.DistrictCode, // Would be resolved from ref data
		DistrictCode:     req.DestinationAddress.DistrictCode,
		City:             req.DestinationAddress.CityCode, // Would be resolved from ref data
		CityCode:         req.DestinationAddress.CityCode,
		PostalCode:       req.DestinationAddress.PostalCode,
	}

	// Prepare COD and declared value
	var codAmountKurus *int64
	if req.CODAmountKurus != nil {
		cod := int64(*req.CODAmountKurus)
		codAmountKurus = &cod
	}

	var declaredValueKurus *int64
	if req.DeclaredValueKurus != nil {
		dv := int64(*req.DeclaredValueKurus)
		declaredValueKurus = &dv
	}

	// Generate offers for each provider account
	for _, account := range accounts {
		// Get provider adapter
		providerCode := domain.ProviderCode(account.ProviderCode)
		adapter, err := s.adapterPool.Get(providerCode)
		if err != nil {
			s.logger.Error("Failed to get provider adapter for BYOA",
				"error", err,
				"provider_code", account.ProviderCode,
				"account_id", account.ID,
			)
			continue
		}

		// Decrypt credentials
		credentials, err := s.accountService.DecryptCredentials(ctx, account)
		if err != nil {
			s.logger.Error("Failed to decrypt credentials for BYOA",
				"error", err,
				"account_id", account.ID,
			)
			continue
		}

		// Get quote from provider
		quoteResp, err := adapter.GetQuote(ctx, provider.QuoteRequest{
			Origin:             originSnapshot,
			Destination:        destSnapshot,
			Parcel:             *req.Parcel,
			CODAmountKurus:     codAmountKurus,
			DeclaredValueKurus: declaredValueKurus,
			Credentials:        credentials,
		})
		if err != nil {
			s.logger.Error("Failed to get BYOA quote from provider",
				"error", err,
				"provider_code", account.ProviderCode,
				"account_id", account.ID,
			)
			continue
		}

		// Create offers from provider response
		for _, providerOffer := range quoteResp.Offers {
			expiresAt := providerOffer.ExpiresAt
			if expiresAt.IsZero() {
				expiresAt = time.Now().Add(QuoteExpiry)
			}

			// For BYOA, no margin - customer pays provider price directly
			offerParams := db.CreateOfferParams{
				QuoteID:            quote.ID,
				ProviderCode:       account.ProviderCode,
				ServiceCode:        providerOffer.ServiceCode,
				ProviderAccountID:  pgtype.UUID{Bytes: account.ID, Valid: true},
				IsMarketplace:      false, // BYOA offer
				BasePriceKurus:     int32(providerOffer.BasePriceKurus),
				FuelSurchargeKurus: int32(providerOffer.FuelSurchargeKurus),
				CodFeeKurus:        int32(providerOffer.CODFeeKurus),
				InsuranceFeeKurus:  int32(providerOffer.InsuranceFeeKurus),
				OtherFeesKurus:     0,
				SubtotalKurus:      int32(providerOffer.BasePriceKurus + providerOffer.FuelSurchargeKurus + providerOffer.CODFeeKurus + providerOffer.InsuranceFeeKurus),
				VatKurus:           int32(providerOffer.VATKurus),
				TotalPriceKurus:    int32(providerOffer.TotalPriceKurus),
				CostPriceKurus:     nil, // No cost tracking for BYOA
				MarginKurus:        nil, // No margin for BYOA
				EstimatedDaysMin:   int32(providerOffer.EstimatedDaysMin),
				EstimatedDaysMax:   int32(providerOffer.EstimatedDaysMax),
				ProviderQuoteID:    &providerOffer.ProviderQuoteID,
				ExpiresAt:          pgtype.Timestamptz{Time: expiresAt, Valid: true},
			}

			createdOffer, err := s.queries.CreateOffer(ctx, offerParams)
			if err != nil {
				s.logger.Error("Failed to create BYOA offer",
					"error", err,
					"provider_code", account.ProviderCode,
					"service_code", providerOffer.ServiceCode,
				)
				continue
			}

			offers = append(offers, createdOffer)
		}

		// Record account usage
		_ = s.accountService.RecordUsage(ctx, account.ID)
	}

	return offers, nil
}

// generateMarketplaceOffers generates offers using marketplace pricing.
func (s *Service) generateMarketplaceOffers(ctx context.Context, quote *db.Quote, req *CreateQuoteRequest) ([]*db.Offer, error) {
	var offers []*db.Offer

	// Get all active providers
	providers, err := s.queries.ListActiveProviders(ctx)
	if err != nil {
		return nil, err
	}

	// Get marketplace rate plans
	ratePlans, err := s.queries.ListCurrentMarketplaceRatePlans(ctx)
	if err != nil {
		return nil, err
	}

	// Build rate plan lookup
	ratePlanMap := make(map[string]*db.MarketplaceRatePlan)
	for _, rp := range ratePlans {
		key := rp.ProviderCode + ":" + rp.ServiceCode
		ratePlanMap[key] = rp
	}

	// Generate offers for each provider
	for _, dbProvider := range providers {
		// Get provider services
		services, err := s.queries.ListActiveProviderServices(ctx, dbProvider.Code)
		if err != nil {
			s.logger.Error("Failed to get provider services", "error", err, "provider", dbProvider.Code)
			continue
		}

		// Generate offer for each service
		for _, service := range services {
			// Check if service supports COD if COD is requested
			if req.CODAmountKurus != nil && *req.CODAmountKurus > 0 && !service.SupportsCod {
				continue
			}

			// Get rate plan
			key := dbProvider.Code + ":" + service.ServiceCode
			ratePlan, ok := ratePlanMap[key]
			if !ok {
				continue
			}

			// Calculate pricing
			offer, err := s.calculateOffer(ctx, quote.ID, dbProvider, service, ratePlan, req)
			if err != nil {
				s.logger.Error("Failed to calculate offer", "error", err, "provider", dbProvider.Code, "service", service.ServiceCode)
				continue
			}

			// Create offer
			createdOffer, err := s.queries.CreateOffer(ctx, *offer)
			if err != nil {
				s.logger.Error("Failed to create offer", "error", err, "provider", dbProvider.Code, "service", service.ServiceCode)
				continue
			}

			offers = append(offers, createdOffer)
		}
	}

	return offers, nil
}

// calculateOffer calculates pricing for a specific provider service.
func (s *Service) calculateOffer(
	ctx context.Context,
	quoteID uuid.UUID,
	provider *db.Provider,
	service *db.ProviderService,
	ratePlan *db.MarketplaceRatePlan,
	req *CreateQuoteRequest,
) (*db.CreateOfferParams, error) {
	// Calculate weight-based pricing
	weightKg := float64(req.Parcel.WeightGrams) / 1000.0

	// Calculate volumetric weight (desi)
	desi := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0

	// Use the greater of actual weight and volumetric weight
	chargeableWeight := weightKg
	if desi > weightKg {
		chargeableWeight = desi
	}

	// Base price
	basePriceKurus := ratePlan.BasePriceKurus

	// Weight surcharge
	weightSurchargeKurus := int32(chargeableWeight * float64(ratePlan.PricePerKgKurus))

	// Desi surcharge
	desiSurchargeKurus := int32(desi * float64(ratePlan.PricePerDesiKurus))

	// Use the greater of weight and desi surcharge
	surchargeKurus := weightSurchargeKurus
	if desiSurchargeKurus > weightSurchargeKurus {
		surchargeKurus = desiSurchargeKurus
	}

	// Fuel surcharge
	fuelSurchargePercent := numericToFloat64(ratePlan.FuelSurchargePercent)
	fuelSurchargeKurus := int32(float64(basePriceKurus+surchargeKurus) * fuelSurchargePercent / 100.0)

	// COD fee
	var codFeeKurus int32
	if req.CODAmountKurus != nil && *req.CODAmountKurus > 0 {
		codFeePercent := numericToFloat64(ratePlan.CodFeePercent)
		codFeeKurus = int32(float64(*req.CODAmountKurus) * codFeePercent / 100.0)
		if codFeeKurus < ratePlan.CodFeeMinKurus {
			codFeeKurus = ratePlan.CodFeeMinKurus
		}
	}

	// Insurance fee (simplified - 1% of declared value)
	var insuranceFeeKurus int32
	if req.DeclaredValueKurus != nil && *req.DeclaredValueKurus > 0 {
		insuranceFeeKurus = int32(float64(*req.DeclaredValueKurus) * 0.01)
	}

	// Subtotal
	subtotalKurus := basePriceKurus + surchargeKurus + fuelSurchargeKurus + codFeeKurus + insuranceFeeKurus

	// VAT
	vatPercent := numericToFloat64(ratePlan.VatPercent)
	vatKurus := int32(float64(subtotalKurus) * vatPercent / 100.0)

	// Total
	totalPriceKurus := subtotalKurus + vatKurus

	// Cost price (for marketplace margin calculation)
	// In a real implementation, this would come from provider contracts
	costPriceKurus := int32(float64(totalPriceKurus) * 0.85) // 15% margin
	marginKurus := totalPriceKurus - costPriceKurus

	expiresAt := time.Now().Add(QuoteExpiry)

	return &db.CreateOfferParams{
		QuoteID:            quoteID,
		ProviderCode:       provider.Code,
		ServiceCode:        service.ServiceCode,
		ProviderAccountID:  pgtype.UUID{}, // NULL for marketplace
		IsMarketplace:      true,
		BasePriceKurus:     basePriceKurus + surchargeKurus,
		FuelSurchargeKurus: fuelSurchargeKurus,
		CodFeeKurus:        codFeeKurus,
		InsuranceFeeKurus:  insuranceFeeKurus,
		OtherFeesKurus:     0,
		SubtotalKurus:      subtotalKurus,
		VatKurus:           vatKurus,
		TotalPriceKurus:    totalPriceKurus,
		CostPriceKurus:     &costPriceKurus,
		MarginKurus:        &marginKurus,
		EstimatedDaysMin:   service.EstimatedDaysMin,
		EstimatedDaysMax:   service.EstimatedDaysMax,
		ProviderQuoteID:    nil,
		ExpiresAt:          pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}, nil
}

// GetQuote retrieves a quote by ID.
func (s *Service) GetQuote(ctx context.Context, quoteID uuid.UUID) (*db.Quote, error) {
	return s.queries.GetQuoteByID(ctx, quoteID)
}

// GetQuoteWithOffers retrieves a quote with all its offers.
func (s *Service) GetQuoteWithOffers(ctx context.Context, quoteID uuid.UUID) (*QuoteResult, error) {
	quote, err := s.queries.GetQuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	offers, err := s.queries.ListOffersByQuote(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	return &QuoteResult{
		Quote:  quote,
		Offers: offers,
	}, nil
}

// GetActiveOffers retrieves active (non-expired) offers for a quote.
func (s *Service) GetActiveOffers(ctx context.Context, quoteID uuid.UUID) ([]*db.Offer, error) {
	return s.queries.ListActiveOffersByQuote(ctx, quoteID)
}

// GetOffer retrieves an offer by ID.
func (s *Service) GetOffer(ctx context.Context, offerID uuid.UUID) (*db.Offer, error) {
	return s.queries.GetOfferByID(ctx, offerID)
}

// GetActiveOffer retrieves an active (non-expired) offer by ID.
func (s *Service) GetActiveOffer(ctx context.Context, offerID uuid.UUID) (*db.Offer, error) {
	return s.queries.GetActiveOfferByID(ctx, offerID)
}

// RefreshQuote refreshes a quote by regenerating offers.
func (s *Service) RefreshQuote(ctx context.Context, quoteID uuid.UUID, orgID uuid.UUID) (*QuoteResult, error) {
	// Get existing quote
	quote, err := s.queries.GetQuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	// Verify organization ownership
	if quote.OrganizationID != orgID {
		return nil, errors.New("quote not found")
	}

	// Parse addresses and parcel
	var originAddress domain.Address
	if err := json.Unmarshal(quote.OriginAddress, &originAddress); err != nil {
		return nil, err
	}

	var destAddress domain.Address
	if err := json.Unmarshal(quote.DestinationAddress, &destAddress); err != nil {
		return nil, err
	}

	var parcel domain.Parcel
	if err := json.Unmarshal(quote.Parcel, &parcel); err != nil {
		return nil, err
	}

	// Parse provider account IDs
	var providerAccountIDs []uuid.UUID
	if len(quote.ProviderAccountIds) > 0 {
		if err := json.Unmarshal(quote.ProviderAccountIds, &providerAccountIDs); err != nil {
			return nil, err
		}
	}

	// Delete existing offers
	if err := s.queries.DeleteOffersByQuote(ctx, quoteID); err != nil {
		return nil, err
	}

	// Update quote status to pending
	quote, err = s.queries.UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
		ID:     quoteID,
		Status: "pending",
	})
	if err != nil {
		return nil, err
	}

	// Regenerate offers
	req := &CreateQuoteRequest{
		OrganizationID:     quote.OrganizationID,
		IdempotencyKey:     quote.IdempotencyKey,
		OriginAddress:      &originAddress,
		DestinationAddress: &destAddress,
		Parcel:             &parcel,
		CODAmountKurus:     quote.CodAmountKurus,
		DeclaredValueKurus: quote.DeclaredValueKurus,
		UseBYOA:            quote.UseByoa,
		ProviderAccountIDs: providerAccountIDs,
	}

	offers, err := s.generateOffers(ctx, quote, req)
	if err != nil {
		s.logger.Error("Failed to regenerate offers", "error", err, "quote_id", quote.ID)
		_, _ = s.queries.UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
			ID:     quote.ID,
			Status: "failed",
		})
		return nil, err
	}

	// Update quote status to completed
	quote, err = s.queries.UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
		ID:     quote.ID,
		Status: "completed",
	})
	if err != nil {
		return nil, err
	}

	return &QuoteResult{
		Quote:  quote,
		Offers: offers,
	}, nil
}

// ListQuotes lists quotes for an organization.
func (s *Service) ListQuotes(ctx context.Context, orgID uuid.UUID, status *string, offset, limit int) ([]*db.Quote, int64, error) {
	count, err := s.queries.CountQuotesByOrganization(ctx, db.CountQuotesByOrganizationParams{
		OrganizationID: orgID,
		Status:         status,
	})
	if err != nil {
		return nil, 0, err
	}

	quotes, err := s.queries.ListQuotesByOrganization(ctx, db.ListQuotesByOrganizationParams{
		OrganizationID: orgID,
		Status:         status,
		OffsetCount:    int32(offset),
		LimitCount:     int32(limit),
	})
	if err != nil {
		return nil, 0, err
	}

	return quotes, count, nil
}

// ExpireQuotes marks expired quotes as expired.
func (s *Service) ExpireQuotes(ctx context.Context) error {
	return s.queries.ExpireQuotes(ctx)
}

// CleanupExpiredQuotes deletes old expired quotes.
func (s *Service) CleanupExpiredQuotes(ctx context.Context) error {
	return s.queries.DeleteExpiredQuotes(ctx)
}

// Helper function to convert pgtype.Numeric to float64
func numericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}
