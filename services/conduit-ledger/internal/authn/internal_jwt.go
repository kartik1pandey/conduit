// Package authn verifies the short-lived internal JWT that conduit-core
// signs on every service-to-service call, per the auth design in
// docs/ARCHITECTURE.md: "It's a private Docker network" is not sufficient
// authentication on its own. conduit-ledger only ever trusts a merchant_id
// that arrived via a JWT signed with the shared INTERNAL_JWT_SECRET — never
// one supplied directly in a request body, which a caller could set to
// anything.
package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey int

const merchantIDKey contextKey = iota

type internalClaims struct {
	MerchantID string `json:"merchant_id"`
	jwt.RegisteredClaims
}

// RequireInternalJWT verifies the Authorization: Bearer <jwt> header against
// secret and, on success, stores the token's merchant_id in the request
// context for handlers to read via MerchantIDFromContext.
func RequireInternalJWT(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := bearerToken(r)
			if !ok {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			claims := &internalClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "invalid internal token", http.StatusUnauthorized)
				return
			}

			merchantID, err := uuid.Parse(claims.MerchantID)
			if err != nil {
				http.Error(w, "invalid merchant_id claim", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), merchantIDKey, merchantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MerchantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(merchantIDKey).(uuid.UUID)
	return id, ok
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	return strings.TrimPrefix(header, prefix), true
}
