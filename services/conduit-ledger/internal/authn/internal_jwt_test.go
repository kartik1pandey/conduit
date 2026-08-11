package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const testSecret = "test-secret"

func signToken(t *testing.T, secret, merchantID string, expiresAt time.Time) string {
	t.Helper()
	claims := internalClaims{
		MerchantID: merchantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return token
}

func TestRequireInternalJWT(t *testing.T) {
	merchantID := uuid.New()

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantBody   bool // whether the handler beneath should have been invoked
	}{
		{
			name:       "valid token is accepted",
			authHeader: "Bearer " + signToken(t, testSecret, merchantID.String(), time.Now().Add(time.Minute)),
			wantStatus: http.StatusOK,
			wantBody:   true,
		},
		{
			name:       "missing header is rejected",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed header is rejected",
			authHeader: "Basic somevalue",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong signing secret is rejected",
			authHeader: "Bearer " + signToken(t, "wrong-secret", merchantID.String(), time.Now().Add(time.Minute)),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token is rejected",
			authHeader: "Bearer " + signToken(t, testSecret, merchantID.String(), time.Now().Add(-time.Minute)),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "non-uuid merchant_id claim is rejected",
			authHeader: "Bearer " + signToken(t, testSecret, "not-a-uuid", time.Now().Add(time.Minute)),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMerchantID uuid.UUID
			var handlerCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				gotMerchantID, _ = MerchantIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			RequireInternalJWT(testSecret)(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantBody, handlerCalled)
			if tt.wantBody {
				assert.Equal(t, merchantID, gotMerchantID)
			}
		})
	}
}
