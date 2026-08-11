package paymentintent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ledgerclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/riskclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhooksclient"
)

type Handlers struct {
	store    *Store
	ledger   *ledgerclient.Client
	risk     *riskclient.Client
	webhooks *webhooksclient.Client
}

func NewHandlers(store *Store, ledger *ledgerclient.Client, risk *riskclient.Client, webhooks *webhooksclient.Client) *Handlers {
	return &Handlers{store: store, ledger: ledger, risk: risk, webhooks: webhooks}
}

// Register mounts every payment_intents route on mux. GET needs no
// Idempotency-Key; POST routes are wrapped in requireIdempotency, since
// every write endpoint requires one (CLAUDE.md non-negotiables).
func (h *Handlers) Register(mux *http.ServeMux, requireIdempotency func(http.Handler) http.Handler) {
	mux.HandleFunc("GET /v1/payment_intents", h.list)
	mux.HandleFunc("GET /v1/payment_intents/{id}", h.get)
	mux.Handle("POST /v1/payment_intents", requireIdempotency(http.HandlerFunc(h.create)))
	mux.Handle("POST /v1/payment_intents/{id}/confirm", requireIdempotency(http.HandlerFunc(h.confirm)))
	mux.Handle("POST /v1/payment_intents/{id}/refund", requireIdempotency(http.HandlerFunc(h.refund)))
}

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// list backs conduit-dashboard's transactions view. limit/offset are plain
// query params rather than an opaque cursor — this project's data volume
// (test-mode traffic from a handful of demo merchants) doesn't need
// cursor-based pagination's extra complexity to stay correct under
// concurrent writes the way, say, a real production Stripe-scale endpoint
// would.
// list godoc
//
//	@Summary		List payment intents
//	@Description	Lists the authenticated merchant's payment intents, newest first.
//	@Tags			payment_intents
//	@Produce		json
//	@Param			limit	query		int	false	"Max results (default 50, capped at 200)"
//	@Param			offset	query		int	false	"Offset for pagination (default 0)"
//	@Success		200		{array}		PaymentIntent
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/payment_intents [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	limit := defaultListLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	intents, err := h.store.List(r.Context(), merchantID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list payment intents")
		return
	}
	writeJSON(w, http.StatusOK, intents)
}

type createRequest struct {
	Amount      decimal.Decimal `json:"amount"`
	Currency    string          `json:"currency"`
	Description string          `json:"description"`
}

// create godoc
//
//	@Summary		Create a payment intent
//	@Description	Creates a new payment intent in the `created` state. Requires an Idempotency-Key — a duplicate key returns the original response instead of creating a second record.
//	@Tags			payment_intents
//	@Accept			json
//	@Produce		json
//	@Param			Idempotency-Key	header		string			true	"Idempotency key"
//	@Param			request			body		createRequest	true	"Payment intent details"
//	@Success		201				{object}	PaymentIntent
//	@Failure		400				{object}	map[string]string
//	@Failure		401				{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/payment_intents [post]
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

