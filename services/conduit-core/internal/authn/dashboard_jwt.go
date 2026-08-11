// dashboard_jwt.go adds a second, independent trust boundary alongside the
// merchant API key in apikey.go: conduit-dashboard signs a short-lived JWT
// (DASHBOARD_JWT_SECRET, distinct from every other shared secret in this
// project) carrying the logged-in user's merchant_id, sent as
// X-Dashboard-Session rather than Authorization so it can never be confused
// with a real sk_test_... key on the wire. This mirrors why Stripe's own
// dashboard sessions and API keys are separate credentials rather than the
// dashboard just holding a merchant's raw secret key: conduit-core only
// ever stores a hash of a merchant's secret key (see merchant.Store), so
// there's no raw key for the dashboard to retain past the one-time signup
// flow (merchant.Handlers' verify-secret endpoint) even if it wanted to.
package authn

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type dashboardClaims struct {
	MerchantID string `json:"merchant_id"`
	jwt.RegisteredClaims
}

// SignDashboardSession issues a 60-second token — minted fresh by
// conduit-dashboard's backend for each request it forwards to core, never
// cached and reused, so a leaked token is only useful for a minute. Same
// lifetime and same reasoning as SignInternalJWT.
func SignDashboardSession(secret string, merchantID uuid.UUID) (string, error) {
	claims := dashboardClaims{
		MerchantID: merchantID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func verifyDashboardSession(secret, tokenString string) (uuid.UUID, error) {
	claims := &dashboardClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return uuid.Nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, errors.New("invalid dashboard session token")
	}
	return uuid.Parse(claims.MerchantID)
}

// RequireMerchantContext accepts either a merchant's own API key
// (Authorization: Bearer sk_test_...) or a conduit-dashboard session
// (X-Dashboard-Session: <jwt>), resolving either to the same merchant_id
// context key RequireAPIKey uses — every downstream handler and middleware
// (rate limiting, usage metering, the payment_intents/webhook_endpoints/
// risk_decisions handlers themselves) works unmodified regardless of which
// credential authenticated the request. The API key is tried first since
// it's the more common, higher-volume path (a merchant's own server-to-
// server integration traffic); the dashboard session is the fallback for
// browser-driven traffic from a logged-in dashboard user.
func RequireMerchantContext(authenticator MerchantAuthenticator, dashboardSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secretKey, ok := bearerToken(r); ok {
				merchantID, err := authenticator.AuthenticateBySecretKey(r.Context(), secretKey)
				if err != nil {
					http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r.WithContext(withMerchantID(r.Context(), merchantID)))
				return
			}

			if sessionToken := r.Header.Get("X-Dashboard-Session"); sessionToken != "" {
				merchantID, err := verifyDashboardSession(dashboardSecret, sessionToken)
				if err != nil {
					http.Error(w, `{"error":"invalid dashboard session"}`, http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r.WithContext(withMerchantID(r.Context(), merchantID)))
				return
			}

			http.Error(w, `{"error":"missing bearer token or dashboard session"}`, http.StatusUnauthorized)
		})
	}
}
