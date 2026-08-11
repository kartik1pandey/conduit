// Package webhooksclient is conduit-core's client for conduit-webhooks'
// internal API — signs a fresh internal JWT per call, same pattern as
// internal/ledgerclient.
package webhooksclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
)

type Client struct {
	baseURL    string
	jwtSecret  string
	httpClient *http.Client
}

func New(baseURL, jwtSecret string, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, jwtSecret: jwtSecret, httpClient: &http.Client{Timeout: timeout}}
}

type Endpoint struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *Client) CreateEndpoint(ctx context.Context, merchantID uuid.UUID, url string) (Endpoint, error) {
	var endpoint Endpoint
	err := c.do(ctx, merchantID, http.MethodPost, "/v1/webhook_endpoints", map[string]string{"url": url}, &endpoint)
	return endpoint, err
}

type Delivery struct {
	ID                 uuid.UUID `json:"id"`
	Status             string    `json:"status"`
	AttemptCount       int       `json:"attempt_count"`
	LastResponseStatus *int      `json:"last_response_status,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

func (c *Client) ListDeliveries(ctx context.Context, merchantID, endpointID uuid.UUID) ([]Delivery, error) {
	var deliveries []Delivery
	err := c.do(ctx, merchantID, http.MethodGet, "/v1/webhook_endpoints/"+endpointID.String()+"/deliveries", nil, &deliveries)
	return deliveries, err
}

// Emit asks conduit-webhooks to deliver eventType to every endpoint
// registered for merchantID. idempotencyKey should be derived
// deterministically from the triggering action (e.g.
// "confirm:"+paymentIntentID+":succeeded"), the same pattern
// internal/ledgerclient uses, so a retried confirm can never double-emit.
func (c *Client) Emit(ctx context.Context, merchantID uuid.UUID, eventType, idempotencyKey string, data any) error {
	return c.do(ctx, merchantID, http.MethodPost, "/v1/events", map[string]any{
		"type":            eventType,
		"idempotency_key": idempotencyKey,
		"data":            data,
	}, nil)
}

func (c *Client) do(ctx context.Context, merchantID uuid.UUID, method, path string, body, out any) error {
	token, err := authn.SignInternalJWT(c.jwtSecret, merchantID)
	if err != nil {
		return fmt.Errorf("signing internal token: %w", err)
	}

	var reqBody *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling conduit-webhooks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("conduit-webhooks returned %d: %s", resp.StatusCode, errBody["error"])
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding conduit-webhooks response: %w", err)
		}
	}
	return nil
}
