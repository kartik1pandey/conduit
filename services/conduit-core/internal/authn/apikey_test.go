package authn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type fakeAuthenticator struct {
	validKey   string
	merchantID uuid.UUID
}

func (f fakeAuthenticator) AuthenticateBySecretKey(_ context.Context, secretKey string) (uuid.UUID, error) {
	if secretKey != f.validKey {
		return uuid.Nil, ErrInvalidAPIKey
	}
	return f.merchantID, nil
}

func TestRequireAPIKey(t *testing.T) {
	merchantID := uuid.New()
	authenticator := fakeAuthenticator{validKey: "sk_test_valid", merchantID: merchantID}

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "valid key resolves merchant context",
			authHeader: "Bearer sk_test_valid",
			wantStatus: http.StatusOK,
			wantCalled: true,
		},
		{
			name:       "missing header is rejected",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong key is rejected",
			authHeader: "Bearer sk_test_wrong",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var gotMerchantID uuid.UUID
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotMerchantID, _ = MerchantIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			RequireAPIKey(authenticator)(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantCalled, called)
			if tt.wantCalled {
				assert.Equal(t, merchantID, gotMerchantID)
			}
		})
	}
}
