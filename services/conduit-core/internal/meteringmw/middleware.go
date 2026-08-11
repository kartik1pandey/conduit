// Package meteringmw records billing usage for every authenticated request.
// Checkpoint 4.1 ("every authenticated call increments a per-merchant
// counter") applies uniformly across the API, so this sits in the same
// middleware chain slot as rate limiting — after authn.RequireAPIKey has
// resolved a merchant_id, before the request reaches a handler.
package meteringmw

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
)

type UsageRecorder interface {
	RecordUsage(ctx context.Context, merchantID uuid.UUID) error
}

// RecordUsage fires the usage-record call in the background, on its own
// timeout detached from the request context, and only logs on failure. A
// conduit-billing outage must never turn into a failed or slowed-down
// payment API call — metering is a side effect of traffic, not a gate on it,
// unlike the risk call in paymentintent.confirm which is deliberately
// synchronous and blocking.
func RecordUsage(recorder UsageRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if merchantID, ok := authn.MerchantIDFromContext(r.Context()); ok {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := recorder.RecordUsage(ctx, merchantID); err != nil {
						log.Printf("meteringmw: could not record usage for merchant %s: %v", merchantID, err)
					}
				}()
			}
			next.ServeHTTP(w, r)
		})
	}
}
