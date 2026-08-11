package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var ErrNotFound = errors.New("not found")

// ErrAlreadyInvoiced means an invoice for this merchant/period already
// exists — invoices, once created, are never overwritten (see
// migrations/0001_init.up.sql's UNIQUE constraint).
var ErrAlreadyInvoiced = errors.New("an invoice for this merchant and period already exists")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// currentPeriod is the first day of the current UTC month — the row every
// RecordUsage call for "now" increments.
func currentPeriod() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// RecordUsage atomically increments merchantID's counter for the current
// period, creating the row on first use. This is the entirety of
// Checkpoint 4.1: "every authenticated call increments a per-merchant
// counter," implemented as one atomic upsert rather than a
// read-then-write, so concurrent calls from the same merchant can never
// race and lose an increment.
func (s *Store) RecordUsage(ctx context.Context, merchantID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO usage_counters (merchant_id, period, call_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (merchant_id, period)
		DO UPDATE SET call_count = usage_counters.call_count + 1, updated_at = now()
	`, merchantID, currentPeriod())
	if err != nil {
		return fmt.Errorf("recording usage: %w", err)
	}
	return nil
}

func (s *Store) UsageForPeriod(ctx context.Context, merchantID uuid.UUID, period time.Time) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `
		SELECT call_count FROM usage_counters WHERE merchant_id = $1 AND period = $2
	`, merchantID, period).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("querying usage: %w", err)
	}
	return count, nil
}

func (s *Store) CurrentUsage(ctx context.Context, merchantID uuid.UUID) (UsageCounter, error) {
	period := currentPeriod()
	count, err := s.UsageForPeriod(ctx, merchantID, period)
	if err != nil {
		return UsageCounter{}, err
	}
	return UsageCounter{MerchantID: merchantID, Period: period, CallCount: count}, nil
}

// AllMerchantsWithUsage returns every merchant that has at least one usage
// row for period — the set generate-invoices needs to iterate to bill
// everyone with recorded activity that period, no more and no less.
func (s *Store) AllMerchantsWithUsage(ctx context.Context, period time.Time) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT merchant_id FROM usage_counters WHERE period = $1
	`, period)
	if err != nil {
		return nil, fmt.Errorf("listing merchants with usage: %w", err)
	}
	defer rows.Close()

	var merchantIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning merchant id: %w", err)
		}
		merchantIDs = append(merchantIDs, id)
	}
	return merchantIDs, rows.Err()
}

// CreateInvoice fails with ErrAlreadyInvoiced instead of overwriting an
// existing invoice for the same merchant/period — re-running
// generate-invoices for a period that's already been billed must be a
// safe no-op-with-an-error, never a silent double-bill or a changed total
// after the fact.
func (s *Store) CreateInvoice(ctx context.Context, merchantID uuid.UUID, period time.Time, callCount int64, total decimal.Decimal) (Invoice, error) {
	var inv Invoice
	err := s.pool.QueryRow(ctx, `
		INSERT INTO invoices (merchant_id, period, call_count, total_amount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (merchant_id, period) DO NOTHING
		RETURNING id, merchant_id, period, call_count, total_amount::text, created_at
	`, merchantID, period, callCount, total.String()).Scan(
		&inv.ID, &inv.MerchantID, &inv.Period, &inv.CallCount, scanDecimal(&inv.TotalAmount), &inv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrAlreadyInvoiced
	}
	if err != nil {
		return Invoice{}, fmt.Errorf("creating invoice: %w", err)
	}
	return inv, nil
}

func (s *Store) GetInvoice(ctx context.Context, merchantID uuid.UUID, period time.Time) (Invoice, error) {
	var inv Invoice
	err := s.pool.QueryRow(ctx, `
		SELECT id, merchant_id, period, call_count, total_amount::text, created_at
		FROM invoices WHERE merchant_id = $1 AND period = $2
	`, merchantID, period).Scan(
		&inv.ID, &inv.MerchantID, &inv.Period, &inv.CallCount, scanDecimal(&inv.TotalAmount), &inv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrNotFound
	}
	if err != nil {
		return Invoice{}, fmt.Errorf("querying invoice: %w", err)
	}
	return inv, nil
}

// scanDecimal adapts decimal.Decimal to pgx's Scan via a *string — amounts
// are always sent to and read from Postgres as text, the same convention
// every service in this project follows so nothing ever touches a float.
func scanDecimal(dst *decimal.Decimal) *decimalScanner {
	return &decimalScanner{dst: dst}
}

type decimalScanner struct {
	dst *decimal.Decimal
}

func (d *decimalScanner) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("expected string for decimal scan, got %T", src)
	}
	parsed, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("parsing decimal %q: %w", s, err)
	}
	*d.dst = parsed
	return nil
}
