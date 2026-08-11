// Package ledgerclient is conduit-core's client for conduit-ledger's
// internal API. Every request signs a fresh internal JWT (see
// internal/authn.SignInternalJWT) — no service reads another service's
// database directly, and "it's a private Docker network" isn't treated as
// authentication on its own (docs/ARCHITECTURE.md).
package ledgerclient

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
	return &Client{
		baseURL:    baseURL,
		jwtSecret:  jwtSecret,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type Account struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Currency string    `json:"currency"`
}

// EnsureAccount creates the named account for merchantID, or returns the
// existing one — conduit-ledger's POST /v1/accounts is an upsert keyed on
// (merchant_id, name), so calling this on every confirm is safe and cheap.
func (c *Client) EnsureAccount(ctx context.Context, merchantID uuid.UUID, name, accountType, currency string) (Account, error) {
	var account Account
	err := c.do(ctx, merchantID, http.MethodPost, "/v1/accounts", map[string]string{
		"name": name, "type": accountType, "currency": currency,
	}, &account)
	return account, err
}

type Entry struct {
	AccountID uuid.UUID       `json:"account_id"`
	Amount    decimal.Decimal `json:"amount"`
	Direction string          `json:"direction"`
}

type Transaction struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

// PostTransaction posts a balanced transaction. idempotencyKey should be
// derived deterministically from the payment_intent id (not a fresh UUID
// per call), so a retried confirm — whether from a client retry or from
// conduit-core recovering after a crash mid-confirm — can never double-post
// to the ledger.
func (c *Client) PostTransaction(ctx context.Context, merchantID uuid.UUID, idempotencyKey, description string, entries []Entry) (Transaction, error) {
	var txn Transaction
	err := c.do(ctx, merchantID, http.MethodPost, "/v1/transactions", map[string]any{
		"idempotency_key": idempotencyKey,
		"description":     description,
		"entries":         entries,
	}, &txn)
	return txn, err
}

func (c *Client) do(ctx context.Context, merchantID uuid.UUID, method, path string, body, out any) error {
	token, err := authn.SignInternalJWT(c.jwtSecret, merchantID)
	if err != nil {
		return fmt.Errorf("signing internal token: %w", err)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling conduit-ledger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return fmt.Errorf("conduit-ledger returned %d: %s", resp.StatusCode, errBody["error"])
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding conduit-ledger response: %w", err)
		}
	}
	return nil
}
