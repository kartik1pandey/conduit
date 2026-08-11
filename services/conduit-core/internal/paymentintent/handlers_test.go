package paymentintent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/idempotency"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ledgerclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/merchant"
	"github.com/kartik1pandey/conduit/services/conduit-core/migrations"
)

const testJWTSecret = "handler-test-internal-secret"

// fakeLedger stands in for conduit-ledger so these tests exercise the real
// HTTP wiring (real JWTs, real JSON request/response shapes) without
// needing the actual ledger service running.
func fakeLedger(t *testing.T, transactionCalls *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": uuid.New(), "name": req["name"], "type": req["type"], "currency": req["currency"],
		})
	})
	mux.HandleFunc("POST /v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(transactionCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": uuid.New(), "status": "posted"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type staticAuthenticator struct {
	secretKey  string
	merchantID uuid.UUID
}

func (s staticAuthenticator) AuthenticateBySecretKey(_ context.Context, secretKey string) (uuid.UUID, error) {
	if secretKey != s.secretKey {
		return uuid.Nil, authn.ErrInvalidAPIKey
	}
	return s.merchantID, nil
}

type testEnv struct {
	server           *httptest.Server
	secretKey        string
	transactionCalls int32
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("CORE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("CORE_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE idempotency_keys, payment_intents, merchants CASCADE")
	require.NoError(t, err)

	m, secretKey, err := merchant.NewStore(pool).Create(ctx, "Test Merchant")
	require.NoError(t, err)

	env := &testEnv{secretKey: secretKey}
	ledgerSrv := fakeLedger(t, &env.transactionCalls)

	ledgerClient := ledgerclient.New(ledgerSrv.URL, testJWTSecret, 5*time.Second)
	handlers := NewHandlers(NewStore(pool), ledgerClient)
	requireIdempotency := idempotency.RequireKey(idempotency.NewStore(pool))

	protected := http.NewServeMux()
	handlers.Register(protected, requireIdempotency)

	outer := http.NewServeMux()
	outer.Handle("/", authn.RequireAPIKey(staticAuthenticator{secretKey: secretKey, merchantID: m.ID})(protected))

	env.server = httptest.NewServer(outer)
	t.Cleanup(env.server.Close)
	return env
}

func TestPaymentIntentLifecycle(t *testing.T) {
	env := newTestEnv(t)
	client := env.server.Client()

	post := func(path, idemKey, body string) *http.Response {
		req, err := http.NewRequest(http.MethodPost, env.server.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+env.secretKey)
		req.Header.Set("Content-Type", "application/json")
		if idemKey != "" {
			req.Header.Set("Idempotency-Key", idemKey)
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	t.Run("create requires Idempotency-Key", func(t *testing.T) {
		resp := post("/v1/payment_intents", "", `{"amount":"10.00","currency":"usd"}`)
		defer resp.Body.Close()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	resp := post("/v1/payment_intents", "create-1", `{"amount":"49.99","currency":"usd","description":"widget"}`)
	var pi PaymentIntent
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pi))
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, StatusCreated, pi.Status)

	t.Run("get is visible to the owning merchant", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, env.server.URL+"/v1/payment_intents/"+pi.ID.String(), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+env.secretKey)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("confirm succeeds and posts to ledger exactly once", func(t *testing.T) {
		resp := post("/v1/payment_intents/"+pi.ID.String()+"/confirm", "confirm-1", `{}`)
		defer resp.Body.Close()
		var confirmed PaymentIntent
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&confirmed))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, StatusSucceeded, confirmed.Status)
		require.Equal(t, int32(1), atomic.LoadInt32(&env.transactionCalls))
	})

	t.Run("re-confirming an already-succeeded intent is a no-op, not a second ledger post", func(t *testing.T) {
		resp := post("/v1/payment_intents/"+pi.ID.String()+"/confirm", "confirm-2-different-key", `{}`)
		defer resp.Body.Close()
		var confirmed PaymentIntent
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&confirmed))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, StatusSucceeded, confirmed.Status)
		require.Equal(t, int32(1), atomic.LoadInt32(&env.transactionCalls), "a terminal payment intent must not trigger another ledger call")
	})
}
