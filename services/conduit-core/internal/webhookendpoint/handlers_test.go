package webhookendpoint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhooksclient"
)

const (
	testSecret    = "test-internal-secret"
	testSecretKey = "sk_test_fake"
)

// fakeWebhooksService stands in for conduit-webhooks, so this test verifies
// the proxy wiring (request/response shapes, auth passthrough) without a
// real webhooks service running.
func fakeWebhooksService(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/webhook_endpoints", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": uuid.New(), "url": "https://merchant.example/hooks", "secret": "whsec_fake", "created_at": time.Now(),
		})
	})
	mux.HandleFunc("GET /v1/webhook_endpoints/{id}/deliveries", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": uuid.New(), "status": "delivered", "attempt_count": 1},
		})
	})
	mux.HandleFunc("GET /v1/webhook_endpoints", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": uuid.New(), "url": "https://merchant.example/hooks", "created_at": time.Now()},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type staticAuthenticator struct{ merchantID uuid.UUID }

func (a staticAuthenticator) AuthenticateBySecretKey(_ context.Context, secretKey string) (uuid.UUID, error) {
	if secretKey != testSecretKey {
		return uuid.Nil, authn.ErrInvalidAPIKey
	}
	return a.merchantID, nil
}

func noopIdempotency(next http.Handler) http.Handler { return next }

func TestWebhookEndpointProxy(t *testing.T) {
	fakeSvc := fakeWebhooksService(t)
	client := webhooksclient.New(fakeSvc.URL, testSecret, 5*time.Second)
	handlers := NewHandlers(client)

	protected := http.NewServeMux()
	handlers.Register(protected, noopIdempotency)

	mux := http.NewServeMux()
	mux.Handle("/", authn.RequireAPIKey(staticAuthenticator{merchantID: uuid.New()})(protected))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/webhook_endpoints", strings.NewReader(`{"url":"https://merchant.example/hooks"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testSecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var created struct {
		ID     uuid.UUID `json:"id"`
		Secret string    `json:"secret"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	require.Equal(t, "whsec_fake", created.Secret)

	listReq, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/webhook_endpoints/"+created.ID.String()+"/deliveries", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+testSecretKey)

	listResp, err := srv.Client().Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)
}

func TestWebhookEndpointProxy_List(t *testing.T) {
	fakeSvc := fakeWebhooksService(t)
	client := webhooksclient.New(fakeSvc.URL, testSecret, 5*time.Second)
	handlers := NewHandlers(client)

	protected := http.NewServeMux()
	handlers.Register(protected, noopIdempotency)

	mux := http.NewServeMux()
	mux.Handle("/", authn.RequireAPIKey(staticAuthenticator{merchantID: uuid.New()})(protected))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/webhook_endpoints", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testSecretKey)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var endpoints []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&endpoints))
	require.Len(t, endpoints, 1)
}

func TestWebhookEndpointProxy_RequiresAuth(t *testing.T) {
	fakeSvc := fakeWebhooksService(t)
	client := webhooksclient.New(fakeSvc.URL, testSecret, 5*time.Second)
	handlers := NewHandlers(client)

	protected := http.NewServeMux()
	handlers.Register(protected, noopIdempotency)

	mux := http.NewServeMux()
	mux.Handle("/", authn.RequireAPIKey(staticAuthenticator{merchantID: uuid.New()})(protected))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/v1/webhook_endpoints", "application/json", strings.NewReader(`{"url":"https://x.example"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
