package paymentintent

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ErrNotFound covers both "no such id" and "belongs to a different
// merchant" — the two are indistinguishable to the caller, which is what
// enforces multi-tenancy at the query layer (Checkpoint 1.7): every query
// here takes merchantID and every row it can return is scoped to it.
var ErrNotFound = errors.New("payment intent not found")

// ErrConflict means the row's status no longer matches what the caller
// expected when it tried to transition it — another request changed it
// concurrently. Vanishingly rare given the idempotency layer already
// serializes retries of the same key, but the conditional UPDATE makes it
// impossible to silently clobber a concurrent transition either way.
var ErrConflict = errors.New("payment intent status changed concurrently")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, merchantID uuid.UUID, amount decimal.Decimal, currency, description string) (PaymentIntent, error) {
	var pi PaymentIntent
	err := s.pool.QueryRow(ctx, `
		INSERT INTO payment_intents (merchant_id, amount, currency, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, merchant_id, amount::text, currency, status, COALESCE(description, ''), created_at, updated_at
	`, merchantID, amount.String(), currency, description).Scan(
		&pi.ID, &pi.MerchantID, scanDecimal(&pi.Amount), &pi.Currency, &pi.Status, &pi.Description, &pi.CreatedAt, &pi.UpdatedAt,
	)
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("creating payment intent: %w", err)
	}
	return pi, nil
}

func (s *Store) Get(ctx context.Context, merchantID, id uuid.UUID) (PaymentIntent, error) {
	var pi PaymentIntent
	err := s.pool.QueryRow(ctx, `
		SELECT id, merchant_id, amount::text, currency, status, COALESCE(description, ''), created_at, updated_at
		FROM payment_intents
		WHERE id = $1 AND merchant_id = $2
	`, id, merchantID).Scan(
		&pi.ID, &pi.MerchantID, scanDecimal(&pi.Amount), &pi.Currency, &pi.Status, &pi.Description, &pi.CreatedAt, &pi.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentIntent{}, ErrNotFound
	}
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("querying payment intent: %w", err)
	}
	return pi, nil
}

// TransitionStatus moves id from expectedCurrent to next, scoped to
// merchantID, only if the row's status still matches expectedCurrent at
// write time.
func (s *Store) TransitionStatus(ctx context.Context, merchantID, id uuid.UUID, expectedCurrent, next Status) (PaymentIntent, error) {
	var pi PaymentIntent
	err := s.pool.QueryRow(ctx, `
		UPDATE payment_intents
		SET status = $4, updated_at = now()
		WHERE id = $1 AND merchant_id = $2 AND status = $3
		RETURNING id, merchant_id, amount::text, currency, status, COALESCE(description, ''), created_at, updated_at
	`, id, merchantID, expectedCurrent, next).Scan(
		&pi.ID, &pi.MerchantID, scanDecimal(&pi.Amount), &pi.Currency, &pi.Status, &pi.Description, &pi.CreatedAt, &pi.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentIntent{}, ErrConflict
	}
	if err != nil {
		return PaymentIntent{}, fmt.Errorf("transitioning payment intent: %w", err)
	}
	return pi, nil
}

// scanDecimal adapts decimal.Decimal to pgx's Scan, which otherwise only
// knows how to populate it via a *string — amounts are always sent to and
// read from Postgres as text so nothing in this path ever touches a float.
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
