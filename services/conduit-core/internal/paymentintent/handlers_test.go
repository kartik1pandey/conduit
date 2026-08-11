package paymentintent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/idempotency"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ledgerclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/merchant"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/riskclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhooksclient"
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

// fakeWebhooks stands in for conduit-webhooks so confirm's best-effort event
// emission can be verified without a real webhooks service running.
func fakeWebhooks(t *testing.T, eventCalls *int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/events", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(eventCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": uuid.New()})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// fakeRiskDecision lets a test flip conduit-risk's mock verdict mid-test
// (e.g. from "allow" to "decline") without spinning up a new server.
type fakeRiskDecision struct {
	mu       sync.Mutex
	decision string
	reasons  []string
}

func newFakeRiskDecision() *fakeRiskDecision {
	return &fakeRiskDecision{decision: "allow"}
}

func (d *fakeRiskDecision) set(decision string, reasons []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.decision = decision
	d.reasons = reasons
}

func (d *fakeRiskDecision) get() (string, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.decision, d.reasons
}

// fakeRisk stands in for conduit-risk so confirm's synchronous risk call
// (Checkpoint 3.2) can be tested without a real classifier/OPA running.
func fakeRisk(t *testing.T, decision *fakeRiskDecision) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /score", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		dec, reasons := decision.get()
		if reasons == nil {
			reasons = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"payment_intent_id": req["payment_intent_id"],
			"decision":          dec,
			"risk_score":        0.1,
			"stage":             "model",
			"reasons":           reasons,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type testEnv struct {
	server           *httptest.Server
	secretKey        string
	transactionCalls int32
	eventCalls       int32
	riskDecision     *fakeRiskDecision
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("CORE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("CORE_DATABASE_URL not set; skipping integration test")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE idempotency_keys, payment_intents, merchants CASCADE")
	require.NoError(t, err)

	redisOpts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	redisClient := redis.NewClient(redisOpts)
	t.Cleanup(func() { redisClient.Close() })

	m, secretKey, err := merchant.NewStore(pool).Create(ctx, "Test Merchant")
	require.NoError(t, err)

	env := &testEnv{secretKey: secretKey, riskDecision: newFakeRiskDecision()}
	ledgerSrv := fakeLedger(t, &env.transactionCalls)
	webhooksSrv := fakeWebhooks(t, &env.eventCalls)
	riskSrv := fakeRisk(t, env.riskDecision)

	ledgerClient := ledgerclient.New(ledgerSrv.URL, testJWTSecret, 5*time.Second)
	webhooksClient := webhooksclient.New(webhooksSrv.URL, testJWTSecret, 5*time.Second)
	riskClient := riskclient.New(riskSrv.URL, testJWTSecret, 5*time.Second)
	handlers := NewHandlers(NewStore(pool), ledgerClient, riskClient, webhooksClient)
	requireIdempotency := idempotency.RequireKey(idempotency.NewStore(pool, redisClient, time.Hour))

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
		require.Eventually(t, func() bool { return atomic.LoadInt32(&env.eventCalls) == 1 }, time.Second, 10*time.Millisecond,
			"a payment.succeeded event should have been emitted")
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

// TestConfirm_RiskDeclineBlocksChargeWithNoLedgerEntry is Checkpoint 3.2's
// exact scenario: "force a high-risk input — payment_intent ends in
// failed, and no ledger entry exists for it."
func TestConfirm_RiskDeclineBlocksChargeWithNoLedgerEntry(t *testing.T) {
	env := newTestEnv(t)
	client := env.server.Client()
	env.riskDecision.set("decline", []string{"velocity_limit_exceeded"})

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

	resp := post("/v1/payment_intents", "decline-create-1", `{"amount":"99999.00","currency":"usd"}`)
	var pi PaymentIntent
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&pi))
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	confirmResp := post("/v1/payment_intents/"+pi.ID.String()+"/confirm", "decline-confirm-1", `{}`)
	defer confirmResp.Body.Close()
	var confirmed PaymentIntent
	require.NoError(t, json.NewDecoder(confirmResp.Body).Decode(&confirmed))
	require.Equal(t, http.StatusOK, confirmResp.StatusCode)
	require.Equal(t, StatusFailed, confirmed.Status)
	require.NotNil(t, confirmed.FailureReason)
	require.Contains(t, *confirmed.FailureReason, "velocity_limit_exceeded")
	require.Equal(t, int32(0), atomic.LoadInt32(&env.transactionCalls), "a declined payment must never reach the ledger")

	require.Eventually(t, func() bool { return atomic.LoadInt32(&env.eventCalls) == 1 }, time.Second, 10*time.Millisecond,
		"a payment.failed event should have been emitted")
}
