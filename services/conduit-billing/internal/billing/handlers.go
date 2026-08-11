package billing

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/authn"
)

type Handlers struct {
	store *Store
}

func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/usage/record", h.recordUsage)
	mux.HandleFunc("GET /v1/usage/current", h.currentUsage)
	mux.HandleFunc("GET /v1/invoices/{period}", h.getInvoice)
}

func (h *Handlers) recordUsage(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	if err := h.store.RecordUsage(r.Context(), merchantID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record usage")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) currentUsage(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	usage, err := h.store.CurrentUsage(r.Context(), merchantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch usage")
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

func (h *Handlers) getInvoice(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	period, err := time.Parse("2006-01-02", r.PathValue("period"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "period must be YYYY-MM-DD (the first day of the billing month)")
		return
	}

	invoice, err := h.store.GetInvoice(r.Context(), merchantID, period)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "no invoice for that period")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not fetch invoice")
		return
	}
	writeJSON(w, http.StatusOK, invoice)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
