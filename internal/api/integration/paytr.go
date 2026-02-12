// Package integration provides handlers for inbound webhooks from external services.
package integration

import (
	"net/http"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/payment/paytr"
	"github.com/knid/kargonet/internal/service/billing"
)

// PayTRHandler handles PayTR payment callbacks.
type PayTRHandler struct {
	topupService *billing.TopupService
	paytrClient  *paytr.Client
	logger       *observability.Logger
}

// NewPayTRHandler creates a new PayTR handler.
func NewPayTRHandler(
	topupService *billing.TopupService,
	paytrClient *paytr.Client,
	logger *observability.Logger,
) *PayTRHandler {
	return &PayTRHandler{
		topupService: topupService,
		paytrClient:  paytrClient,
		logger:       logger,
	}
}

// HandleCallback handles the PayTR server-to-server callback notification.
// PayTR POSTs form data with merchant_oid, status, total_amount, hash.
// We must verify the hash and respond with plain text "OK".
// Endpoint: POST /integrations/paytr/callback
func (h *PayTRHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.logger.Error("Failed to parse PayTR callback form", "error", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("OK"))
		return
	}

	merchantOID := r.FormValue("merchant_oid")
	status := r.FormValue("status")
	totalAmount := r.FormValue("total_amount")
	hash := r.FormValue("hash")

	if merchantOID == "" || status == "" || hash == "" {
		h.logger.Error("PayTR callback missing required fields",
			"merchant_oid", merchantOID,
			"status", status,
		)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("OK"))
		return
	}

	h.logger.Info("Received PayTR callback",
		"merchant_oid", merchantOID,
		"status", status,
		"total_amount", totalAmount,
	)

	// Verify hash
	if !h.paytrClient.VerifyCallbackHash(merchantOID, status, totalAmount, hash) {
		h.logger.Warn("Invalid PayTR callback hash",
			"merchant_oid", merchantOID,
		)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("OK"))
		return
	}

	// Process based on status
	if status == "success" {
		topup, err := h.topupService.ConfirmByMerchantOID(ctx, merchantOID, status, totalAmount)
		if err != nil {
			h.logger.Error("Failed to confirm topup from PayTR callback",
				"error", err,
				"merchant_oid", merchantOID,
			)
		} else {
			h.logger.Info("Topup confirmed via PayTR callback",
				"topup_id", topup.ID,
				"organization_id", topup.OrganizationID,
				"amount_kurus", topup.AmountKurus,
			)
		}
	} else {
		// status == "failed"
		if err := h.topupService.FailByMerchantOID(ctx, merchantOID); err != nil {
			h.logger.Error("Failed to mark topup as failed from PayTR callback",
				"error", err,
				"merchant_oid", merchantOID,
			)
		}
	}

	// PayTR requires plain text "OK" response
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
