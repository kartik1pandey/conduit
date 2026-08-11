package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/authn"
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

func newHandlersTestServer(t *testing.T) (*httptest.Server, uuid.UUID) {
	t.Helper()
	store := setupStore(t)

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}
	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, client.Del(context.Background(), retryScheduleKey).Err())

	queue := NewRetryQueue(client)
	deliverer := NewDeliverer(2 * time.Second)
	worker := NewWorker(store, queue, deliverer, 5)
	handlers := NewHandlers(store, worker)

	mux := http.NewServeMux()
	protected := http.NewServeMux()
	handlers.Register(protected)
	mux.Handle("/", authn.RequireInternalJWT(handlerTestSecret)(protected))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, uuid.New()
}

func TestWebhookEndpointsAndEventsEndToEnd(t *testing.T) {
	srv, merchantID := newHandlersTestServer(t)
	token := signTestToken(t, merchantID)
	client := srv.Client()

	doJSON := func(method, path, body string) *http.Response {
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	// Register an endpoint.
	resp := doJSON(http.MethodPost, "/v1/webhook_endpoints", `{"url":"https://example.com/hooks"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var endpoint createEndpointResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&endpoint))
	require.Contains(t, endpoint.Secret, "whsec_")

	// List endpoints — should show the one just registered.
	listEndpointsResp := doJSON(http.MethodGet, "/v1/webhook_endpoints", "")
	defer listEndpointsResp.Body.Close()
	require.Equal(t, http.StatusOK, listEndpointsResp.StatusCode)
	var endpoints []Endpoint
	require.NoError(t, json.NewDecoder(listEndpointsResp.Body).Decode(&endpoints))
	require.Len(t, endpoints, 1)
	require.Equal(t, endpoint.ID, endpoints[0].ID)

	// Emit an event — the (unreachable) example.com endpoint means the first
	// delivery attempt will fail, but the event and delivery rows must still
	// exist immediately, which is what this test actually checks.
	eventResp := doJSON(http.MethodPost, "/v1/events", `{"type":"payment.succeeded","idempotency_key":"evt-key-1","data":{"payment_intent_id":"pi_123"}}`)
	defer eventResp.Body.Close()
	require.Equal(t, http.StatusCreated, eventResp.StatusCode)

	// List deliveries for the endpoint — should show one pending/attempted delivery.
	require.Eventually(t, func() bool {
		listResp := doJSON(http.MethodGet, "/v1/webhook_endpoints/"+endpoint.ID.String()+"/deliveries", "")
		defer listResp.Body.Close()
		if listResp.StatusCode != http.StatusOK {
			return false
		}
		var deliveries []Delivery
		_ = json.NewDecoder(listResp.Body).Decode(&deliveries)
		return len(deliveries) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestCreateEndpoint_RequiresAuth(t *testing.T) {
	srv, _ := newHandlersTestServer(t)
	resp, err := http.Post(srv.URL+"/v1/webhook_endpoints", "application/json", strings.NewReader(`{"url":"https://example.com"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
