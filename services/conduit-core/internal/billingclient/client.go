// Package billingclient is conduit-core's client for conduit-billing's
// internal /v1/usage/record API — same signed-internal-JWT pattern as
// internal/ledgerclient, internal/webhooksclient, and internal/riskclient.
package billingclient

import (
	"context"
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

// RecordUsage increments merchantID's call counter for the current billing
// period. Checkpoint 4.1 requires this for every authenticated call, so it's
// called from best-effort middleware (see internal/meteringmw) — a billing
// outage must never block a merchant's actual API traffic.
func (c *Client) RecordUsage(ctx context.Context, merchantID uuid.UUID) error {
	token, err := authn.SignInternalJWT(c.jwtSecret, merchantID)
	if err != nil {
		return fmt.Errorf("signing internal token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/usage/record", nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling conduit-billing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("conduit-billing returned %d", resp.StatusCode)
	}
	return nil
}
