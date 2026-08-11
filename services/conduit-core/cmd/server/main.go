// Command server runs the conduit-core HTTP API.
//
// @title						Conduit API
// @version					1.0
// @description				The payment API for Conduit, a test-mode payments infrastructure platform. Mirrors Stripe's own object naming (payment_intents) and auth model (sk_test_/pk_test_ key pairs) deliberately.
// @contact.name				Conduit
// @license.name				MIT
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						Authorization
// @description				Merchant secret key, sent as "Bearer sk_test_...". Obtained once from POST /v1/merchants.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/redis/go-redis/v9"

	_ "github.com/kartik1pandey/conduit/services/conduit-core/docs"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/billingclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/idempotency"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ledgerclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/merchant"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/meteringmw"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/paymentintent"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/ratelimit"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/riskclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/riskdecision"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhookendpoint"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/webhooksclient"
	"github.com/kartik1pandey/conduit/services/conduit-core/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS, "."); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("parsing REDIS_URL: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connecting to redis: %v", err)
	}

	merchantStore := merchant.NewStore(pool)
	merchantHandlers := merchant.NewHandlers(merchantStore)

	idemStore := idempotency.NewStore(pool, redisClient, cfg.IdempotencyCacheTTL)
	requireIdempotency := idempotency.RequireKey(idemStore)

	limiter := ratelimit.New(redisClient, cfg.RateLimitPerMinute)
	requireWithinLimit := ratelimit.RequireWithinLimit(limiter)

	ledgerClient := ledgerclient.New(cfg.LedgerBaseURL, cfg.InternalJWTSecret, cfg.LedgerCallTimeout)
	webhooksClient := webhooksclient.New(cfg.WebhooksBaseURL, cfg.InternalJWTSecret, cfg.LedgerCallTimeout)
	riskClient := riskclient.New(cfg.RiskBaseURL, cfg.InternalJWTSecret, cfg.RiskCallTimeout)
	billingClient := billingclient.New(cfg.BillingBaseURL, cfg.InternalJWTSecret, cfg.BillingCallTimeout)
	recordUsage := meteringmw.RecordUsage(billingClient)

	piStore := paymentintent.NewStore(pool)
	piHandlers := paymentintent.NewHandlers(piStore, ledgerClient, riskClient, webhooksClient)
	webhookEndpointHandlers := webhookendpoint.NewHandlers(webhooksClient)
	riskDecisionHandlers := riskdecision.NewHandlers(riskClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(pool, redisClient))
	mux.Handle("/swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))) // public API docs, no API key required to read them
	merchantHandlers.RegisterUnauthenticated(mux)                                      // no API key exists yet for a merchant that doesn't exist yet

	protected := http.NewServeMux()
	piHandlers.Register(protected, requireIdempotency)
	webhookEndpointHandlers.Register(protected, requireIdempotency)
	riskDecisionHandlers.Register(protected)
	mux.Handle("/", authn.RequireMerchantContext(merchantStore, cfg.DashboardSessionSecret)(requireWithinLimit(recordUsage(protected))))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("conduit-core listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// healthHandler godoc
//
//	@Summary		Health check
//	@Description	Returns 200 if Postgres and Redis are both reachable, 503 otherwise.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/health [get]
func healthHandler(pool interface{ Ping(context.Context) error }, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			log.Printf("health check: database unreachable: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("health check: redis unreachable: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
