package merchant

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := setupStore(t)
	handlers := NewHandlers(store)
	mux := http.NewServeMux()
	handlers.RegisterUnauthenticated(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestVerifySecret is conduit-dashboard's signup flow, exactly: prove you
// hold a merchant's secret key once, get its merchant_id back, never the
// key again.
func TestVerifySecret(t *testing.T) {
	srv := newTestServer(t)
	client := srv.Client()

	createResp, err := client.Post(srv.URL+"/v1/merchants", "application/json", bytes.NewReader([]byte(`{"name":"Acme Corp"}`)))
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var created createResponse
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&created))

	t.Run("valid secret key resolves the merchant", func(t *testing.T) {
		body, _ := json.Marshal(verifySecretRequest{SecretKey: created.SecretKey})
		resp, err := client.Post(srv.URL+"/v1/merchants/verify-secret", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var got verifySecretResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.Equal(t, created.ID.String(), got.MerchantID)
		require.Equal(t, "Acme Corp", got.Name)
	})

	t.Run("wrong secret key is rejected", func(t *testing.T) {
		body, _ := json.Marshal(verifySecretRequest{SecretKey: "sk_test_does_not_exist"})
		resp, err := client.Post(srv.URL+"/v1/merchants/verify-secret", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing secret key is a bad request, not a lookup", func(t *testing.T) {
		resp, err := client.Post(srv.URL+"/v1/merchants/verify-secret", "application/json", bytes.NewReader([]byte(`{}`)))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
