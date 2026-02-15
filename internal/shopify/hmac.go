package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// VerifyWebhookHMAC verifies a Shopify webhook signature.
// Shopify signs webhooks with HMAC-SHA256 and sends the signature
// as a base64-encoded string in the X-Shopify-Hmac-SHA256 header.
func VerifyWebhookHMAC(body []byte, hmacHeader string, secret string) bool {
	if hmacHeader == "" || secret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(hmacHeader))
}
