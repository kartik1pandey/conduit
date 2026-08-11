package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/authn"
)

type Handlers struct {
	store  *Store
	worker *Worker
}

func NewHandlers(store *Store, worker *Worker) *Handlers {
	return &Handlers{store: store, worker: worker}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/webhook_endpoints", h.createEndpoint)
	mux.HandleFunc("GET /v1/webhook_endpoints", h.listEndpoints)
	mux.HandleFunc("GET /v1/webhook_endpoints/{id}/deliveries", h.listDeliveries)
	mux.HandleFunc("POST /v1/events", h.createEvent)
}

type createEndpointRequest struct {
	URL string `json:"url"`
}

type createEndpointResponse struct {
	Endpoint
	Secret string `json:"secret"`
}

func (h *Handlers) createEndpoint(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req createEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	endpoint, secret, err := h.store.CreateEndpoint(r.Context(), merchantID, req.URL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create webhook endpoint")
		return
	}
	writeJSON(w, http.StatusCreated, createEndpointResponse{Endpoint: endpoint, Secret: secret})
}

// listEndpoints backs conduit-core's dashboard-facing proxy — Store.ListEndpoints
// already existed for internal use by createEvent's fan-out; this just
// exposes the same merchant-scoped read over HTTP.
func (h *Handlers) listEndpoints(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	endpoints, err := h.store.ListEndpoints(r.Context(), merchantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list webhook endpoints")
		return
	}
	writeJSON(w, http.StatusOK, endpoints)
}

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

	// Confirm the endpoint belongs to this merchant before listing anything
	// — otherwise a merchant could enumerate another merchant's delivery
	// history just by guessing endpoint IDs, even though the deliveries
	// query itself is already scoped.
	if _, err := h.store.GetEndpoint(r.Context(), merchantID, endpointID); err != nil {
		writeError(w, http.StatusNotFound, "webhook endpoint not found")
		return
	}

	deliveries, err := h.store.ListDeliveriesForEndpoint(r.Context(), merchantID, endpointID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list deliveries")
		return
	}
	writeJSON(w, http.StatusOK, deliveries)
}

type createEventRequest struct {
	Type           string          `json:"type"`
	IdempotencyKey string          `json:"idempotency_key"`
	Data           json.RawMessage `json:"data"`
}

// eventPayload is the exact shape signed and sent to merchant endpoints.
// merchant_id is deliberately omitted — it's Conduit's own internal
// identifier, not something a merchant's webhook receiver needs.
type eventPayload struct {
	ID   uuid.UUID       `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (h *Handlers) createEvent(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := authn.MerchantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing merchant context")
		return
	}

	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Type == "" || req.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "type and idempotency_key are required")
		return
	}

	eventID := uuid.New()
	payload, err := json.Marshal(eventPayload{ID: eventID, Type: req.Type, Data: req.Data})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode event payload")
		return
	}

	event, wasNew, err := h.store.CreateEvent(r.Context(), merchantID, req.Type, req.IdempotencyKey, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create event")
		return
	}

	// A replayed emission (same idempotency_key as before) must not
	// re-enqueue deliveries — they were already created and scheduled the
	// first time this idempotency_key was seen.
	if wasNew {
		endpoints, err := h.store.ListEndpoints(r.Context(), merchantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list endpoints")
			return
		}
		if len(endpoints) > 0 {
			if err := h.enqueueDeliveries(r.Context(), event, endpoints); err != nil {
				writeError(w, http.StatusInternalServerError, "could not schedule deliveries")
				return
			}
		}
	}

	writeJSON(w, http.StatusCreated, event)
}

func (h *Handlers) enqueueDeliveries(ctx context.Context, event Event, endpoints []Endpoint) error {
	endpointIDs := make([]uuid.UUID, len(endpoints))
	for i, e := range endpoints {
		endpointIDs[i] = e.ID
	}

	deliveries, err := h.store.CreateDeliveries(ctx, event.ID, endpointIDs)
	if err != nil {
		return err
	}
	for _, d := range deliveries {
		if err := h.worker.ScheduleFirstAttempt(ctx, d.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
