package ratelimit

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
)

// RequireWithinLimit must be applied inside (after) authn.RequireAPIKey — it
// reads the merchant_id that middleware resolved, so throttling is always
// per authenticated merchant, never per raw IP or global.
func RequireWithinLimit(limiter *Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			merchantID, ok := authn.MerchantIDFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"missing merchant context"}`, http.StatusUnauthorized)
				return
			}

			allowed, err := limiter.Allow(r.Context(), merchantID.String())
			if err != nil {
				log.Printf("ratelimit: check failed for merchant %s: %v", merchantID, err)
				http.Error(w, `{"error":"rate limit check failed"}`, http.StatusInternalServerError)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
