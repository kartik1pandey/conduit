package paymentintent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ledgerclient"
)

type Handlers struct {
	store  *Store
	ledger *ledgerclient.Client
}

func NewHandlers(store *Store, ledger *ledgerclient.Client) *Handlers {
	return &Handlers{store: store, ledger: ledger}
}

// Register mounts every payment_intents route on mux. GET needs no
// Idempotency-Key; POST routes are wrapped in requireIdempotency, since
// every write endpoint requires one (CLAUDE.md non-negotiables).
func (h *Handlers) Register(mux *http.ServeMux, requireIdempotency func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /v1/payment_intents/{id}", h.get)
	mux.Handle("POST /v1/payment_intents", requireIdempotency(http.HandlerFunc(h.create)))
	mux.Handle("POST /v1/payment_intents/{id}/confirm", requireIdempotency(http.HandlerFunc(h.confirm)))
}

type createRequest struct {
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	Description string          `json:"description"`
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !req.Amount.IsPositive() {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if req.Currency == "" {
		writeError(w, http.StatusBadRequest, "currency is required")
		return
	}

	pi, err := h.store.Create(r.Context(), merchantID, req.Amount, req.Currency, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create payment intent")
		return
	}
	writeJSON(w, http.StatusCreated, pi)
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment intent id")
		return
	}

	pi, err := h.store.Get(r.Context(), merchantID, id)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "payment intent not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not fetch payment intent")
		return
	}
	writeJSON(w, http.StatusOK, pi)
}

// confirm drives created|pending -> succeeded|failed by posting a balanced
// transaction to conduit-ledger. If the payment intent is already in a
// terminal state, it's returned as-is (200) instead of reprocessed — that,
// combined with the ledger call's own deterministic idempotency key, is what
// makes retrying a confirm (whether a client retry or conduit-core recovering
// from a crash mid-confirm) safe to do more than once.
//
// Risk scoring (docs/ARCHITECTURE.md: "a decline blocks the charge entirely")
// isn't wired in yet — that's Checkpoint 3.2, once conduit-risk exists.
func (h *Handlers) confirm(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid payment intent id")
		return
	}

	pi, err := h.store.Get(r.Context(), merchantID, id)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "payment intent not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not fetch payment intent")
		return
	}

	if pi.Status.IsTerminal() {
		writeJSON(w, http.StatusOK, pi)
		return
	}

	if pi.Status == StatusCreated {
		pi, err = h.store.TransitionStatus(r.Context(), merchantID, id, StatusCreated, StatusPending)
		if err != nil {
			writeError(w, http.StatusConflict, "payment intent status changed concurrently, retry")
			return
		}
	}

	if err := h.postToLedger(r.Context(), merchantID, pi); err != nil {
		// Deliberately left in "pending": a transient failure calling
		// conduit-ledger isn't a risk decline, it's a dependency hiccup. The
		// next confirm attempt (same Idempotency-Key after its lease
		// expires, or a fresh one) will retry the ledger call safely.
		writeError(w, http.StatusBadGateway, fmt.Sprintf("could not post to ledger: %v", err))
		return
	}

	pi, err = h.store.TransitionStatus(r.Context(), merchantID, id, StatusPending, StatusSucceeded)
	if err != nil {
		writeError(w, http.StatusConflict, "payment intent status changed concurrently, retry")
		return
	}
	writeJSON(w, http.StatusOK, pi)
}

func (h *Handlers) postToLedger(ctx context.Context, merchantID uuid.UUID, pi PaymentIntent) error {
	cash, err := h.ledger.EnsureAccount(ctx, merchantID, "cash", "asset", pi.Currency)
	if err != nil {
		return fmt.Errorf("ensuring cash account: %w", err)
	}
	revenue, err := h.ledger.EnsureAccount(ctx, merchantID, "payments_revenue", "revenue", pi.Currency)
	if err != nil {
		return fmt.Errorf("ensuring revenue account: %w", err)
	}

	idempotencyKey := "core:confirm:" + pi.ID.String()
	_, err = h.ledger.PostTransaction(ctx, merchantID, idempotencyKey, "payment_intent "+pi.ID.String()+" confirmed", []ledgerclient.Entry{
		{AccountID: cash.ID, Amount: pi.Amount, Direction: "debit"},
		{AccountID: revenue.ID, Amount: pi.Amount, Direction: "credit"},
	})
	return err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
