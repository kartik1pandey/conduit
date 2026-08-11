package billing

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-billing/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-billing/migrations"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("BILLING_DATABASE_URL")
	if dbURL == "" {
		t.Skip("BILLING_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE usage_counters, invoices")
	require.NoError(t, err)

	return NewStore(pool)
}

// TestRecordUsage_NCallsMeansCounterReadsExactlyN is Checkpoint 4.1's exact
// verification step.
func TestRecordUsage_NCallsMeansCounterReadsExactlyN(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	const n = 17
	for i := 0; i < n; i++ {
		require.NoError(t, store.RecordUsage(ctx, merchantID))
	}

	usage, err := store.CurrentUsage(ctx, merchantID)
	require.NoError(t, err)
	require.Equal(t, int64(n), usage.CallCount)
}

func TestRecordUsage_IsScopedPerMerchant(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantA := uuid.New()
	merchantB := uuid.New()

	for i := 0; i < 5; i++ {
		require.NoError(t, store.RecordUsage(ctx, merchantA))
	}
	require.NoError(t, store.RecordUsage(ctx, merchantB))

	usageA, err := store.CurrentUsage(ctx, merchantA)
	require.NoError(t, err)
	require.Equal(t, int64(5), usageA.CallCount)

	usageB, err := store.CurrentUsage(ctx, merchantB)
	require.NoError(t, err)
	require.Equal(t, int64(1), usageB.CallCount, "merchant B's count must be unaffected by merchant A's calls")
}

func TestCurrentUsage_ZeroForAMerchantWithNoCalls(t *testing.T) {
	store := setupStore(t)
	usage, err := store.CurrentUsage(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Zero(t, usage.CallCount)
}

func TestCreateInvoice_RejectsDuplicatePeriod(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()
	period := currentPeriod()

	_, err := store.CreateInvoice(ctx, merchantID, period, 100, CalculateTotal(100, DefaultTiers))
	require.NoError(t, err)

	_, err = store.CreateInvoice(ctx, merchantID, period, 999, CalculateTotal(999, DefaultTiers))
	require.ErrorIs(t, err, ErrAlreadyInvoiced, "re-billing an already-invoiced period must not silently overwrite it")
}
