// Command migrate applies pending SQL migrations to LEDGER_DATABASE_URL.
// Usage: go run ./cmd/migrate
package main

import (
	"context"
	"log"

	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS, "."); err != nil {
		log.Fatalf("running migrations: %v", err)
	}
	log.Println("migrations applied")
}
