package merchant

import (
	"encoding/json"
	"net/http"
)

type Handlers struct {
	store *Store
}

func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

// RegisterUnauthenticated mounts the bootstrap endpoint on a mux that is NOT
// behind API-key auth (a merchant obviously can't authenticate with a key it
// doesn't have yet). Kept separate from Handlers.Register-style patterns
// used elsewhere specifically so it's never accidentally wrapped in the
// merchant-auth middleware alongside payment_intents routes.
func (h *Handlers) RegisterUnauthenticated(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/merchants", h.create)
}

type createRequest struct {
	Name string `json:"name"`
}

type createResponse struct {
	Merchant
	SecretKey string `json:"secret_key"`
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "name is required"})
		return
	}

	m, secretKey, err := h.store.Create(r.Context(), req.Name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "could not create merchant"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createResponse{Merchant: m, SecretKey: secretKey})
}
