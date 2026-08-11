package ledger

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-ledger/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-ledger/migrations"

	"github.com/google/uuid"
)

// setupStore connects to a real Postgres instance and returns a Store backed
// by freshly migrated, empty tables. Integration tests are skipped (not
// failed) when LEDGER_DATABASE_URL isn't set, so `go test ./...` still works
// in an environment with no database — CI's per-service job sets the env var
// explicitly once it exists (see .github/workflows/ci.yml).
func setupStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("LEDGER_DATABASE_URL")
	if dbURL == "" {
		t.Skip("LEDGER_DATABASE_URL not set; skipping integration test")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE ledger_entries, transactions, accounts")
	require.NoError(t, err)

	redisOpts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	redisClient := redis.NewClient(redisOpts)
	t.Cleanup(func() { redisClient.Close() })
	require.NoError(t, redisClient.FlushDB(ctx).Err())

	return NewStore(pool, redisClient)
}

func TestPostTransaction_Balanced(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	revenue, err := store.UpsertAccount(ctx, merchantID, "revenue", AccountRevenue, "usd")
	require.NoError(t, err)

	txn, err := store.PostTransaction(ctx, merchantID, "pi_123-confirm", "payment confirmed", []EntryInput{
		{AccountID: cash.ID, Amount: decimal.NewFromInt(100), Direction: Debit},
		{AccountID: revenue.ID, Amount: decimal.NewFromInt(100), Direction: Credit},
	})
	require.NoError(t, err)
	require.Equal(t, "posted", txn.Status)
	require.Len(t, txn.Entries, 2)

	cashBalance, err := store.Balance(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, cashBalance.Equal(decimal.NewFromInt(100)), "cash balance = %s, want 100", cashBalance)

	revenueBalance, err := store.Balance(ctx, merchantID, revenue.ID)
	require.NoError(t, err)
	require.True(t, revenueBalance.Equal(decimal.NewFromInt(100)), "revenue balance = %s, want 100", revenueBalance)
}

func TestPostTransaction_UnbalancedIsRejectedWithNoPartialWrite(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	revenue, err := store.UpsertAccount(ctx, merchantID, "revenue", AccountRevenue, "usd")
	require.NoError(t, err)

	_, err = store.PostTransaction(ctx, merchantID, "pi_unbalanced", "should fail", []EntryInput{
		{AccountID: cash.ID, Amount: decimal.NewFromInt(100), Direction: Debit},
		{AccountID: revenue.ID, Amount: decimal.NewFromInt(50), Direction: Credit},
	})
	require.ErrorIs(t, err, ErrUnbalanced)

	_, err = store.GetTransactionByIdempotencyKey(ctx, merchantID, "pi_unbalanced")
	require.ErrorIs(t, err, ErrNotFound, "a rejected transaction must leave no row behind")
}

func TestPostTransaction_IdempotentReplayDoesNotDuplicate(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	revenue, err := store.UpsertAccount(ctx, merchantID, "revenue", AccountRevenue, "usd")
	require.NoError(t, err)

	entries := []EntryInput{
		{AccountID: cash.ID, Amount: decimal.NewFromInt(100), Direction: Debit},
		{AccountID: revenue.ID, Amount: decimal.NewFromInt(100), Direction: Credit},
	}

	first, err := store.PostTransaction(ctx, merchantID, "pi_retry", "first attempt", entries)
	require.NoError(t, err)

	second, err := store.PostTransaction(ctx, merchantID, "pi_retry", "first attempt", entries)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "replay must return the original transaction, not create a new one")

	balance, err := store.Balance(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, balance.Equal(decimal.NewFromInt(100)), "balance = %s, want 100 (entries must not be double-posted)", balance)
}

func TestUpsertAccount_IsIdempotentByName(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	first, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	second, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
}

func TestAccountsAreScopedPerMerchant(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantA := uuid.New()
	merchantB := uuid.New()

	account, err := store.UpsertAccount(ctx, merchantA, "cash", AccountAsset, "usd")
	require.NoError(t, err)

	_, err = store.Balance(ctx, merchantB, account.ID)
	require.ErrorIs(t, err, ErrNotFound, "merchant B must not be able to read merchant A's account")
}
