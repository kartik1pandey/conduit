package paymentintent

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/merchant"
	"github.com/kartik1pandey/conduit/services/conduit-core/migrations"
)

// setupStore returns a payment-intent Store plus a helper that creates a
// real merchant row — payment_intents.merchant_id has a foreign key to
// merchants, so tests can't use an arbitrary uuid.New() the way
// conduit-ledger's tests do (accounts.merchant_id has no such FK there,
// deliberately — see migrations/0001_init.up.sql's comment on that).
func setupStore(t *testing.T) (*Store, func(t *testing.T) uuid.UUID) {
	t.Helper()
	dbURL := os.Getenv("CORE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("CORE_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE idempotency_keys, payment_intents, merchants CASCADE")
	require.NoError(t, err)

	merchantStore := merchant.NewStore(pool)
	newMerchant := func(t *testing.T) uuid.UUID {
		t.Helper()
		m, _, err := merchantStore.Create(ctx, "Test Merchant")
		require.NoError(t, err)
		return m.ID
	}

	return NewStore(pool), newMerchant
}

func TestCreateAndGet(t *testing.T) {
	store, newMerchant := setupStore(t)
	merchantID := newMerchant(t)

	pi, err := store.Create(context.Background(), merchantID, decimal.NewFromFloat(49.99), "usd", "test charge")
	require.NoError(t, err)
	require.Equal(t, StatusCreated, pi.Status)

	fetched, err := store.Get(context.Background(), merchantID, pi.ID)
	require.NoError(t, err)
	require.Equal(t, pi.ID, fetched.ID)
	require.True(t, fetched.Amount.Equal(decimal.NewFromFloat(49.99)))
}

func TestGetIsScopedPerMerchant(t *testing.T) {
	store, newMerchant := setupStore(t)
	merchantA := newMerchant(t)
	merchantB := newMerchant(t)

	pi, err := store.Create(context.Background(), merchantA, decimal.NewFromInt(10), "usd", "")
	require.NoError(t, err)

	_, err = store.Get(context.Background(), merchantB, pi.ID)
	require.ErrorIs(t, err, ErrNotFound, "merchant B must not be able to read merchant A's payment intent")
}

func TestTransitionStatus(t *testing.T) {
	store, newMerchant := setupStore(t)
	merchantID := newMerchant(t)
	ctx := context.Background()

	pi, err := store.Create(ctx, merchantID, decimal.NewFromInt(10), "usd", "")
	require.NoError(t, err)

	pending, err := store.TransitionStatus(ctx, merchantID, pi.ID, StatusCreated, StatusPending)
	require.NoError(t, err)
	require.Equal(t, StatusPending, pending.Status)

	succeeded, err := store.TransitionStatus(ctx, merchantID, pi.ID, StatusPending, StatusSucceeded)
	require.NoError(t, err)
	require.Equal(t, StatusSucceeded, succeeded.Status)
}

func TestTransitionStatus_WrongExpectedStateIsRejected(t *testing.T) {
	store, newMerchant := setupStore(t)
	merchantID := newMerchant(t)
	ctx := context.Background()

	pi, err := store.Create(ctx, merchantID, decimal.NewFromInt(10), "usd", "")
	require.NoError(t, err)

	// pi is still "created" — trying to transition as if it were "pending"
	// must fail rather than silently applying to the wrong state.
	_, err = store.TransitionStatus(ctx, merchantID, pi.ID, StatusPending, StatusSucceeded)
	require.ErrorIs(t, err, ErrConflict)
}
