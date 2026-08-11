package merchant

import (
	"encoding/json"
	"errors"
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
	mux.HandleFunc("POST /v1/merchants/verify-secret", h.verifySecret)
}

type createRequest struct {
	Name string `json:"name"`
}

type createResponse struct {
	Merchant
	SecretKey string `json:"secret_key"`
}

// create godoc
//
//	@Summary		Create a merchant
//	@Description	Bootstraps a new test-mode merchant with an sk_test_/pk_test_ key pair. The secret key is returned exactly once and never retrievable again — only its hash is persisted.
//	@Tags			merchants
//	@Accept			json
//	@Produce		json
//	@Param			request	body		createRequest	true	"Merchant name"
//	@Success		201		{object}	createResponse
//	@Failure		400		{object}	map[string]string
//	@Router			/v1/merchants [post]
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

type verifySecretRequest struct {
	SecretKey string `json:"secret_key"`
}

type verifySecretResponse struct {
	MerchantID string `json:"merchant_id"`
	Name       string `json:"name"`
}

// verifySecret lets conduit-dashboard's signup flow prove a merchant holds
// a given sk_test_... key exactly once, without conduit-core ever handing
// back the merchant_id to anyone who doesn't already have the key — the
// dashboard never stores this raw secret key afterward (see
// authn.RequireMerchantContext's package doc), only the merchant_id and the
// dashboard user's own credentials. Unauthenticated for the same reason
// Create is: proving you hold the key IS the authentication.
// verifySecret godoc
//
//	@Summary		Verify a merchant secret key
//	@Description	Proves ownership of a merchant's sk_test_... key once and resolves it to a merchant_id. Used by conduit-dashboard's signup flow — the raw key is never stored afterward.
//	@Tags			merchants
//	@Accept			json
//	@Produce		json
//	@Param			request	body		verifySecretRequest	true	"Secret key to verify"
//	@Success		200		{object}	verifySecretResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/merchants/verify-secret [post]
func (h *Handlers) verifySecret(w http.ResponseWriter, r *http.Request) {
	var req verifySecretRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SecretKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "secret_key is required"})
		return
	}

	merchantID, err := h.store.AuthenticateBySecretKey(r.Context(), req.SecretKey)
	switch {
	case errors.Is(err, ErrNotFound):
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid secret key"})
		return
	case err != nil:
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "could not verify secret key"})
		return
	}

	m, err := h.store.Get(r.Context(), merchantID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "could not fetch merchant"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(verifySecretResponse{MerchantID: m.ID.String(), Name: m.Name})
}
