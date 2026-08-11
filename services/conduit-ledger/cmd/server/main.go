// Command server runs the conduit-ledger HTTP API.
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

	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/authn"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/ledger"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/migrations"
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

	store := ledger.NewStore(pool, redisClient)
	handlers := ledger.NewHandlers(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(store, redisClient))

	protected := http.NewServeMux()
	handlers.Register(protected)
	mux.Handle("/", authn.RequireInternalJWT(cfg.InternalJWTSecret)(protected))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("conduit-ledger listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(store *ledger.Store, redisClient *redis.Client) http.HandlerFunc {
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
