// Package integration provides handlers for inbound webhooks from external services.
package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/payment/iyzico"
	"github.com/e-port/shipping-aggregator/internal/service/billing"
)

// IyzicoHandler handles Iyzico payment callbacks and webhooks.
type IyzicoHandler struct {
	topupService *billing.TopupService
	iyzicoClient *iyzico.Client
	logger       *observability.Logger
	returnURL    string
}

// NewIyzicoHandler creates a new Iyzico handler.
func NewIyzicoHandler(
	topupService *billing.TopupService,
	iyzicoClient *iyzico.Client,
	logger *observability.Logger,
	returnURL string,
) *IyzicoHandler {
	return &IyzicoHandler{
		topupService: topupService,
		iyzicoClient: iyzicoClient,
		logger:       logger,
		returnURL:    returnURL,
	}
}

// HandleCallback handles the Iyzico checkout form callback.
// This is called when the user completes payment on the Iyzico checkout page.
// Iyzico sends a POST request with form data containing the token.
// Endpoint: POST /integrations/iyzico/callback
func (h *IyzicoHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.logger.Error("Failed to parse Iyzico callback form", "error", err)
		h.redirectWithError(w, r, "invalid_request", "Failed to parse callback data")
		return
	}

	// Get token from form data
	token := r.FormValue("token")
	if token == "" {
		h.logger.Error("Iyzico callback missing token")
		h.redirectWithError(w, r, "missing_token", "Payment token not provided")
		return
	}

	h.logger.Info("Received Iyzico callback", "token_prefix", truncateToken(token))

	// Confirm the topup using the token
	topup, err := h.topupService.ConfirmByToken(ctx, token)
	if err != nil {
		h.logger.Error("Failed to confirm topup from callback",
			"error", err,
			"token_prefix", truncateToken(token),
		)
		h.redirectWithError(w, r, "payment_failed", "Payment confirmation failed")
		return
	}

	h.logger.Info("Topup confirmed via callback",
		"topup_id", topup.ID,
		"organization_id", topup.OrganizationID,
		"amount_kurus", topup.AmountKurus,
		"status", topup.Status,
	)

	// Redirect to success page
	h.redirectWithSuccess(w, r, topup.ID.String(), topup.Status)
}

