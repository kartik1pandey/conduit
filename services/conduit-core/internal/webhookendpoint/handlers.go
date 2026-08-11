// Package webhookendpoint exposes conduit-core's public
// /v1/webhook_endpoints surface, per docs/ARCHITECTURE.md's API contract —
// a thin proxy to conduit-webhooks, the same pattern conduit-ledger's
// balance endpoint uses ("proxied from conduit-ledger"). conduit-core never
// touches webhook_endpoints/webhook_deliveries tables directly; it owns no
// such tables — conduit-webhooks does.
package webhookendpoint

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhooksclient"
)

type Handlers struct {
	client *webhooksclient.Client
}

func NewHandlers(client *webhooksclient.Client) *Handlers {
	return &Handlers{client: client}
}

func (h *Handlers) Register(mux *http.ServeMux, requireIdempotency func(http.Handler) http.Handler) {
	mux.Handle("POST /v1/webhook_endpoints", requireIdempotency(http.HandlerFunc(h.create)))
	mux.HandleFunc("GET /v1/webhook_endpoints", h.list)
	mux.HandleFunc("GET /v1/webhook_endpoints/{id}/deliveries", h.listDeliveries)
}

type createRequest struct {
	URL string `json:"url"`
}

// create godoc
//
//	@Summary		Register a webhook endpoint
//	@Description	Registers a URL to receive signed event deliveries. The response includes a per-endpoint HMAC secret, shown once, used to sign the Conduit-Signature header on every delivery to this URL.
//	@Tags			webhook_endpoints
//	@Accept			json
//	@Produce		json
//	@Param			Idempotency-Key	header		string			true	"Idempotency key"
//	@Param			request			body		createRequest	true	"Endpoint URL"
//	@Success		201				{object}	webhooksclient.Endpoint
//	@Failure		400				{object}	map[string]string
//	@Failure		401				{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/webhook_endpoints [post]
func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	endpoint, err := h.client.CreateEndpoint(r.Context(), merchantID, req.URL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not register webhook endpoint")
		return
	}
	writeJSON(w, http.StatusCreated, endpoint)
}

// list godoc
//
//	@Summary		List webhook endpoints
//	@Description	Lists every webhook endpoint registered for the authenticated merchant.
//	@Tags			webhook_endpoints
//	@Produce		json
//	@Success		200	{array}		webhooksclient.Endpoint
//	@Failure		401	{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/webhook_endpoints [get]
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	endpoints, err := h.client.ListEndpoints(r.Context(), merchantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list webhook endpoints")
		return
	}
	writeJSON(w, http.StatusOK, endpoints)
}

// listDeliveries godoc
//
//	@Summary		List an endpoint's deliveries
//	@Description	Lists delivery attempts for one webhook endpoint, scoped to the authenticated merchant.
//	@Tags			webhook_endpoints
//	@Produce		json
//	@Param			id	path		string	true	"Webhook endpoint ID"
//	@Success		200	{array}		webhooksclient.Delivery
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Security		ApiKeyAuth
//	@Router			/v1/webhook_endpoints/{id}/deliveries [get]
func (h *Handlers) listDeliveries(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	endpointID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid endpoint id")
		return
	}

	deliveries, err := h.client.ListDeliveries(r.Context(), merchantID, endpointID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list deliveries")
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
