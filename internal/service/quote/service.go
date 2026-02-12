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

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	providerservice "github.com/knid/kargonet/internal/service/provider"
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
	platformCreds  map[domain.ProviderCode]*domain.ProviderCredentials
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

// NewServiceWithPlatformCreds creates a new quote service with platform-level
// provider credentials for live marketplace pricing.
func NewServiceWithPlatformCreds(
	queries *db.Queries,
	adapterPool *provider.AdapterPool,
	platformCreds map[domain.ProviderCode]*domain.ProviderCredentials,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:       queries,
		adapterPool:   adapterPool,
		platformCreds: platformCreds,
		logger:        logger,
	}
}

// NewServiceWithPlatformCredsAndBYOA creates a quote service with both platform
// credentials (for marketplace offers) and BYOA support.
func NewServiceWithPlatformCredsAndBYOA(
	queries *db.Queries,
	adapterPool *provider.AdapterPool,
	accountService *providerservice.AccountService,
	platformCreds map[domain.ProviderCode]*domain.ProviderCredentials,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:        queries,
		adapterPool:    adapterPool,
		accountService: accountService,
		platformCreds:  platformCreds,
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

// MarketplaceMarginPercent is the margin applied on top of provider prices for marketplace offers.
const MarketplaceMarginPercent = 15.0

// generateMarketplaceOffers generates offers by calling provider APIs with
// platform credentials and applying a margin on top of the real prices.
func (s *Service) generateMarketplaceOffers(ctx context.Context, quote *db.Quote, req *CreateQuoteRequest) ([]*db.Offer, error) {
	if s.adapterPool == nil || len(s.platformCreds) == 0 {
		s.logger.Debug("skipping marketplace offers: no platform credentials configured")
		return nil, nil
	}

	// Build address snapshots for provider API calls
	originSnapshot := addressToSnapshot(req.OriginAddress)
	destSnapshot := addressToSnapshot(req.DestinationAddress)

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

	var offers []*db.Offer

	for providerCode, creds := range s.platformCreds {
		adapter, err := s.adapterPool.Get(providerCode)
		if err != nil {
			s.logger.Error("failed to get adapter for marketplace quote",
				"error", err,
				"provider_code", providerCode,
			)
			continue
		}

		quoteResp, err := adapter.GetQuote(ctx, provider.QuoteRequest{
			Origin:             originSnapshot,
			Destination:        destSnapshot,
			Parcel:             *req.Parcel,
			CODAmountKurus:     codAmountKurus,
			DeclaredValueKurus: declaredValueKurus,
			Credentials:        creds,
		})
		if err != nil {
			// Skip providers that don't support GetQuote (e.g. Yurtici)
			var provErr *provider.ProviderError
			if errors.As(err, &provErr) && provErr.Code == "UNSUPPORTED_OPERATION" {
				s.logger.Debug("provider does not support GetQuote, skipping",
					"provider_code", providerCode,
				)
				continue
			}
			s.logger.Error("failed to get marketplace quote from provider",
				"error", err,
				"provider_code", providerCode,
			)
			continue
		}

		// Log provider-side errors (e.g. Aras returned Success=false)
		for _, provErr := range quoteResp.Errors {
			s.logger.Warn("provider returned error in quote response",
				"provider_code", providerCode,
				"error_code", provErr.Code,
				"error_message", provErr.Message,
				"provider_error_code", provErr.ProviderCode,
				"provider_error_message", provErr.ProviderMessage,
			)
		}

		for _, providerOffer := range quoteResp.Offers {
			expiresAt := providerOffer.ExpiresAt
			if expiresAt.IsZero() {
				expiresAt = time.Now().Add(QuoteExpiry)
			}

			// The cost price is what the provider charges us
			costSubtotal := providerOffer.BasePriceKurus + providerOffer.FuelSurchargeKurus + providerOffer.CODFeeKurus + providerOffer.InsuranceFeeKurus
			costTotal := providerOffer.TotalPriceKurus // includes VAT from provider

			// Apply margin on the subtotal (pre-VAT), then recalculate VAT
			marginKurus := int64(float64(costSubtotal) * MarketplaceMarginPercent / 100.0)
			sellSubtotal := costSubtotal + marginKurus
			sellVATKurus := int64(float64(sellSubtotal) * VATPercent / 100.0)
			sellTotal := sellSubtotal + sellVATKurus

			costTotalI32 := int32(costTotal)
			marginI32 := int32(marginKurus)

			offerParams := db.CreateOfferParams{
				QuoteID:            quote.ID,
				ProviderCode:       string(providerCode),
				ServiceCode:        providerOffer.ServiceCode,
				ProviderAccountID:  pgtype.UUID{}, // NULL for marketplace
				IsMarketplace:      true,
				BasePriceKurus:     int32(providerOffer.BasePriceKurus),
				FuelSurchargeKurus: int32(providerOffer.FuelSurchargeKurus),
				CodFeeKurus:        int32(providerOffer.CODFeeKurus),
				InsuranceFeeKurus:  int32(providerOffer.InsuranceFeeKurus),
				OtherFeesKurus:     int32(marginKurus), // margin stored as "other fees" on the sell side
				SubtotalKurus:      int32(sellSubtotal),
				VatKurus:           int32(sellVATKurus),
				TotalPriceKurus:    int32(sellTotal),
				CostPriceKurus:     &costTotalI32,
				MarginKurus:        &marginI32,
				EstimatedDaysMin:   int32(providerOffer.EstimatedDaysMin),
				EstimatedDaysMax:   int32(providerOffer.EstimatedDaysMax),
				ProviderQuoteID:    &providerOffer.ProviderQuoteID,
				ExpiresAt:          pgtype.Timestamptz{Time: expiresAt, Valid: true},
			}

			createdOffer, err := s.queries.CreateOffer(ctx, offerParams)
			if err != nil {
				s.logger.Error("failed to create marketplace offer",
					"error", err,
					"provider_code", providerCode,
					"service_code", providerOffer.ServiceCode,
				)
				continue
			}

			offers = append(offers, createdOffer)
		}
	}

	return offers, nil
}

// addressToSnapshot converts a domain.Address to a domain.AddressSnapshot.
func addressToSnapshot(addr *domain.Address) domain.AddressSnapshot {
	return domain.AddressSnapshot{
		ContactName:      addr.ContactName,
		CompanyName:      addr.CompanyName,
		Phone:            addr.Phone,
		Email:            addr.Email,
		StreetLine1:      addr.StreetLine1,
		StreetLine2:      addr.StreetLine2,
		Neighborhood:     addr.NeighborhoodCode,
		NeighborhoodCode: addr.NeighborhoodCode,
		District:         addr.DistrictCode,
		DistrictCode:     addr.DistrictCode,
		City:             addr.CityCode,
		CityCode:         addr.CityCode,
		PostalCode:       addr.PostalCode,
	}
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

