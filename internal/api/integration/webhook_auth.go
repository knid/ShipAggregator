package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
)

// VerifyWebhookSignature verifies an HMAC-SHA256 webhook signature.
// If secret is empty (not configured), it logs a warning and allows the request
// for backward compatibility in development environments.
func VerifyWebhookSignature(body []byte, signatureHeader, secret string) bool {
	if secret == "" {
		slog.Warn("webhook secret not configured, skipping signature verification")
		return true
	}

	if signatureHeader == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(signatureHeader))
}
