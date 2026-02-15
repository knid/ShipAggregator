// Package shopify provides the Shopify integration service for order processing,
// shipping rule evaluation, and fulfillment synchronization.
package shopify

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/crypto"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	shopifyclient "github.com/knid/kargonet/internal/shopify"
)

// Service handles Shopify order processing, rule evaluation, and fulfillment sync.
type Service struct {
	queries       *db.Queries
	encryptionKey []byte
	apiVersion    string
	logger        *observability.Logger
}

// NewService creates a new Shopify service.
func NewService(
	queries *db.Queries,
	encryptionKey []byte,
	apiVersion string,
	logger *observability.Logger,
) *Service {
	return &Service{
		queries:       queries,
		encryptionKey: encryptionKey,
		apiVersion:    apiVersion,
		logger:        logger,
	}
}

// ProcessOrder handles an incoming Shopify order webhook.
// It creates an order mapping and, if in auto mode, evaluates rules.
func (s *Service) ProcessOrder(ctx context.Context, shopDomain string, orderData []byte) error {
	// Look up store by shop domain
	store, err := s.queries.GetShopifyStoreByShopDomain(ctx, shopDomain)
	if err != nil {
		return fmt.Errorf("store not found for domain %s: %w", shopDomain, err)
	}

	if store.Status != "active" {
		s.logger.Info("skipping order for non-active store",
			"shop_domain", shopDomain,
			"status", store.Status,
		)
		return nil
	}

	// Parse the order
	var order shopifyclient.Order
	if err := json.Unmarshal(orderData, &order); err != nil {
		return fmt.Errorf("parsing order data: %w", err)
	}

	// Check if any line items require shipping
	hasPhysical := false
	for _, li := range order.LineItems {
		if li.RequiresShipping {
			hasPhysical = true
			break
		}
	}
	if !hasPhysical {
		s.logger.Info("skipping order with no physical items",
			"order_id", order.ID,
			"shop_domain", shopDomain,
		)
		return nil
	}

	// Calculate order total in kurus
	totalKurus := parsePriceToKurus(order.TotalPrice)

	// Build customer name
	customerName := ""
	if order.Customer != nil {
		customerName = order.Customer.FullName()
	}

	lineItemCount := int32(len(order.LineItems))

	// Create order mapping (idempotent via unique index)
	mapping, err := s.queries.CreateShopifyOrderMapping(ctx, db.CreateShopifyOrderMappingParams{
		OrganizationID:         store.OrganizationID,
		ShopifyStoreID:         store.ID,
		ShopifyOrderID:         order.ID,
		ShopifyOrderNumber:     &order.Name,
		Status:                 "pending",
		ShopifyCustomerName:    &customerName,
		ShopifyOrderTotalKurus: &totalKurus,
		ShopifyLineItemCount:   &lineItemCount,
		RawOrder:               orderData,
	})
	if err != nil {
		return fmt.Errorf("creating order mapping: %w", err)
	}

	// If mapping is nil, ON CONFLICT DO NOTHING fired — duplicate order
	if mapping == nil {
		s.logger.Info("duplicate order ignored",
			"order_id", order.ID,
			"shop_domain", shopDomain,
		)
		return nil
	}

	s.logger.Info("shopify order mapping created",
		"mapping_id", mapping.ID,
		"order_id", order.ID,
		"order_number", order.Name,
		"sync_mode", store.SyncMode,
	)

	// In auto mode, evaluate rules and attempt shipment creation
	if store.SyncMode == "auto" {
		if err := s.processAutoMode(ctx, store, mapping, &order); err != nil {
			// Mark as failed but don't return error (can be retried)
			syncErr := err.Error()
			_ = s.queries.UpdateShopifyOrderMappingStatus(ctx, db.UpdateShopifyOrderMappingStatusParams{
				ID:        mapping.ID,
				Status:    "failed",
				SyncError: &syncErr,
			})
			return fmt.Errorf("auto mode processing failed: %w", err)
		}
	}

	// Update store last sync timestamp
	_ = s.queries.UpdateShopifyStoreLastSync(ctx, store.ID)

	return nil
}

