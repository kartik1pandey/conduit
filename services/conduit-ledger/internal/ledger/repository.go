package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ErrUnbalanced is returned when a transaction's entries don't net to zero.
// It surfaces the database's own constraint-trigger rejection (SQLSTATE
// 23514, check_violation) as a typed Go error the HTTP layer can map to 422,
// rather than leaking a raw Postgres error to the caller.
var ErrUnbalanced = errors.New("transaction entries do not net to zero")

// ErrNotFound is returned when a lookup scoped to merchantID finds nothing —
// either because the row doesn't exist, or because it belongs to a different
// merchant. Callers can't distinguish those cases, which is the point: this
// is what enforces multi-tenancy at the query layer.
var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// UpsertAccount creates the named account for merchantID, or returns the
// existing one if an account with that name already exists for that
// merchant. Callers (conduit-core, provisioning its default chart of
// accounts) can call this on every confirm without worrying about creating
// duplicates.
func (s *Store) UpsertAccount(ctx context.Context, merchantID uuid.UUID, name string, accountType AccountType, currency string) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (merchant_id, name, type, currency)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (merchant_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, merchant_id, name, type, currency, created_at
	`, merchantID, name, accountType, currency).Scan(&a.ID, &a.MerchantID, &a.Name, &a.Type, &a.Currency, &a.CreatedAt)
	if err != nil {
		return Account{}, fmt.Errorf("upserting account: %w", err)
	}
	return a, nil
}

// PostTransaction posts a balanced transaction: all entries are inserted in
// a single database transaction, and the debit=credit invariant is checked
// by the deferred constraint trigger at COMMIT (see migrations/0001). If
// idempotencyKey has already been used for this merchant, the existing
// transaction is returned unchanged instead of writing anything new.
func (s *Store) PostTransaction(ctx context.Context, merchantID uuid.UUID, idempotencyKey, description string, entries []EntryInput) (Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transaction{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	var txnID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO transactions (merchant_id, idempotency_key, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
		RETURNING id
	`, merchantID, idempotencyKey, description).Scan(&txnID)

	if errors.Is(err, pgx.ErrNoRows) {
		// Idempotency key already used by this merchant: this is a replay,
		// not a new write. Return the transaction as originally posted.
		tx.Rollback(ctx) //nolint:errcheck
		return s.GetTransactionByIdempotencyKey(ctx, merchantID, idempotencyKey)
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("inserting transaction: %w", err)
	}

	for _, e := range entries {
		if _, err := tx.Exec(ctx, `
			INSERT INTO ledger_entries (transaction_id, account_id, amount, direction)
			VALUES ($1, $2, $3, $4)
		`, txnID, e.AccountID, e.Amount.String(), string(e.Direction)); err != nil {
			return Transaction{}, fmt.Errorf("inserting entry: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return Transaction{}, ErrUnbalanced
		}
		return Transaction{}, fmt.Errorf("committing transaction: %w", err)
	}

	return s.GetTransaction(ctx, merchantID, txnID)
}

func (s *Store) GetTransaction(ctx context.Context, merchantID, transactionID uuid.UUID) (Transaction, error) {
	var t Transaction
	err := s.pool.QueryRow(ctx, `
		SELECT id, merchant_id, idempotency_key, status, COALESCE(description, ''), created_at
		FROM transactions
		WHERE id = $1 AND merchant_id = $2
	`, transactionID, merchantID).Scan(&t.ID, &t.MerchantID, &t.IdempotencyKey, &t.Status, &t.Description, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("querying transaction: %w", err)
	}

	entries, err := s.entriesForTransaction(ctx, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	t.Entries = entries
	return t, nil
}

func (s *Store) GetTransactionByIdempotencyKey(ctx context.Context, merchantID uuid.UUID, idempotencyKey string) (Transaction, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM transactions WHERE merchant_id = $1 AND idempotency_key = $2
	`, merchantID, idempotencyKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("querying transaction by idempotency key: %w", err)
	}
	return s.GetTransaction(ctx, merchantID, id)
}

func (s *Store) entriesForTransaction(ctx context.Context, transactionID uuid.UUID) ([]Entry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, transaction_id, account_id, amount::text, direction, created_at
		FROM ledger_entries
		WHERE transaction_id = $1
		ORDER BY created_at
	`, transactionID)
	if err != nil {
		return nil, fmt.Errorf("querying entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var amountText string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &amountText, &e.Direction, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning entry: %w", err)
		}
		amount, err := decimal.NewFromString(amountText)
		if err != nil {
			return nil, fmt.Errorf("parsing entry amount: %w", err)
		}
		e.Amount = amount
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Balance returns accountID's current balance, scoped to merchantID so one
// merchant can never query another's account by guessing an ID. The sign
// convention follows standard double-entry accounting: debits increase
// asset/expense accounts and decrease liability/revenue accounts.
func (s *Store) Balance(ctx context.Context, merchantID, accountID uuid.UUID) (decimal.Decimal, error) {
	var accountType AccountType
	err := s.pool.QueryRow(ctx, `
		SELECT type FROM accounts WHERE id = $1 AND merchant_id = $2
	`, accountID, merchantID).Scan(&accountType)
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, ErrNotFound
	}
	if err != nil {
		return decimal.Zero, fmt.Errorf("querying account: %w", err)
	}

	var debitTotal, creditTotal string
	err = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE direction = 'debit'), 0)::text,
			COALESCE(SUM(amount) FILTER (WHERE direction = 'credit'), 0)::text
		FROM ledger_entries
		WHERE account_id = $1
	`, accountID).Scan(&debitTotal, &creditTotal)
	if err != nil {
		return decimal.Zero, fmt.Errorf("summing entries: %w", err)
	}

	debits, err := decimal.NewFromString(debitTotal)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parsing debit total: %w", err)
	}
	credits, err := decimal.NewFromString(creditTotal)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parsing credit total: %w", err)
	}

	switch accountType {
	case AccountAsset, AccountExpense:
		return debits.Sub(credits), nil
	default: // liability, revenue
		return credits.Sub(debits), nil
	}
}
