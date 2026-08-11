// Package riskclient is conduit-core's client for conduit-risk's internal
// /score API — same signed-internal-JWT pattern as internal/ledgerclient
// and internal/webhooksclient.
package riskclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

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

type ScoreResult struct {
	PaymentIntentID uuid.UUID `json:"payment_intent_id"`
	Decision        string    `json:"decision"`
	RiskScore       float64   `json:"risk_score"`
	Stage           string    `json:"stage"`
	Reasons         []string  `json:"reasons"`
}

// Score calls conduit-risk synchronously — docs/ARCHITECTURE.md: "On
// confirmation, calls conduit-risk synchronously — a decline blocks the
// charge entirely, no ledger entry is created." There's no separate
// idempotency key here the way ledger/webhooks calls have one: a Core crash
// between a risk call and the ledger call it gates causes a second scoring
// event on retry, which only means one duplicate velocity data point in
// conduit-risk's own history, never a duplicate ledger posting — the
// ledger call's own idempotency key (see paymentintent.postToLedger)
// still guarantees that regardless of how many times Score is called.
func (c *Client) Score(
	ctx context.Context, merchantID, paymentIntentID uuid.UUID, amount decimal.Decimal, currency string,
) (ScoreResult, error) {
	token, err := authn.SignInternalJWT(c.jwtSecret, merchantID)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("signing internal token: %w", err)
	}

	body, err := json.Marshal(map[string]any{
		"payment_intent_id": paymentIntentID,
		"amount":            amount.StringFixed(2),
		"currency":          currency,
	})
	if err != nil {
		return ScoreResult{}, fmt.Errorf("encoding request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/score", bytes.NewReader(body))
	if err != nil {
		return ScoreResult{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ScoreResult{}, fmt.Errorf("calling conduit-risk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return ScoreResult{}, fmt.Errorf("conduit-risk returned %d: %v", resp.StatusCode, errBody)
	}

	var result ScoreResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ScoreResult{}, fmt.Errorf("decoding conduit-risk response: %w", err)
	}
	return result, nil
}
