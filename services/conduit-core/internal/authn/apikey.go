// Package authn handles both directions of authentication conduit-core
// deals with: merchant API keys on the public API (this file), and the
// internal JWT it signs to call conduit-ledger (internal_jwt.go).
package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

var ErrInvalidAPIKey = errors.New("invalid API key")

// MerchantAuthenticator resolves a merchant's secret key to their
// merchant_id. Defined here (rather than depending on the merchant package
// directly) so this middleware stays testable without a real store.
type MerchantAuthenticator interface {
	AuthenticateBySecretKey(ctx context.Context, secretKey string) (uuid.UUID, error)
}

// RequireAPIKey resolves Authorization: Bearer sk_test_... to a merchant_id
// and stores it in the request context. Every downstream query must scope
// itself to this ID — that's the actual multi-tenancy boundary, not this
// middleware alone (see docs/ARCHITECTURE.md's auth section).
func RequireAPIKey(authenticator MerchantAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secretKey, ok := bearerToken(r)
			if !ok {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}

			merchantID, err := authenticator.AuthenticateBySecretKey(r.Context(), secretKey)
			if err != nil {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}

			ctx := withMerchantID(r.Context(), merchantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	return strings.TrimPrefix(header, prefix), true
}
