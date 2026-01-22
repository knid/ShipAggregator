// Package shipment provides shipment management services.
package shipment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/provider"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/billing"
	providerservice "github.com/e-port/shipping-aggregator/internal/service/provider"
)

// Common errors for shipment operations.
var (
	ErrShipmentNotFound        = errors.New("shipment not found")
	ErrOfferNotFound           = errors.New("offer not found")
	ErrOfferExpired            = errors.New("offer has expired")
	ErrInvalidShipmentStatus   = errors.New("invalid shipment status for this operation")
	ErrLabelAlreadyPurchased   = errors.New("label has already been purchased")
	ErrInsufficientBalance     = errors.New("insufficient wallet balance")
	ErrProviderError           = errors.New("provider error")
	ErrProviderAccountNotFound = errors.New("provider account not found")
	ErrProviderAccountInvalid  = errors.New("provider account is not verified")
)

// Service provides shipment management operations.
type Service struct {
	queries        *db.Queries
	adapterPool    *provider.AdapterPool
	ledgerService  *billing.LedgerService
	walletService  *billing.WalletService
	accountService *providerservice.AccountService
	logger         *observability.Logger
}

// NewService creates a new shipment service.
func NewService(
	queries *db.Queries,
	adapterPool *provider.AdapterPool,
	ledgerService *billing.LedgerService,
	walletService *billing.WalletService,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:       queries,
		adapterPool:   adapterPool,
		ledgerService: ledgerService,
		walletService: walletService,
		logger:        logger,
	}
}

// NewServiceWithBYOA creates a new shipment service with BYOA support.
func NewServiceWithBYOA(
	queries *db.Queries,
	adapterPool *provider.AdapterPool,
	ledgerService *billing.LedgerService,
	walletService *billing.WalletService,
	accountService *providerservice.AccountService,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:        queries,
		adapterPool:    adapterPool,
		ledgerService:  ledgerService,
		walletService:  walletService,
		accountService: accountService,
		logger:         logger,
	}
}

// CreateFromOfferRequest represents a request to create a shipment from an offer.
type CreateFromOfferRequest struct {
	OrganizationID uuid.UUID
	OfferID        uuid.UUID
	IdempotencyKey string
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
	Metadata       map[string]interface{}
}

// ShipmentResult represents the result of a shipment operation.
type ShipmentResult struct {
	Shipment *db.Shipment
}

