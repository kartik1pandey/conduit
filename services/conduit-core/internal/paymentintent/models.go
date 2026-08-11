package paymentintent

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Status string

const (
	StatusCreated   Status = "created"
	StatusPending   Status = "pending"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusRefunded  Status = "refunded"
)

type PaymentIntent struct {
	ID            uuid.UUID       `json:"id"`
	MerchantID    uuid.UUID       `json:"merchant_id"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Status        Status          `json:"status"`
	Description   string          `json:"description"`
	FailureReason *string         `json:"failure_reason,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// IsTerminal reports whether the payment intent has reached a final state
// that Confirm should treat as an idempotent no-op rather than reprocessing.
func (s Status) IsTerminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusRefunded
}

// MarshalJSON renders Amount with exactly 2 decimal places. decimal.Decimal's
// default String() normalizes away trailing zeros (25.00 -> "25"), which is
// fine numerically — no precision is lost — but wrong for a money API: a
// client should always see cents. Hardcoding 2 places is a phase-1
// simplification; a zero-decimal currency like JPY would need this to be
// currency-aware.
func (p PaymentIntent) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID            uuid.UUID `json:"id"`
		MerchantID    uuid.UUID `json:"merchant_id"`
		Amount        string    `json:"amount"`
		Currency      string    `json:"currency"`
		Status        Status    `json:"status"`
		Description   string    `json:"description"`
		FailureReason *string   `json:"failure_reason,omitempty"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
	}
	return json.Marshal(alias{
		ID:            p.ID,
		MerchantID:    p.MerchantID,
		Amount:        p.Amount.StringFixed(2),
		Currency:      p.Currency,
		Status:        p.Status,
		Description:   p.Description,
		FailureReason: p.FailureReason,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	})
}
