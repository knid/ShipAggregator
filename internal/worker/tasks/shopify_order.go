package tasks

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/knid/kargonet/internal/observability"
	shopifyservice "github.com/knid/kargonet/internal/service/shopify"
)

// ShopifyOrderHandler processes Shopify order tasks.
type ShopifyOrderHandler struct {
	shopifyService *shopifyservice.Service
	logger         *observability.Logger
}

// NewShopifyOrderHandler creates a new Shopify order handler.
func NewShopifyOrderHandler(
	shopifyService *shopifyservice.Service,
	logger *observability.Logger,
) *ShopifyOrderHandler {
	return &ShopifyOrderHandler{
		shopifyService: shopifyService,
		logger:         logger,
	}
}

// ProcessOrderTask handles the shopify:order_process task.
func (h *ShopifyOrderHandler) ProcessOrderTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseShopifyOrderPayload(task)
	if err != nil {
		return fmt.Errorf("parsing shopify order payload: %w", err)
	}

	h.logger.Info("processing Shopify order",
		"shop_domain", payload.ShopDomain,
	)

	if err := h.shopifyService.ProcessOrder(ctx, payload.ShopDomain, payload.OrderData); err != nil {
		h.logger.Error("failed to process Shopify order",
			"error", err,
			"shop_domain", payload.ShopDomain,
		)
		return err
	}

	return nil
}

// ProcessCancelTask handles the shopify:order_cancel task.
func (h *ShopifyOrderHandler) ProcessCancelTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseShopifyOrderPayload(task)
	if err != nil {
		return fmt.Errorf("parsing shopify cancel payload: %w", err)
	}

	h.logger.Info("processing Shopify order cancellation",
		"shop_domain", payload.ShopDomain,
	)

	if err := h.shopifyService.ProcessOrderCancellation(ctx, payload.ShopDomain, payload.OrderData); err != nil {
		h.logger.Error("failed to process Shopify order cancellation",
			"error", err,
			"shop_domain", payload.ShopDomain,
		)
		return err
	}

	return nil
}