// HandleWebhook handles Iyzico webhook notifications.
// This is called by Iyzico for asynchronous payment status updates.
// Endpoint: POST /integrations/iyzico/webhook
func (h *IyzicoHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read Iyzico webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	signature := r.Header.Get("X-IYZ-SIGNATURE")
	if signature != "" && !h.iyzicoClient.VerifyWebhookSignature(body, signature) {
		h.logger.Warn("Invalid Iyzico webhook signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse webhook payload
	var payload iyzico.WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("Failed to parse Iyzico webhook payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	h.logger.Info("Received Iyzico webhook",
		"event_type", payload.IyziEventType,
		"payment_id", payload.PaymentID,
		"conversation_id", payload.PaymentConversationID,
		"status", payload.Status,
	)

	// Handle different event types
	switch payload.IyziEventType {
	case iyzico.WebhookEventCheckoutFormAuth:
		h.handleCheckoutFormAuth(ctx, w, &payload)
	case iyzico.WebhookEventPaymentSuccess:
		h.handlePaymentSuccess(ctx, w, &payload)
	case iyzico.WebhookEventPaymentFailure:
		h.handlePaymentFailure(ctx, w, &payload)
	case iyzico.WebhookEventRefund:
		h.handleRefund(ctx, w, &payload)
	default:
		h.logger.Warn("Unknown Iyzico webhook event type", "event_type", payload.IyziEventType)
		// Acknowledge unknown events to prevent retries
		w.WriteHeader(http.StatusOK)
	}
}

// handleCheckoutFormAuth handles checkout form authentication events.
func (h *IyzicoHandler) handleCheckoutFormAuth(ctx context.Context, w http.ResponseWriter, payload *iyzico.WebhookPayload) {
	if !payload.IsSuccess() {
		h.logger.Warn("Checkout form auth failed",
			"conversation_id", payload.PaymentConversationID,
			"status", payload.Status,
		)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Confirm the topup using the token
	topup, err := h.topupService.ConfirmByToken(ctx, payload.Token)
	if err != nil {
		h.logger.Error("Failed to confirm topup from webhook",
			"error", err,
			"conversation_id", payload.PaymentConversationID,
		)
		// Return 200 to acknowledge receipt, but log the error
		// The topup can be reconciled later
		w.WriteHeader(http.StatusOK)
		return
	}

	h.logger.Info("Topup confirmed via webhook",
		"topup_id", topup.ID,
		"organization_id", topup.OrganizationID,
		"amount_kurus", topup.AmountKurus,
	)

	w.WriteHeader(http.StatusOK)
}

// handlePaymentSuccess handles payment success events.
func (h *IyzicoHandler) handlePaymentSuccess(ctx context.Context, w http.ResponseWriter, payload *iyzico.WebhookPayload) {
	h.logger.Info("Payment success webhook received",
		"payment_id", payload.PaymentID,
		"conversation_id", payload.PaymentConversationID,
	)

	// The topup should already be confirmed via callback
	// This is a backup notification
	if payload.Token != "" {
		topup, err := h.topupService.ConfirmByToken(ctx, payload.Token)
		if err != nil {
			// May already be confirmed, which is fine
			h.logger.Debug("Topup confirmation from success webhook",
				"error", err,
				"conversation_id", payload.PaymentConversationID,
			)
		} else {
			h.logger.Info("Topup confirmed via success webhook",
				"topup_id", topup.ID,
			)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handlePaymentFailure handles payment failure events.
func (h *IyzicoHandler) handlePaymentFailure(ctx context.Context, w http.ResponseWriter, payload *iyzico.WebhookPayload) {
	h.logger.Warn("Payment failure webhook received",
		"payment_id", payload.PaymentID,
		"conversation_id", payload.PaymentConversationID,
		"status", payload.PaymentStatus,
	)

	// The topup should be marked as failed via the callback
	// This is a backup notification - no action needed
	w.WriteHeader(http.StatusOK)
}

// handleRefund handles refund events.
func (h *IyzicoHandler) handleRefund(ctx context.Context, w http.ResponseWriter, payload *iyzico.WebhookPayload) {
	h.logger.Info("Refund webhook received",
		"payment_id", payload.PaymentID,
		"conversation_id", payload.PaymentConversationID,
	)

	// Refunds are handled separately through the admin interface
	// Log for reconciliation purposes
	w.WriteHeader(http.StatusOK)
}

// redirectWithError redirects to the return URL with error parameters.
func (h *IyzicoHandler) redirectWithError(w http.ResponseWriter, r *http.Request, code, message string) {
	redirectURL := h.buildRedirectURL("error", code, message, "")
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// redirectWithSuccess redirects to the return URL with success parameters.
func (h *IyzicoHandler) redirectWithSuccess(w http.ResponseWriter, r *http.Request, topupID, status string) {
	redirectURL := h.buildRedirectURL("success", "", "", topupID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// buildRedirectURL builds the redirect URL with query parameters.
func (h *IyzicoHandler) buildRedirectURL(result, errorCode, errorMessage, topupID string) string {
	u, err := url.Parse(h.returnURL)
	if err != nil {
		// Fallback to basic URL
		return h.returnURL + "?result=" + result
	}

	q := u.Query()
	q.Set("result", result)
	if errorCode != "" {
		q.Set("error_code", errorCode)
	}
	if errorMessage != "" {
		q.Set("error_message", errorMessage)
	}
	if topupID != "" {
		q.Set("topup_id", topupID)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

// truncateToken truncates a token for logging purposes.
func truncateToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
