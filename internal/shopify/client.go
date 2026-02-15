package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is a lightweight Shopify Admin API client.
type Client struct {
	shopDomain  string
	accessToken string
	apiVersion  string
	httpClient  *http.Client

	mu            sync.Mutex
	callsUsed     int
	callsLimit    int
	lastThrottled time.Time
}

// NewClient creates a new Shopify API client for a specific store.
func NewClient(shopDomain, accessToken, apiVersion string) *Client {
	return &Client{
		shopDomain:  shopDomain,
		accessToken: accessToken,
		apiVersion:  apiVersion,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// baseURL returns the REST Admin API base URL.
func (c *Client) baseURL() string {
	return fmt.Sprintf("https://%s/admin/api/%s", c.shopDomain, c.apiVersion)
}

// graphqlURL returns the GraphQL Admin API URL.
func (c *Client) graphqlURL() string {
	return fmt.Sprintf("https://%s/admin/api/%s/graphql.json", c.shopDomain, c.apiVersion)
}

// doRequest executes an HTTP request with auth headers and rate limiting.
func (c *Client) doRequest(ctx context.Context, method, urlStr string, body io.Reader) (*http.Response, error) {
	// Check if we should back off
	c.mu.Lock()
	if c.callsLimit > 0 && c.callsUsed > 0 {
		remaining := c.callsLimit - c.callsUsed
		if remaining < 5 {
			c.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
			c.mu.Lock()
		}
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Shopify-Access-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	// Parse rate limit header: "used/limit"
	if limitHeader := resp.Header.Get("X-Shopify-Shop-Api-Call-Limit"); limitHeader != "" {
		parts := strings.Split(limitHeader, "/")
		if len(parts) == 2 {
			c.mu.Lock()
			c.callsUsed, _ = strconv.Atoi(parts[0])
			c.callsLimit, _ = strconv.Atoi(parts[1])
			c.mu.Unlock()
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		retryAfter := resp.Header.Get("Retry-After")
		wait := 2 * time.Second
		if secs, err := strconv.Atoi(retryAfter); err == nil {
			wait = time.Duration(secs) * time.Second
		}
		time.Sleep(wait)
		return c.doRequest(ctx, method, urlStr, body)
	}

	return resp, nil
}

// ExchangeCodeForToken exchanges an authorization code for an access token.
// This is a static method that doesn't require an existing access token.
func ExchangeCodeForToken(ctx context.Context, shopDomain, apiKey, apiSecret, code string) (*AccessTokenResponse, error) {
	tokenURL := fmt.Sprintf("https://%s/admin/oauth/access_token", shopDomain)

	payload := map[string]string{
		"client_id":     apiKey,
		"client_secret": apiSecret,
		"code":          code,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &tokenResp, nil
}

// GetShop fetches the shop information.
func (c *Client) GetShop(ctx context.Context) (*ShopInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, c.baseURL()+"/shop.json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("get shop failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var shopResp ShopResponse
	if err := json.NewDecoder(resp.Body).Decode(&shopResp); err != nil {
		return nil, fmt.Errorf("decoding shop response: %w", err)
	}

	return &shopResp.Shop, nil
}

// GetOrder fetches a specific order by ID.
func (c *Client) GetOrder(ctx context.Context, orderID int64) (*Order, error) {
	u := fmt.Sprintf("%s/orders/%d.json", c.baseURL(), orderID)
	resp, err := c.doRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("get order failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var orderResp struct {
		Order Order `json:"order"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, fmt.Errorf("decoding order response: %w", err)
	}

	return &orderResp.Order, nil
}

// RegisterWebhooks subscribes to webhook topics using the GraphQL API.
func (c *Client) RegisterWebhooks(ctx context.Context, topics []string, callbackURL string) error {
	for _, topic := range topics {
		mutation := fmt.Sprintf(`mutation {
			webhookSubscriptionCreate(
				topic: %s
				webhookSubscription: {
					format: JSON
					callbackUrl: "%s"
				}
			) {
				webhookSubscription { id }
				userErrors { field message }
			}
		}`, toGraphQLTopic(topic), callbackURL)

		payload := map[string]string{"query": mutation}
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshaling webhook mutation for topic %s: %w", topic, err)
		}

		resp, err := c.doRequest(ctx, http.MethodPost, c.graphqlURL(), bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("registering webhook for topic %s: %w", topic, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("webhook registration for %s returned status %d", topic, resp.StatusCode)
		}
	}
	return nil
}

// CreateFulfillment creates a fulfillment for an order via the REST API.
func (c *Client) CreateFulfillment(ctx context.Context, req *FulfillmentRequest) (*FulfillmentResponse, error) {
	// Build the fulfillment payload
	payload := map[string]interface{}{
		"fulfillment": map[string]interface{}{
			"tracking_number":  req.TrackingNumber,
			"tracking_url":     req.TrackingURL,
			"tracking_company": req.TrackingCompany,
			"notify_customer":  true,
		},
	}

	if len(req.LineItems) > 0 {
		items := make([]map[string]interface{}, len(req.LineItems))
		for i, li := range req.LineItems {
			items[i] = map[string]interface{}{"id": li.ID}
			if li.Quantity > 0 {
				items[i]["quantity"] = li.Quantity
			}
		}
		payload["fulfillment"].(map[string]interface{})["line_items"] = items
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling fulfillment: %w", err)
	}

	u := fmt.Sprintf("%s/orders/%d/fulfillments.json", c.baseURL(), req.OrderID)
	resp, err := c.doRequest(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("create fulfillment failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var fulfillResp FulfillmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&fulfillResp); err != nil {
		return nil, fmt.Errorf("decoding fulfillment response: %w", err)
	}

	return &fulfillResp, nil
}

// BuildInstallURL returns the Shopify OAuth authorization URL.
func BuildInstallURL(shopDomain, apiKey, scopes, redirectURI, nonce string) string {
	params := url.Values{
		"client_id":    {apiKey},
		"scope":        {scopes},
		"redirect_uri": {redirectURI},
		"state":        {nonce},
	}
	return fmt.Sprintf("https://%s/admin/oauth/authorize?%s", shopDomain, params.Encode())
}

// toGraphQLTopic converts a webhook topic like "orders/paid" to ORDERS_PAID.
func toGraphQLTopic(topic string) string {
	t := strings.ReplaceAll(topic, "/", "_")
	return strings.ToUpper(t)
}
