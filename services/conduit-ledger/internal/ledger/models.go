package ledger

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AccountType string

const (
	AccountAsset     AccountType = "asset"
	AccountLiability AccountType = "liability"
	AccountRevenue   AccountType = "revenue"
	AccountExpense   AccountType = "expense"
)

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

type Account struct {
	ID         uuid.UUID   `json:"id"`
	MerchantID uuid.UUID   `json:"merchant_id"`
	Name       string      `json:"name"`
	Type       AccountType `json:"type"`
	Currency   string      `json:"currency"`
	CreatedAt  time.Time   `json:"created_at"`
}

type Transaction struct {
	ID             uuid.UUID `json:"id"`
	MerchantID     uuid.UUID `json:"merchant_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	Entries        []Entry   `json:"entries"`
}

type Entry struct {
	ID            uuid.UUID       `json:"id"`
	TransactionID uuid.UUID       `json:"transaction_id"`
	AccountID     uuid.UUID       `json:"account_id"`
	Amount        decimal.Decimal `json:"amount"`
	Direction     Direction       `json:"direction"`
	CreatedAt     time.Time       `json:"created_at"`
}

type EntryInput struct {
	AccountID uuid.UUID       `json:"account_id"`
	Amount    decimal.Decimal `json:"amount"`
	Direction Direction       `json:"direction"`
}
