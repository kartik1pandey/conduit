package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireMerchantContext(t *testing.T) {
	const dashboardSecret = "test-dashboard-secret"
	apiKeyMerchantID := uuid.New()
	dashboardMerchantID := uuid.New()
	authenticator := fakeAuthenticator{validKey: "sk_test_valid", merchantID: apiKeyMerchantID}

	validSession, err := SignDashboardSession(dashboardSecret, dashboardMerchantID)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		dashboardToken string
		wantStatus     int
		wantMerchantID uuid.UUID
	}{
		{
			name:           "API key takes the API-key path",
			authHeader:     "Bearer sk_test_valid",
			wantStatus:     http.StatusOK,
			wantMerchantID: apiKeyMerchantID,
		},
		{
			name:           "valid dashboard session resolves merchant context",
			dashboardToken: validSession,
			wantStatus:     http.StatusOK,
			wantMerchantID: dashboardMerchantID,
		},
		{
			name:           "invalid dashboard session is rejected",
			dashboardToken: "not-a-real-jwt",
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:       "wrong API key is rejected, not silently falling through",
			authHeader: "Bearer sk_test_wrong",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "neither credential present is rejected",
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
			if tt.dashboardToken != "" {
				req.Header.Set("X-Dashboard-Session", tt.dashboardToken)
			}
			rec := httptest.NewRecorder()

			RequireMerchantContext(authenticator, dashboardSecret)(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusOK {
				assert.True(t, called)
				assert.Equal(t, tt.wantMerchantID, gotMerchantID)
			}
		})
	}
}

// TestSignAndVerifyDashboardSession_WrongSecretRejected guards the actual
// security property this token exists for: a session signed with one
// secret must never verify against a different one — otherwise the
// dashboard-vs-API-key trust boundary this file's package doc talks about
// wouldn't be a boundary at all.
func TestSignAndVerifyDashboardSession_WrongSecretRejected(t *testing.T) {
	token, err := SignDashboardSession("secret-a", uuid.New())
	require.NoError(t, err)

	_, err = verifyDashboardSession("secret-b", token)
	require.Error(t, err)
}
