package tasks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	shopifyservice "github.com/knid/kargonet/internal/service/shopify"
)

// ShopifyFulfillmentHandler processes Shopify fulfillment sync tasks.
type ShopifyFulfillmentHandler struct {
	shopifyService *shopifyservice.Service
	queries        *db.Queries
	asynqClient    *asynq.Client
	logger         *observability.Logger
}

// NewShopifyFulfillmentHandler creates a new Shopify fulfillment handler.
func NewShopifyFulfillmentHandler(
	shopifyService *shopifyservice.Service,
	queries *db.Queries,
	asynqClient *asynq.Client,
	logger *observability.Logger,
) *ShopifyFulfillmentHandler {
	return &ShopifyFulfillmentHandler{
		shopifyService: shopifyService,
		queries:        queries,
		asynqClient:    asynqClient,
		logger:         logger,
	}
}

// ProcessSyncTask handles the shopify:fulfillment_sync task.
func (h *ShopifyFulfillmentHandler) ProcessSyncTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseShopifyFulfillmentSyncPayload(task)
	if err != nil {
		return fmt.Errorf("parsing fulfillment sync payload: %w", err)
	}

	mappingID, err := uuid.Parse(payload.MappingID)
	if err != nil {
		return fmt.Errorf("invalid mapping ID: %w", err)
	}

	h.logger.Info("syncing fulfillment to Shopify",
		"mapping_id", payload.MappingID,
		"tracking_number", payload.TrackingNumber,
	)

	if err := h.shopifyService.SyncFulfillment(ctx, mappingID, payload.TrackingNumber, payload.TrackingCompany, payload.TrackingURL); err != nil {
		h.logger.Error("failed to sync fulfillment",
			"error", err,
			"mapping_id", payload.MappingID,
		)
		return err
	}

	return nil
}

// ProcessRetryTask handles the shopify:fulfillment_retry periodic task.
// It finds all mappings with label_purchased status that haven't been synced and enqueues sync tasks.
func (h *ShopifyFulfillmentHandler) ProcessRetryTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseShopifyFulfillmentRetryPayload(task)
	if err != nil {
		return fmt.Errorf("parsing fulfillment retry payload: %w", err)
	}

	h.logger.Info("retrying pending fulfillment syncs", "batch_size", payload.BatchSize)

	pending, err := h.queries.ListPendingFulfillmentSyncs(ctx, int32(payload.BatchSize))
	if err != nil {
		return fmt.Errorf("listing pending fulfillment syncs: %w", err)
	}

	if len(pending) == 0 {
		h.logger.Info("no pending fulfillment syncs found")
		return nil
	}

	enqueued := 0
	for _, m := range pending {
		// We need tracking info from the shipment — for now just enqueue with mapping ID
		// The sync task handler will look up the tracking details
		syncTask, err := NewShopifyFulfillmentSyncTask(m.ID.String(), "", "", "")
		if err != nil {
			h.logger.Error("failed to create fulfillment sync task",
				"mapping_id", m.ID,
				"error", err,
			)
			continue
		}

		if _, err := h.asynqClient.EnqueueContext(ctx, syncTask); err != nil {
			h.logger.Error("failed to enqueue fulfillment sync task",
				"mapping_id", m.ID,
				"error", err,
			)
			continue
		}
		enqueued++
	}

	h.logger.Info("fulfillment retry batch complete",
		"total_pending", len(pending),
		"enqueued", enqueued,
	)

	return nil
}