// get godoc
//
//	@Summary		Get a payment intent
//	@Description	Fetches a payment intent by id, scoped to the authenticated merchant. Returns 404 for another merchant's payment intent — never a 403 that would confirm it exists.
//	@Tags			payment_intents
//	@Produce		json
//	@Param			id	path		string	true	"Payment intent ID"
//	@Success		200	{object}	PaymentIntent
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		404	{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/payment_intents/{id} [get]
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

// confirm drives created|pending -> succeeded|failed: conduit-risk is called
// synchronously first, and a decline moves straight to failed with no
// ledger call at all (Checkpoint 3.2) — the balanced transaction is only
// ever posted to conduit-ledger once risk has allowed the charge. If the
// payment intent is already in a terminal state, it's returned as-is (200)
// instead of reprocessed — that, combined with the ledger call's own
// deterministic idempotency key, is what makes retrying a confirm (whether
// a client retry or conduit-core recovering from a crash mid-confirm) safe
// to do more than once.
// confirm godoc
//
//	@Summary		Confirm a payment intent
//	@Description	Drives created|pending -> succeeded|failed. Scores the charge with conduit-risk synchronously first; a decline transitions straight to failed with no ledger entry ever created. An already-terminal payment intent is returned as-is rather than reprocessed.
//	@Tags			payment_intents
//	@Produce		json
//	@Param			Idempotency-Key	header		string	true	"Idempotency key"
//	@Param			id				path		string	true	"Payment intent ID"
//	@Success		200				{object}	PaymentIntent
//	@Failure		400				{object}	map[string]string
//	@Failure		401				{object}	map[string]string
//	@Failure		404				{object}	map[string]string
//	@Failure		409				{object}	map[string]string
//	@Failure		502				{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/payment_intents/{id}/confirm [post]
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

	riskResult, err := h.risk.Score(r.Context(), merchantID, pi.ID, pi.Amount, pi.Currency)
	if err != nil {
		// A conduit-risk outage is a dependency hiccup, not a decline —
		// left in "pending" for the same reason a ledger-call failure is,
		// below: a retry (same key after its lease expires, or a fresh one)
		// tries again safely.
		writeError(w, http.StatusBadGateway, fmt.Sprintf("could not score risk: %v", err))
		return
	}
	if riskResult.Decision == "decline" {
		pi, err = h.store.TransitionToFailed(r.Context(), merchantID, id, StatusPending, strings.Join(riskResult.Reasons, ","))
		if err != nil {
			writeError(w, http.StatusConflict, "payment intent status changed concurrently, retry")
			return
		}
		h.emitPaymentFailed(r.Context(), merchantID, pi)
		writeJSON(w, http.StatusOK, pi)
		return
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

	// Best-effort: whether a merchant's webhook endpoint could be notified
	// is never a reason to fail a payment that already succeeded. A
	// conduit-webhooks outage degrades notification latency, not payments.
	h.emitPaymentSucceeded(r.Context(), merchantID, pi)

	writeJSON(w, http.StatusOK, pi)
}

// refund drives succeeded -> refunded. An already-refunded payment intent is
// returned as-is (200) rather than reprocessed — note this is deliberately
// narrower than Status.IsTerminal(), which also treats a merely-succeeded
// intent as terminal (correct for confirm, which must never reprocess a
// successful charge; wrong for refund, whose entire job is to act on a
// succeeded intent). The refund idempotency key ("core:refund:"+id) on the
// ledger call is what actually makes a retried refund request safe; this
// early-return check just avoids a wasted ledger round trip on the common
// case of a client retrying a request that already refunded. Only a payment
// in StatusSucceeded can be refunded — never StatusCreated or StatusPending
// (there is no charge on the ledger yet to reverse) and never StatusFailed
// (there was never a charge at all, per Checkpoint 3.2).
// refund godoc
//
//	@Summary		Refund a payment intent
//	@Description	Drives succeeded -> refunded, posting a second balanced ledger transaction with entries reversed relative to the original charge. Only a succeeded payment intent can be refunded.
//	@Tags			payment_intents
//	@Produce		json
//	@Param			Idempotency-Key	header		string	true	"Idempotency key"
//	@Param			id				path		string	true	"Payment intent ID"
//	@Success		200				{object}	PaymentIntent
//	@Failure		400				{object}	map[string]string
//	@Failure		401				{object}	map[string]string
//	@Failure		404				{object}	map[string]string
//	@Failure		409				{object}	map[string]string
//	@Failure		502				{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/payment_intents/{id}/refund [post]
func (h *Handlers) refund(w http.ResponseWriter, r *http.Request) {
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

	if pi.Status == StatusRefunded {
		writeJSON(w, http.StatusOK, pi)
		return
	}

	if pi.Status != StatusSucceeded {
		writeError(w, http.StatusConflict, "only a succeeded payment intent can be refunded")
		return
	}

	if err := h.postRefundToLedger(r.Context(), merchantID, pi); err != nil {
		// Same treatment as confirm's ledger call: a transient failure here
		// isn't a decision, it's a dependency hiccup. The payment intent
		// stays succeeded so a retry (same Idempotency-Key, or a fresh one)
		// can safely try the refund again.
		writeError(w, http.StatusBadGateway, fmt.Sprintf("could not post refund to ledger: %v", err))
		return
	}

	pi, err = h.store.TransitionStatus(r.Context(), merchantID, id, StatusSucceeded, StatusRefunded)
	if err != nil {
		writeError(w, http.StatusConflict, "payment intent status changed concurrently, retry")
		return
	}

	h.emitPaymentRefunded(r.Context(), merchantID, pi)

	writeJSON(w, http.StatusOK, pi)
}

func (h *Handlers) emitPaymentSucceeded(ctx context.Context, merchantID uuid.UUID, pi PaymentIntent) {
	idempotencyKey := "confirm:" + pi.ID.String() + ":succeeded"
	data := map[string]any{
		"payment_intent_id": pi.ID,
		"amount":            pi.Amount.StringFixed(2),
		"currency":          pi.Currency,
		"status":            pi.Status,
	}
	if err := h.webhooks.Emit(ctx, merchantID, "payment.succeeded", idempotencyKey, data); err != nil {
		log.Printf("paymentintent: could not emit payment.succeeded for %s: %v", pi.ID, err)
	}
}

func (h *Handlers) emitPaymentFailed(ctx context.Context, merchantID uuid.UUID, pi PaymentIntent) {
	idempotencyKey := "confirm:" + pi.ID.String() + ":failed"
	data := map[string]any{
		"payment_intent_id": pi.ID,
		"amount":            pi.Amount.StringFixed(2),
		"currency":          pi.Currency,
		"status":            pi.Status,
		"failure_reason":    pi.FailureReason,
	}
	if err := h.webhooks.Emit(ctx, merchantID, "payment.failed", idempotencyKey, data); err != nil {
		log.Printf("paymentintent: could not emit payment.failed for %s: %v", pi.ID, err)
	}
}

func (h *Handlers) emitPaymentRefunded(ctx context.Context, merchantID uuid.UUID, pi PaymentIntent) {
	idempotencyKey := "refund:" + pi.ID.String() + ":refunded"
	data := map[string]any{
		"payment_intent_id": pi.ID,
		"amount":            pi.Amount.StringFixed(2),
		"currency":          pi.Currency,
		"status":            pi.Status,
	}
	if err := h.webhooks.Emit(ctx, merchantID, "payment.refunded", idempotencyKey, data); err != nil {
		log.Printf("paymentintent: could not emit payment.refunded for %s: %v", pi.ID, err)
	}
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

// postRefundToLedger posts a second, independent balanced transaction with
// entries reversed relative to the original charge — never mutates or
// deletes the original posting. Ledger entries are append-only
// (docs/ARCHITECTURE.md), so "reversing a charge" means recording a new
// transaction that offsets it, exactly how real double-entry bookkeeping
// handles a reversal; the running balance nets to the correct post-refund
// amount without ever rewriting history.
func (h *Handlers) postRefundToLedger(ctx context.Context, merchantID uuid.UUID, pi PaymentIntent) error {
	cash, err := h.ledger.EnsureAccount(ctx, merchantID, "cash", "asset", pi.Currency)
	if err != nil {
		return fmt.Errorf("ensuring cash account: %w", err)
	}
	revenue, err := h.ledger.EnsureAccount(ctx, merchantID, "payments_revenue", "revenue", pi.Currency)
	if err != nil {
		return fmt.Errorf("ensuring revenue account: %w", err)
	}

	idempotencyKey := "core:refund:" + pi.ID.String()
	_, err = h.ledger.PostTransaction(ctx, merchantID, idempotencyKey, "payment_intent "+pi.ID.String()+" refunded", []ledgerclient.Entry{
		{AccountID: revenue.ID, Amount: pi.Amount, Direction: "debit"},
		{AccountID: cash.ID, Amount: pi.Amount, Direction: "credit"},
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
