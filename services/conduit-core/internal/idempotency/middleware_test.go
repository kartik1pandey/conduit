package idempotency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
)

func withMerchantContext(req *http.Request, merchantID uuid.UUID) *http.Request {
	// authn.MerchantIDFromContext reads a package-private context key, so
	// the only way to set it from another package is through the real
	// RequireAPIKey middleware — reuse it here with a trivial always-succeed
	// authenticator instead of re-implementing the context wiring.
	authenticator := alwaysAuthenticates{merchantID: merchantID}
	var out *http.Request
	authn.RequireAPIKey(authenticator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out = r
	})).ServeHTTP(httptest.NewRecorder(), req)
	return out
}

type alwaysAuthenticates struct{ merchantID uuid.UUID }

func (a alwaysAuthenticates) AuthenticateBySecretKey(context.Context, string) (uuid.UUID, error) {
	return a.merchantID, nil
}

func TestRequireKey_MissingHeaderRejected(t *testing.T) {
	store, _, merchantID := setupStore(t)

	var called int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/payment_intents", nil)
	req.Header.Set("Authorization", "Bearer sk_test_x")
	req = withMerchantContext(req, merchantID)

	rec := httptest.NewRecorder()
	RequireKey(store)(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, atomic.LoadInt32(&called))
}

func TestRequireKey_SecondRequestReplaysFirstResponseWithoutRerunningHandler(t *testing.T) {
	store, _, merchantID := setupStore(t)

	var callCount int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"pi_123"}`))
	})
	handler := RequireKey(store)(next)

	makeRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/payment_intents", strings.NewReader(`{"amount":"10.00"}`))
		req.Header.Set("Authorization", "Bearer sk_test_x")
		req.Header.Set("Idempotency-Key", "same-key")
		req = withMerchantContext(req, merchantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	first := makeRequest()
	second := makeRequest()

	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, first.Code, second.Code)
	require.Equal(t, first.Body.String(), second.Body.String())
	require.Equal(t, int32(1), atomic.LoadInt32(&callCount), "handler must run exactly once, not once per request")
}

func TestRequireKey_SameKeyDifferentBodyIsRejected(t *testing.T) {
	store, _, merchantID := setupStore(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := RequireKey(store)(next)

	req1 := httptest.NewRequest(http.MethodPost, "/v1/payment_intents", strings.NewReader(`{"amount":"10.00"}`))
	req1.Header.Set("Authorization", "Bearer sk_test_x")
	req1.Header.Set("Idempotency-Key", "same-key")
	req1 = withMerchantContext(req1, merchantID)
	handler.ServeHTTP(httptest.NewRecorder(), req1)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/payment_intents", strings.NewReader(`{"amount":"99.00"}`))
	req2.Header.Set("Authorization", "Bearer sk_test_x")
	req2.Header.Set("Idempotency-Key", "same-key")
	req2 = withMerchantContext(req2, merchantID)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusUnprocessableEntity, rec2.Code)
}
