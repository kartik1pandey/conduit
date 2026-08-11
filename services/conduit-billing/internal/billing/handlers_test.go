package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/authn"
)

const handlerTestSecret = "handler-test-internal-secret"

type internalClaims struct {
	MerchantID string `json:"merchant_id"`
	jwt.RegisteredClaims
}

func signTestToken(t *testing.T, merchantID uuid.UUID) string {
	t.Helper()
	claims := internalClaims{
		MerchantID:       merchantID.String(),
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(handlerTestSecret))
	require.NoError(t, err)
	return token
}

func newTestServer(t *testing.T) (*httptest.Server, uuid.UUID) {
	t.Helper()
	store := setupStore(t)
	handlers := NewHandlers(store)

	mux := http.NewServeMux()
	protected := http.NewServeMux()
	handlers.Register(protected)
	mux.Handle("/", authn.RequireInternalJWT(handlerTestSecret)(protected))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, uuid.New()
}

func TestUsageEndpointsEndToEnd(t *testing.T) {
	srv, merchantID := newTestServer(t)
	token := signTestToken(t, merchantID)
	client := srv.Client()

	doReq := func(method, path string) *http.Response {
		req, err := http.NewRequest(method, srv.URL+path, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	const n = 5
	for i := 0; i < n; i++ {
		resp := doReq(http.MethodPost, "/v1/usage/record")
		require.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()
	}

	resp := doReq(http.MethodGet, "/v1/usage/current")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var usage UsageCounter
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&usage))
	require.Equal(t, int64(n), usage.CallCount)
}

func TestRecordUsage_RequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Post(srv.URL+"/v1/usage/record", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
