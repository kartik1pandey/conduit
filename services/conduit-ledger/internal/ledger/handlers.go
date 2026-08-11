package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/authn"
)

type Handlers struct {
	store *Store
}

func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/accounts", h.upsertAccount)
	mux.HandleFunc("POST /v1/transactions", h.postTransaction)
	mux.HandleFunc("GET /v1/accounts/{id}/balance", h.getBalance)
}

type upsertAccountRequest struct {
	Name     string      `json:"name"`
	Type     AccountType `json:"type"`
	Currency string      `json:"currency"`
}

func (h *Handlers) upsertAccount(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req upsertAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.Currency == "" || !validAccountType(req.Type) {
		writeError(w, http.StatusBadRequest, "name, currency, and a valid type are required")
		return
	}

	account, err := h.store.UpsertAccount(r.Context(), merchantID, req.Name, req.Type, req.Currency)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

type entryRequest struct {
	AccountID uuid.UUID       `json:"account_id"`
	Amount    decimal.Decimal `json:"amount"`
	Direction Direction       `json:"direction"`
}

type postTransactionRequest struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Description    string         `json:"description"`
	Entries        []entryRequest `json:"entries"`
}

func (h *Handlers) postTransaction(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req postTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "idempotency_key is required")
		return
	}
	if len(req.Entries) < 2 {
		writeError(w, http.StatusBadRequest, "a transaction needs at least one debit and one credit entry")
		return
	}

	entries := make([]EntryInput, 0, len(req.Entries))
	for _, e := range req.Entries {
		if e.Direction != Debit && e.Direction != Credit {
			writeError(w, http.StatusBadRequest, "each entry's direction must be debit or credit")
			return
		}
		if !e.Amount.IsPositive() {
			writeError(w, http.StatusBadRequest, "each entry's amount must be positive")
			return
		}
		entries = append(entries, EntryInput{AccountID: e.AccountID, Amount: e.Amount, Direction: e.Direction})
	}

	txn, err := h.store.PostTransaction(r.Context(), merchantID, req.IdempotencyKey, req.Description, entries)
	switch {
	case errors.Is(err, ErrUnbalanced):
		writeError(w, http.StatusUnprocessableEntity, "transaction entries do not net to zero")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not post transaction")
		return
	}
	writeJSON(w, http.StatusCreated, txn)
}

func (h *Handlers) getBalance(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	balance, err := h.store.Balance(r.Context(), merchantID, accountID)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "account not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not compute balance")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"account_id": accountID.String(),
		"balance":    balance.StringFixed(2),
	})
}

func validAccountType(t AccountType) bool {
	switch t {
	case AccountAsset, AccountLiability, AccountRevenue, AccountExpense:
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// HealthCheck reports whether the database is reachable, per Checkpoint 1.9.
func HealthCheck(ctx context.Context, store *Store) error {
	return store.Ping(ctx)
}
