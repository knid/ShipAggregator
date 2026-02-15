package integration

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/knid/kargonet/internal/config"
	"github.com/knid/kargonet/internal/crypto"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	"github.com/knid/kargonet/internal/shopify"
	"github.com/knid/kargonet/internal/worker/tasks"
)

// ShopifyWebhookHandler handles webhooks and store registration from Shopify.
type ShopifyWebhookHandler struct {
	queries       *db.Queries
	asynqClient   *asynq.Client
	encryptionKey []byte
	shopifyCfg    config.ShopifyConfig
	logger        *observability.Logger
}

// NewShopifyWebhookHandler creates a new Shopify webhook handler.
func NewShopifyWebhookHandler(
	queries *db.Queries,
	asynqClient *asynq.Client,
	encryptionKey []byte,
	shopifyCfg config.ShopifyConfig,
	logger *observability.Logger,
) *ShopifyWebhookHandler {
	return &ShopifyWebhookHandler{
		queries:       queries,
		asynqClient:   asynqClient,
		encryptionKey: encryptionKey,
		shopifyCfg:    shopifyCfg,
		logger:        logger,
	}
}

// HandleWebhook receives Shopify webhook callbacks.
// It verifies the HMAC signature, identifies the store, and routes by topic.
func (h *ShopifyWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read body (1MB limit)
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		h.logger.Error("failed to read Shopify webhook body", "error", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify HMAC signature
	hmacHeader := r.Header.Get("X-Shopify-Hmac-SHA256")
	if !shopify.VerifyWebhookHMAC(body, hmacHeader, h.shopifyCfg.APISecret) {
		h.logger.Warn("invalid Shopify webhook HMAC",
			"shop_domain", r.Header.Get("X-Shopify-Shop-Domain"),
		)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	shopDomain := r.Header.Get("X-Shopify-Shop-Domain")
	topic := r.Header.Get("X-Shopify-Topic")

	h.logger.Info("received Shopify webhook",
		"shop_domain", shopDomain,
		"topic", topic,
	)

	switch topic {
	case "orders/paid":
		task, err := tasks.NewShopifyOrderProcessTask(shopDomain, body)
		if err != nil {
			h.logger.Error("failed to create order process task", "error", err, "shop_domain", shopDomain)
			break
		}
		if _, err := h.asynqClient.EnqueueContext(ctx, task); err != nil {
			h.logger.Error("failed to enqueue order process task", "error", err, "shop_domain", shopDomain)
		}

	case "orders/cancelled":
		task, err := tasks.NewShopifyOrderCancelTask(shopDomain, body)
		if err != nil {
			h.logger.Error("failed to create order cancel task", "error", err, "shop_domain", shopDomain)
			break
		}
		if _, err := h.asynqClient.EnqueueContext(ctx, task); err != nil {
			h.logger.Error("failed to enqueue order cancel task", "error", err, "shop_domain", shopDomain)
		}

	case "app/uninstalled":
		if err := h.queries.MarkShopifyStoreUninstalled(ctx, shopDomain); err != nil {
			h.logger.Error("failed to mark store uninstalled", "error", err, "shop_domain", shopDomain)
		} else {
			h.logger.Info("shopify store marked uninstalled", "shop_domain", shopDomain)
		}

	default:
		h.logger.Warn("unhandled Shopify webhook topic", "topic", topic, "shop_domain", shopDomain)
	}

	// Always return 200 OK — processing is async
	w.WriteHeader(http.StatusOK)
}

// RegisterStoreRequest is the request body for registering a Shopify store.
type RegisterStoreRequest struct {
	ShopDomain     string `json:"shop_domain"`
	AccessToken    string `json:"access_token"`
	OrganizationID string `json:"organization_id"`
}

// HandleRegisterStore registers a Shopify store after OAuth completion.
// Called by the embedded app with service secret auth.
func (h *ShopifyWebhookHandler) HandleRegisterStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify service secret
	serviceSecret := r.Header.Get("X-Service-Secret")
	if serviceSecret == "" || serviceSecret != h.shopifyCfg.ServiceSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req RegisterStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ShopDomain == "" || req.AccessToken == "" || req.OrganizationID == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	orgID, err := uuid.Parse(req.OrganizationID)
	if err != nil {
		http.Error(w, "invalid organization_id", http.StatusBadRequest)
		return
	}

	// Encrypt access token
	ciphertext, iv, err := crypto.EncryptCredentialsWithIV([]byte(req.AccessToken), h.encryptionKey)
	if err != nil {
		h.logger.Error("failed to encrypt Shopify token", "error", err, "shop_domain", req.ShopDomain)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fetch shop info from Shopify
	client := shopify.NewClient(req.ShopDomain, req.AccessToken, h.shopifyCfg.APIVersion)
	shopInfo, err := client.GetShop(ctx)
	if err != nil {
		h.logger.Error("failed to fetch Shopify shop info", "error", err, "shop_domain", req.ShopDomain)
		http.Error(w, "failed to verify shop", http.StatusBadGateway)
		return
	}

	// Create store record
	store, err := h.queries.CreateShopifyStore(ctx, db.CreateShopifyStoreParams{
		OrganizationID:       orgID,
		ShopDomain:           req.ShopDomain,
		AccessTokenEncrypted: ciphertext,
		AccessTokenIv:        iv,
		Scopes:               &h.shopifyCfg.Scopes,
		ShopifyStoreID:       &shopInfo.ID,
		StoreName:            &shopInfo.Name,
		StoreEmail:           &shopInfo.Email,
		Currency:             &shopInfo.Currency,
		Status:               "active",
		SyncMode:             "manual",
	})
	if err != nil {
		h.logger.Error("failed to create Shopify store", "error", err, "shop_domain", req.ShopDomain)
		http.Error(w, "failed to save store", http.StatusInternalServerError)
		return
	}

	// Register webhooks pointing to our backend
	callbackURL := h.shopifyCfg.AppURL + "/integrations/shopify/webhook"
	webhookTopics := []string{"orders/paid", "orders/cancelled", "app/uninstalled"}
	if err := client.RegisterWebhooks(ctx, webhookTopics, callbackURL); err != nil {
		h.logger.Error("failed to register Shopify webhooks", "error", err, "shop_domain", req.ShopDomain)
		// Non-fatal — webhooks can be registered later
	}

	h.logger.Info("shopify store registered",
		"store_id", store.ID,
		"shop_domain", req.ShopDomain,
		"organization_id", orgID,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"store_id": store.ID.String(),
		"status":   "active",
	})
}
