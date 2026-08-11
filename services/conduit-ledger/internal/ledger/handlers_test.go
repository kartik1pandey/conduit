package ledger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/authn"
)

const handlerTestSecret = "handler-test-secret"

type internalClaims struct {
	MerchantID string `json:"merchant_id"`
	jwt.RegisteredClaims
}

func signTestToken(t *testing.T, merchantID uuid.UUID) string {
	t.Helper()
	claims := internalClaims{
		MerchantID: merchantID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
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

func TestPostTransactionEndpoint(t *testing.T) {
	srv, merchantID := newTestServer(t)
	token := signTestToken(t, merchantID)
	client := srv.Client()

	doJSON := func(method, path string, body any) *http.Response {
		var reader *bytes.Reader
		if body != nil {
			b, err := json.Marshal(body)
			require.NoError(t, err)
			reader = bytes.NewReader(b)
		} else {
			reader = bytes.NewReader(nil)
		}
		req, err := http.NewRequest(method, srv.URL+path, reader)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	createAccount := func(name, accountType string) uuid.UUID {
		resp := doJSON(http.MethodPost, "/v1/accounts", map[string]string{
			"name": name, "type": accountType, "currency": "usd",
		})
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var account Account
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&account))
		return account.ID
	}

	cashID := createAccount("cash", "asset")
	revenueID := createAccount("revenue", "revenue")

	t.Run("balanced transaction succeeds", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/v1/transactions", map[string]any{
			"idempotency_key": "http-balanced",
			"description":     "balanced via HTTP",
			"entries": []map[string]any{
				{"account_id": cashID, "amount": "75.00", "direction": "debit"},
				{"account_id": revenueID, "amount": "75.00", "direction": "credit"},
			},
		})
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("unbalanced transaction is rejected, not partially written", func(t *testing.T) {
		resp := doJSON(http.MethodPost, "/v1/transactions", map[string]any{
			"idempotency_key": "http-unbalanced",
			"description":     "unbalanced via HTTP",
			"entries": []map[string]any{
				{"account_id": cashID, "amount": "75.00", "direction": "debit"},
				{"account_id": revenueID, "amount": "50.00", "direction": "credit"},
			},
		})
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("missing bearer token is rejected", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/accounts/"+cashID.String()+"/balance", nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
