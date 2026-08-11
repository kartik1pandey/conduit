package billing

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type UsageCounter struct {
	MerchantID uuid.UUID `json:"merchant_id"`
	Period     time.Time `json:"period"`
	CallCount  int64     `json:"call_count"`
}

type Invoice struct {
	ID          uuid.UUID       `json:"id"`
	MerchantID  uuid.UUID       `json:"merchant_id"`
	Period      time.Time       `json:"period"`
	CallCount   int64           `json:"call_count"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	CreatedAt   time.Time       `json:"created_at"`
}

// MarshalJSON renders TotalAmount with exactly 2 decimal places and Period
// as a plain date — see conduit-core's PaymentIntent.MarshalJSON for the
// identical rationale on the amount formatting.
func (i Invoice) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID          uuid.UUID `json:"id"`
		MerchantID  uuid.UUID `json:"merchant_id"`
		Period      string    `json:"period"`
		CallCount   int64     `json:"call_count"`
		TotalAmount string    `json:"total_amount"`
		CreatedAt   time.Time `json:"created_at"`
	}
	return json.Marshal(alias{
		ID:          i.ID,
		MerchantID:  i.MerchantID,
		Period:      i.Period.Format("2006-01-02"),
		CallCount:   i.CallCount,
		TotalAmount: i.TotalAmount.StringFixed(2),
		CreatedAt:   i.CreatedAt,
	})
}
