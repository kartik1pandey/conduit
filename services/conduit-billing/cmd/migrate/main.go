// Command migrate applies pending SQL migrations to BILLING_DATABASE_URL.
package main

import (
	"context"
	"log"

	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/config"
	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-billing/migrations"
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
