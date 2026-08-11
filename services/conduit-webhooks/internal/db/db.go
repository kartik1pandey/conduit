// Package db owns the Postgres connection pool and the embedded migration
// runner shared by cmd/server and cmd/migrate. Deliberately duplicated from
// conduit-ledger's copy of the same file — see docs/learning for why (each
// service stays self-contained and independently deployable).
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

// Migrate applies every *.up.sql file in migrationsFS that hasn't already
// been recorded in schema_migrations, in filename order, each inside its own
// transaction. See conduit-ledger/internal/db/db.go for the rationale behind
// this hand-rolled runner instead of a dependency like golang-migrate.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsFS embed.FS, dir string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename    TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}
	var filenames []string
	for _, e := range entries {
		if !e.IsDir() {
			filenames = append(filenames, e.Name())
		}
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		var alreadyApplied bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`,
			filename,
		).Scan(&alreadyApplied)
		if err != nil {
			return fmt.Errorf("checking migration status for %s: %w", filename, err)
		}
		if alreadyApplied {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrationsFS, path.Join(dir, filename))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", filename, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("applying migration %s: %w", filename, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, filename,
		); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", filename, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", filename, err)
		}
	}
	return nil
}