// processAutoMode evaluates shipping rules and creates a shipment for auto-mode stores.
func (s *Service) processAutoMode(ctx context.Context, store *db.ShopifyStore, mapping *db.ShopifyOrderMapping, order *shopifyclient.Order) error {
	// Mark as processing
	_ = s.queries.UpdateShopifyOrderMappingStatus(ctx, db.UpdateShopifyOrderMappingStatusParams{
		ID:     mapping.ID,
		Status: "processing",
	})

	// Map the address
	mappedAddr := MapShopifyAddress(ctx, s.queries, order.ShippingAddress, s.logger)
	if mappedAddr == nil || mappedAddr.CityCode == "" {
		return fmt.Errorf("could not resolve shipping address for order %d", order.ID)
	}

	// Build order context for rule evaluation
	orderTags := strings.Split(order.Tags, ",")
	for i := range orderTags {
		orderTags[i] = strings.TrimSpace(orderTags[i])
	}

	orderCtx := &OrderContext{
		DestinationCityCode: mappedAddr.CityCode,
		WeightGrams:         int(order.TotalWeight),
		OrderTotalKurus:     parsePriceToKurus(order.TotalPrice),
		HasCOD:              order.FinancialStatus == "pending", // COD orders are typically "pending"
		Tags:                orderTags,
	}

	storeID := store.ID
	rule, err := EvaluateRules(ctx, s.queries, store.OrganizationID, &storeID, orderCtx)
	if err != nil {
		return fmt.Errorf("evaluating shipping rules: %w", err)
	}
	if rule == nil {
		return fmt.Errorf("no matching shipping rule for order %d", order.ID)
	}

	s.logger.Info("shipping rule matched",
		"mapping_id", mapping.ID,
		"rule_id", rule.ID,
		"rule_name", rule.Name,
		"provider_code", rule.ActionProviderCode,
	)

	// NOTE: Actual shipment creation via quote → offer → shipment flow
	// would be done here by calling quoteService.Create() and then
	// shipmentService.CreateFromOffer(). This requires injecting those
	// services as dependencies. For now, we mark as draft_created
	// and store the matched rule info. Full auto-creation will be
	// connected when the service is wired with QuoteService and ShipmentService.

	_ = s.queries.UpdateShopifyOrderMappingStatus(ctx, db.UpdateShopifyOrderMappingStatusParams{
		ID:     mapping.ID,
		Status: "draft_created",
	})

	return nil
}

// ProcessOrderCancellation handles a Shopify order cancellation webhook.
func (s *Service) ProcessOrderCancellation(ctx context.Context, shopDomain string, orderData []byte) error {
	store, err := s.queries.GetShopifyStoreByShopDomain(ctx, shopDomain)
	if err != nil {
		return fmt.Errorf("store not found for domain %s: %w", shopDomain, err)
	}

	var order shopifyclient.Order
	if err := json.Unmarshal(orderData, &order); err != nil {
		return fmt.Errorf("parsing order data: %w", err)
	}

	// Find existing mapping
	mapping, err := s.queries.GetShopifyOrderMappingByOrderID(ctx, db.GetShopifyOrderMappingByOrderIDParams{
		ShopifyStoreID: store.ID,
		ShopifyOrderID: order.ID,
	})
	if err != nil {
		s.logger.Info("no mapping found for cancelled order",
			"order_id", order.ID,
			"shop_domain", shopDomain,
		)
		return nil
	}

	// Update mapping status
	if err := s.queries.UpdateShopifyOrderMappingStatus(ctx, db.UpdateShopifyOrderMappingStatusParams{
		ID:     mapping.ID,
		Status: "cancelled",
	}); err != nil {
		return fmt.Errorf("updating mapping status: %w", err)
	}

	s.logger.Info("shopify order mapping cancelled",
		"mapping_id", mapping.ID,
		"order_id", order.ID,
	)

	return nil
}

// SyncFulfillment pushes tracking info back to Shopify for a shipment.
func (s *Service) SyncFulfillment(ctx context.Context, mappingID uuid.UUID, trackingNumber, trackingCompany, trackingURL string) error {
	mapping, err := s.queries.GetShopifyOrderMappingByID(ctx, mappingID)
	if err != nil {
		return fmt.Errorf("mapping not found: %w", err)
	}

	// Get store with access token
	store, err := s.queries.GetShopifyStoreByID(ctx, mapping.ShopifyStoreID)
	if err != nil {
		return fmt.Errorf("store not found: %w", err)
	}

	if store.Status != "active" {
		return fmt.Errorf("store is not active (status: %s)", store.Status)
	}

	// Decrypt access token
	accessToken, err := crypto.DecryptCredentialsWithIV(store.AccessTokenEncrypted, store.AccessTokenIv, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("decrypting access token: %w", err)
	}

	// Create Shopify client and push fulfillment
	client := shopifyclient.NewClient(store.ShopDomain, string(accessToken), s.apiVersion)
	fulfillResp, err := client.CreateFulfillment(ctx, &shopifyclient.FulfillmentRequest{
		OrderID:         mapping.ShopifyOrderID,
		TrackingNumber:  trackingNumber,
		TrackingCompany: trackingCompany,
		TrackingURL:     trackingURL,
	})
	if err != nil {
		return fmt.Errorf("creating Shopify fulfillment: %w", err)
	}

	// Update mapping with fulfillment ID
	fulfillmentID := fulfillResp.Fulfillment.ID
	if err := s.queries.UpdateShopifyOrderFulfillment(ctx, db.UpdateShopifyOrderFulfillmentParams{
		ID:                   mapping.ID,
		ShopifyFulfillmentID: &fulfillmentID,
	}); err != nil {
		return fmt.Errorf("updating fulfillment record: %w", err)
	}

	s.logger.Info("fulfillment synced to Shopify",
		"mapping_id", mapping.ID,
		"order_id", mapping.ShopifyOrderID,
		"fulfillment_id", fulfillmentID,
		"tracking_number", trackingNumber,
	)

	return nil
}

// parsePriceToKurus converts a Shopify price string (e.g. "150.00") to kurus (integer cents).
func parsePriceToKurus(price string) int64 {
	f, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(f * 100))
}