// CreateFromOffer creates a new shipment from an offer.
func (s *Service) CreateFromOffer(ctx context.Context, req *CreateFromOfferRequest) (*ShipmentResult, error) {
	// Check for existing shipment with same idempotency key
	existingShipment, err := s.queries.GetShipmentByIdempotencyKey(ctx, db.GetShipmentByIdempotencyKeyParams{
		OrganizationID: req.OrganizationID,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err == nil {
		return &ShipmentResult{Shipment: existingShipment}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to check idempotency: %w", err)
	}

	// Get the offer
	offer, err := s.queries.GetActiveOfferByID(ctx, req.OfferID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOfferNotFound
		}
		return nil, fmt.Errorf("failed to get offer: %w", err)
	}

	// Check if offer has expired
	if offer.ExpiresAt.Valid && time.Now().After(offer.ExpiresAt.Time) {
		return nil, ErrOfferExpired
	}

	// Get the quote to retrieve addresses and parcel info
	quote, err := s.queries.GetQuoteByID(ctx, offer.QuoteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	// Verify organization ownership
	if quote.OrganizationID != req.OrganizationID {
		return nil, ErrOfferNotFound
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// Create provider account ID pgtype
	var providerAccountID pgtype.UUID
	if offer.ProviderAccountID.Valid {
		providerAccountID = offer.ProviderAccountID
	}

	// Create offer ID pgtype
	offerID := pgtype.UUID{Bytes: offer.ID, Valid: true}

	// Create shipment in draft status
	shipment, err := s.queries.CreateShipment(ctx, db.CreateShipmentParams{
		OrganizationID:     req.OrganizationID,
		IdempotencyKey:     req.IdempotencyKey,
		OfferID:            offerID,
		ProviderCode:       offer.ProviderCode,
		ServiceCode:        offer.ServiceCode,
		ProviderAccountID:  providerAccountID,
		IsMarketplace:      offer.IsMarketplace,
		OriginAddress:      quote.OriginAddress,
		DestinationAddress: quote.DestinationAddress,
		Parcel:             quote.Parcel,
		CodAmountKurus:     quote.CodAmountKurus,
		DeclaredValueKurus: quote.DeclaredValueKurus,
		Status:             domain.ShipmentStatusDraft.String(),
		Metadata:           metadataJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	s.logger.Info("Shipment created from offer",
		"shipment_id", shipment.ID,
		"offer_id", offer.ID,
		"organization_id", req.OrganizationID,
		"provider_code", offer.ProviderCode,
		"service_code", offer.ServiceCode,
	)

	return &ShipmentResult{Shipment: shipment}, nil
}

// PurchaseLabelRequest represents a request to purchase a label for a shipment.
type PurchaseLabelRequest struct {
	ShipmentID     uuid.UUID
	OrganizationID uuid.UUID
	LabelFormat    domain.LabelFormat
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
}

// PurchaseLabelResult represents the result of purchasing a label.
type PurchaseLabelResult struct {
	Shipment       *db.Shipment
	TrackingNumber string
	LabelDocument  *db.Document
}

// PurchaseLabel purchases a shipping label for a shipment.
func (s *Service) PurchaseLabel(ctx context.Context, req *PurchaseLabelRequest) (*PurchaseLabelResult, error) {
	// Get shipment
	shipment, err := s.queries.GetShipmentForUpdate(ctx, req.ShipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to get shipment: %w", err)
	}

	// Verify organization ownership
	if shipment.OrganizationID != req.OrganizationID {
		return nil, ErrShipmentNotFound
	}

	// Check shipment status
	if shipment.Status != domain.ShipmentStatusDraft.String() {
		return nil, ErrLabelAlreadyPurchased
	}

	// Determine if this is a BYOA shipment
	isBYOA := !shipment.IsMarketplace && shipment.ProviderAccountID.Valid

	// Get offer to determine price
	var priceKurus int32
	var costKurus *int32
	if shipment.OfferID.Valid {
		offer, err := s.queries.GetOfferByID(ctx, shipment.OfferID.Bytes)
		if err == nil {
			priceKurus = offer.TotalPriceKurus
			costKurus = offer.CostPriceKurus
		}
	}

	// For marketplace shipments, check wallet balance and charge
	// For BYOA shipments, customer pays provider directly - no wallet charge
	if !isBYOA {
		hasSufficientBalance, err := s.walletService.HasSufficientBalance(ctx, req.OrganizationID, int64(priceKurus))
		if err != nil {
			return nil, fmt.Errorf("failed to check balance: %w", err)
		}
		if !hasSufficientBalance {
			return nil, ErrInsufficientBalance
		}
	}

	// Parse addresses and parcel
	var originAddress domain.AddressSnapshot
	if err := json.Unmarshal(shipment.OriginAddress, &originAddress); err != nil {
		return nil, fmt.Errorf("failed to parse origin address: %w", err)
	}

	var destAddress domain.AddressSnapshot
	if err := json.Unmarshal(shipment.DestinationAddress, &destAddress); err != nil {
		return nil, fmt.Errorf("failed to parse destination address: %w", err)
	}

	var parcel domain.Parcel
	if err := json.Unmarshal(shipment.Parcel, &parcel); err != nil {
		return nil, fmt.Errorf("failed to parse parcel: %w", err)
	}

	// Get provider adapter
	providerCode := domain.ProviderCode(shipment.ProviderCode)
	adapter, err := s.adapterPool.Get(providerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider adapter: %w", err)
	}

	// Prepare COD amount
	var codAmountKurus *int64
	if shipment.CodAmountKurus != nil {
		cod := int64(*shipment.CodAmountKurus)
		codAmountKurus = &cod
	}

	// Prepare declared value
	var declaredValueKurus *int64
	if shipment.DeclaredValueKurus != nil {
		dv := int64(*shipment.DeclaredValueKurus)
		declaredValueKurus = &dv
	}

	// Build create shipment request
	createReq := provider.CreateShipmentRequest{
		Origin:             originAddress,
		Destination:        destAddress,
		Parcel:             parcel,
		ServiceCode:        shipment.ServiceCode,
		CODAmountKurus:     codAmountKurus,
		DeclaredValueKurus: declaredValueKurus,
		Reference:          shipment.IdempotencyKey,
		LabelFormat:        req.LabelFormat,
	}

	// For BYOA shipments, get and use customer credentials
	if isBYOA && s.accountService != nil {
		// Get provider account
		account, err := s.accountService.GetAccount(ctx, shipment.ProviderAccountID.Bytes, req.OrganizationID)
		if err != nil {
			if errors.Is(err, providerservice.ErrAccountNotFound) {
				return nil, ErrProviderAccountNotFound
			}
			return nil, fmt.Errorf("failed to get provider account: %w", err)
		}

		// Verify account is verified
		if account.Status != domain.ProviderAccountStatusVerified.String() {
			return nil, ErrProviderAccountInvalid
		}

		// Decrypt credentials
		credentials, err := s.accountService.DecryptCredentials(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
		}

		createReq.Credentials = credentials

		// Record account usage
		_ = s.accountService.RecordUsage(ctx, account.ID)
	}

	// Create shipment with provider
	createResp, err := adapter.CreateShipment(ctx, createReq)
	if err != nil {
		s.logger.Error("Provider shipment creation failed",
			"error", err,
			"shipment_id", shipment.ID,
			"provider_code", shipment.ProviderCode,
			"is_byoa", isBYOA,
		)
		return nil, fmt.Errorf("%w: %v", ErrProviderError, err)
	}

	// Store label document
	var labelDocumentID pgtype.UUID
	var labelDocument *db.Document
	if createResp.Label != nil && len(createResp.Label.Content) > 0 {
		// Create document record
		doc, err := s.queries.CreateDocument(ctx, db.CreateDocumentParams{
			OrganizationID: req.OrganizationID,
			Type:           domain.DocumentTypeLabel.String(),
			Format:         string(createResp.Label.Format),
			Filename:       fmt.Sprintf("label_%s.%s", createResp.TrackingNumber, createResp.Label.Format),
			StoragePath:    fmt.Sprintf("labels/%s/%s", req.OrganizationID, shipment.ID),
			StorageBucket:  "documents",
			ContentType:    createResp.Label.ContentType,
			SizeBytes:      int32(len(createResp.Label.Content)),
			ContentHash:    "", // TODO: Calculate hash
			Metadata:       []byte("{}"),
		})
		if err != nil {
			s.logger.Error("Failed to create label document", "error", err)
		} else {
			labelDocumentID = pgtype.UUID{Bytes: doc.ID, Valid: true}
			labelDocument = doc
		}
	}

	// For BYOA shipments, don't charge wallet - set price to 0 for tracking
	chargedPriceKurus := priceKurus
	if isBYOA {
		chargedPriceKurus = 0
	}

	// Update shipment with provider details
	labelFormat := string(req.LabelFormat)
	updatedShipment, err := s.queries.UpdateShipmentAfterLabelPurchase(ctx, db.UpdateShipmentAfterLabelPurchaseParams{
		ID:                     shipment.ID,
		ProviderShipmentID:     &createResp.ProviderShipmentID,
		ProviderTrackingNumber: &createResp.TrackingNumber,
		ProviderBarcode:        &createResp.Barcode,
		LabelDocumentID:        labelDocumentID,
		LabelFormat:            &labelFormat,
		PriceChargedKurus:      &chargedPriceKurus,
		CostKurus:              costKurus,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update shipment: %w", err)
	}

	// Debit wallet only for marketplace shipments
	if !isBYOA && priceKurus > 0 {
		_, err = s.ledgerService.Debit(ctx, &billing.DebitRequest{
			OrganizationID: req.OrganizationID,
			AmountKurus:    int64(priceKurus),
			ReferenceType:  domain.ReferenceTypeShipment,
			ReferenceID:    shipment.ID,
			Description:    fmt.Sprintf("Shipment %s - %s %s", createResp.TrackingNumber, shipment.ProviderCode, shipment.ServiceCode),
			Metadata: map[string]interface{}{
				"shipment_id":     shipment.ID.String(),
				"tracking_number": createResp.TrackingNumber,
				"provider_code":   shipment.ProviderCode,
				"service_code":    shipment.ServiceCode,
			},
			ActorID:   req.ActorID,
			ActorType: req.ActorType,
		})
		if err != nil {
			s.logger.Error("Failed to debit wallet for shipment",
				"error", err,
				"shipment_id", shipment.ID,
				"amount_kurus", priceKurus,
			)
			// Note: In production, this should be handled with a saga/compensation pattern
			// For now, we log the error but don't fail the operation
		}
	}

	s.logger.Info("Label purchased successfully",
		"shipment_id", shipment.ID,
		"tracking_number", createResp.TrackingNumber,
		"provider_code", shipment.ProviderCode,
		"price_kurus", chargedPriceKurus,
		"is_byoa", isBYOA,
	)

	return &PurchaseLabelResult{
		Shipment:       updatedShipment,
		TrackingNumber: createResp.TrackingNumber,
		LabelDocument:  labelDocument,
	}, nil
}

// Get retrieves a shipment by ID.
func (s *Service) Get(ctx context.Context, shipmentID, organizationID uuid.UUID) (*db.Shipment, error) {
	shipment, err := s.queries.GetShipmentByID(ctx, shipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}

	// Verify organization ownership
	if shipment.OrganizationID != organizationID {
		return nil, ErrShipmentNotFound
	}

	return shipment, nil
}

// GetByTrackingNumber retrieves a shipment by tracking number.
func (s *Service) GetByTrackingNumber(ctx context.Context, providerCode, trackingNumber string) (*db.Shipment, error) {
	shipment, err := s.queries.GetShipmentByTrackingNumber(ctx, db.GetShipmentByTrackingNumberParams{
		ProviderCode:           providerCode,
		ProviderTrackingNumber: &trackingNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, err
	}
	return shipment, nil
}

// ListRequest represents a request to list shipments.
type ListRequest struct {
	OrganizationID uuid.UUID
	Status         *string
	ProviderCode   *string
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int
	Limit          int
}

// ListResult represents the result of listing shipments.
type ListResult struct {
	Shipments []*db.Shipment
	Total     int64
}

// List lists shipments for an organization with filtering.
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResult, error) {
	// Build query params
	params := db.ListShipmentsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
		ProviderCode:   req.ProviderCode,
		OffsetCount:    int32(req.Offset),
		LimitCount:     int32(req.Limit),
	}

	if req.CreatedAfter != nil {
		params.CreatedAfter = pgtype.Timestamptz{Time: *req.CreatedAfter, Valid: true}
	}
	if req.CreatedBefore != nil {
		params.CreatedBefore = pgtype.Timestamptz{Time: *req.CreatedBefore, Valid: true}
	}

	// Get count
	countParams := db.CountShipmentsByOrganizationParams{
		OrganizationID: req.OrganizationID,
		Status:         req.Status,
		ProviderCode:   req.ProviderCode,
		CreatedAfter:   params.CreatedAfter,
		CreatedBefore:  params.CreatedBefore,
	}
	total, err := s.queries.CountShipmentsByOrganization(ctx, countParams)
	if err != nil {
		return nil, fmt.Errorf("failed to count shipments: %w", err)
	}

	// Get shipments
	shipments, err := s.queries.ListShipmentsByOrganization(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list shipments: %w", err)
	}

	return &ListResult{
		Shipments: shipments,
		Total:     total,
	}, nil
}

// CancelRequest represents a request to cancel a shipment.
type CancelRequest struct {
	ShipmentID     uuid.UUID
	OrganizationID uuid.UUID
	Reason         string
	ActorID        *uuid.UUID
	ActorType      domain.ActorType
}

// Cancel cancels a shipment.
func (s *Service) Cancel(ctx context.Context, req *CancelRequest) (*db.Shipment, error) {
	// Get shipment
	shipment, err := s.queries.GetShipmentForUpdate(ctx, req.ShipmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShipmentNotFound
		}
		return nil, fmt.Errorf("failed to get shipment: %w", err)
	}

	// Verify organization ownership
	if shipment.OrganizationID != req.OrganizationID {
		return nil, ErrShipmentNotFound
	}

	// Check if shipment can be cancelled
	status := domain.ShipmentStatus(shipment.Status)
	if !status.CanBeCancelled() {
		return nil, ErrInvalidShipmentStatus
	}

	// If label was purchased, try to cancel with provider
	if shipment.ProviderShipmentID != nil && *shipment.ProviderShipmentID != "" {
		providerCode := domain.ProviderCode(shipment.ProviderCode)
		adapter, err := s.adapterPool.Get(providerCode)
		if err == nil {
			_, cancelErr := adapter.CancelShipment(ctx, provider.CancelShipmentRequest{
				ProviderShipmentID: *shipment.ProviderShipmentID,
				TrackingNumber:     *shipment.ProviderTrackingNumber,
				Reason:             req.Reason,
			})
			if cancelErr != nil {
				s.logger.Error("Failed to cancel shipment with provider",
					"error", cancelErr,
					"shipment_id", shipment.ID,
					"provider_code", shipment.ProviderCode,
				)
				// Continue with local cancellation even if provider fails
			}
		}

		// Refund wallet if label was purchased
		if shipment.PriceChargedKurus != nil && *shipment.PriceChargedKurus > 0 {
			_, err = s.ledgerService.Credit(ctx, &billing.CreditRequest{
				OrganizationID: req.OrganizationID,
				AmountKurus:    int64(*shipment.PriceChargedKurus),
				ReferenceType:  domain.ReferenceTypeRefund,
				ReferenceID:    shipment.ID,
				Description:    fmt.Sprintf("Refund for cancelled shipment %s", *shipment.ProviderTrackingNumber),
				Metadata: map[string]interface{}{
					"shipment_id":     shipment.ID.String(),
					"tracking_number": *shipment.ProviderTrackingNumber,
					"reason":          req.Reason,
				},
				ActorID:   req.ActorID,
				ActorType: req.ActorType,
			})
			if err != nil {
				s.logger.Error("Failed to refund wallet for cancelled shipment",
					"error", err,
					"shipment_id", shipment.ID,
				)
			}
		}
	}

	// Update shipment status
	updatedShipment, err := s.queries.UpdateShipmentCancelled(ctx, db.UpdateShipmentCancelledParams{
		ID:                 shipment.ID,
		CancellationReason: &req.Reason,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cancel shipment: %w", err)
	}

	s.logger.Info("Shipment cancelled",
		"shipment_id", shipment.ID,
		"reason", req.Reason,
	)

	return updatedShipment, nil
}

// UpdateStatus updates the status of a shipment.
func (s *Service) UpdateStatus(ctx context.Context, shipmentID uuid.UUID, status domain.ShipmentStatus) (*db.Shipment, error) {
	return s.queries.UpdateShipmentStatus(ctx, db.UpdateShipmentStatusParams{
		ID:     shipmentID,
		Status: status.String(),
	})
}
