// Package riskdecision exposes conduit-core's public read-only
// /v1/risk_decisions surface — a thin proxy to conduit-risk, the same
// pattern conduit-ledger's balance endpoint and webhookendpoint's list
// endpoint use. conduit-core owns no risk data itself; conduit-risk does.
package riskdecision

import (
	"encoding/json"
	"net/http"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/riskclient"
)

type Handlers struct {
	client *riskclient.Client
}

func NewHandlers(client *riskclient.Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/risk_decisions", h.list)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	decisions, err := h.client.ListDecisions(r.Context(), merchantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list risk decisions")
		return
	}
	writeJSON(w, http.StatusOK, decisions)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
