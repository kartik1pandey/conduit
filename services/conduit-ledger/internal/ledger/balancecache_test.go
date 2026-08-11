package ledger

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestToMinorUnitsAndBack(t *testing.T) {
	tests := []struct {
		amount string
		want   int64
	}{
		{"0.00", 0},
		{"1.00", 100},
		{"49.99", 4999},
		{"100.50", 10050},
	}
	for _, tt := range tests {
		amount, err := decimal.NewFromString(tt.amount)
		require.NoError(t, err)
		units := toMinorUnits(amount)
		require.Equal(t, tt.want, units, "amount %s", tt.amount)
		require.True(t, fromMinorUnits(units).Equal(amount), "round trip for %s", tt.amount)
	}
}

func TestBalance_IsServedFromCacheAfterFirstRead(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	revenue, err := store.UpsertAccount(ctx, merchantID, "revenue", AccountRevenue, "usd")
	require.NoError(t, err)

	_, err = store.PostTransaction(ctx, merchantID, "cache-test-1", "", []EntryInput{
		{AccountID: cash.ID, Amount: decimal.NewFromInt(50), Direction: Debit},
		{AccountID: revenue.ID, Amount: decimal.NewFromInt(50), Direction: Credit},
	})
	require.NoError(t, err)

	// PostTransaction should have already primed the cache via incrBy — a
	// direct cache read (bypassing Balance()) should already show 50.
	cached, ok, err := store.cache.get(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, ok, "PostTransaction should have updated the balance cache")
	require.True(t, cached.Equal(decimal.NewFromInt(50)))

	balance, err := store.Balance(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, balance.Equal(decimal.NewFromInt(50)))
}

func TestBalance_ColdCacheFallsBackToLiveRecomputeAndPopulatesCache(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)

	// No transactions posted at all — the cache has never been touched for
	// this account. Balance() must still return the correct (zero) value by
	// falling back to a live recompute, and must populate the cache.
	balance, err := store.Balance(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, balance.IsZero())

	cached, ok, err := store.cache.get(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, ok, "a cold-cache read must populate the cache for next time")
	require.True(t, cached.IsZero())
}

func TestBalance_MultipleTransactionsAccumulateCorrectly(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	cash, err := store.UpsertAccount(ctx, merchantID, "cash", AccountAsset, "usd")
	require.NoError(t, err)
	revenue, err := store.UpsertAccount(ctx, merchantID, "revenue", AccountRevenue, "usd")
	require.NoError(t, err)

	for i, amount := range []int64{10, 20, 30} {
		_, err = store.PostTransaction(ctx, merchantID, "cache-multi-"+string(rune('a'+i)), "", []EntryInput{
			{AccountID: cash.ID, Amount: decimal.NewFromInt(amount), Direction: Debit},
			{AccountID: revenue.ID, Amount: decimal.NewFromInt(amount), Direction: Credit},
		})
		require.NoError(t, err)
	}

	balance, err := store.Balance(ctx, merchantID, cash.ID)
	require.NoError(t, err)
	require.True(t, balance.Equal(decimal.NewFromInt(60)), "cash balance = %s, want 60", balance)

	revenueBalance, err := store.Balance(ctx, merchantID, revenue.ID)
	require.NoError(t, err)
	require.True(t, revenueBalance.Equal(decimal.NewFromInt(60)), "revenue balance = %s, want 60", revenueBalance)
}
