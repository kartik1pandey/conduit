// Command server runs the conduit-webhooks HTTP API and its background
// delivery-retry worker.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/webhook"
	"github.com/kartik1pandey/conduit/services/conduit-webhooks/migrations"
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

	store := webhook.NewStore(pool)
	queue := webhook.NewRetryQueue(redisClient)
	deliverer := webhook.NewDeliverer(cfg.DeliveryTimeout)
	worker := webhook.NewWorker(store, queue, deliverer, cfg.MaxDeliveryAttempts)
	handlers := webhook.NewHandlers(store, worker)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(store, redisClient))

	protected := http.NewServeMux()
	handlers.Register(protected)
	mux.Handle("/", authn.RequireInternalJWT(cfg.InternalJWTSecret)(protected))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go runRetryLoop(ctx, worker)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("conduit-webhooks listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// runRetryLoop is the production wrapper around Worker.ProcessOnce — a
// plain ticker, not a message queue consumer, since Worker.ProcessOnce
// already pulls exactly the currently-due work from Redis each time. Tests
// call ProcessOnce directly instead of running this loop, so they can
// control time deterministically.
func runRetryLoop(ctx context.Context, worker *webhook.Worker) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := worker.ProcessOnce(ctx); err != nil {
				log.Printf("webhooks: retry loop error: %v", err)
			}
		}
	}
}

func healthHandler(store *webhook.Store, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
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
