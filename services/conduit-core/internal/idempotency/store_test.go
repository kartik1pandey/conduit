package idempotency

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/internal/merchant"
	"github.com/kartik1pandey/conduit/services/conduit-core/migrations"
)

func setupStore(t *testing.T) (*Store, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	dbURL := os.Getenv("CORE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("CORE_DATABASE_URL not set; skipping integration test")
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
	_, err = pool.Exec(ctx, "TRUNCATE idempotency_keys, payment_intents, merchants CASCADE")
	require.NoError(t, err)

	redisOpts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	redisClient := redis.NewClient(redisOpts)
	t.Cleanup(func() { redisClient.Close() })

	m, _, err := merchant.NewStore(pool).Create(ctx, "Test Merchant")
	require.NoError(t, err)

	return NewStore(pool, redisClient, time.Hour), pool, m.ID
}

func TestClaimAndFill_FreshKeyThenReplay(t *testing.T) {
	store, _, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, existing, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)
	require.Nil(t, existing)

	require.NoError(t, store.Fill(ctx, merchantID, "key-1", 201, []byte(`{"id":"abc"}`)))

	claimed, existing, err = store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.False(t, claimed)
	require.NotNil(t, existing)
	require.Equal(t, 201, *existing.ResponseStatus)
	require.Equal(t, []byte(`{"id":"abc"}`), existing.ResponseBody)
}

func TestClaim_ReusedKeyWithDifferentParamsIsRejected(t *testing.T) {
	store, _, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, _, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, store.Fill(ctx, merchantID, "key-1", 200, []byte(`{}`)))

	_, _, err = store.Claim(ctx, merchantID, "key-1", "hash-b")
	require.ErrorIs(t, err, ErrKeyReused)
}

func TestClaim_InFlightKeyBlocksConcurrentRetry(t *testing.T) {
	store, _, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, _, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)
	// Deliberately not filled yet — simulates a request still being processed.

	_, _, err = store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.ErrorIs(t, err, ErrInFlight)
}

func TestClaim_ReplayIsServedFromCache(t *testing.T) {
	store, _, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, _, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, store.Fill(ctx, merchantID, "key-1", 201, []byte(`{"id":"abc"}`)))

	require.EqualValues(t, 0, store.CacheHitCount(), "no replay has happened yet")

	claimed, existing, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.False(t, claimed)
	require.NotNil(t, existing)
	require.EqualValues(t, 1, store.CacheHitCount(), "the replay should have been served from Redis, not Postgres")
}

func TestFill_SetsRedisTTL(t *testing.T) {
	store, _, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, _, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, store.Fill(ctx, merchantID, "key-1", 200, []byte(`{}`)))

	ttl, err := store.cache.client.TTL(ctx, cacheKey(merchantID, "key-1")).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0), "a cached idempotency key must have an expiry set")
	require.LessOrEqual(t, ttl, time.Hour)
}

func TestClaim_StaleInFlightKeyIsReclaimed(t *testing.T) {
	store, pool, merchantID := setupStore(t)
	ctx := context.Background()

	claimed, _, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed)

	// Simulate the original claim being old enough for its lease to have
	// expired (e.g. the process handling it crashed) without sleeping in
	// the test for real.
	_, err = pool.Exec(ctx, `
		UPDATE idempotency_keys SET created_at = $1 WHERE merchant_id = $2 AND key = $3
	`, time.Now().Add(-leaseDuration-time.Second), merchantID, "key-1")
	require.NoError(t, err)

	claimed, existing, err := store.Claim(ctx, merchantID, "key-1", "hash-a")
	require.NoError(t, err)
	require.True(t, claimed, "a stale in-flight key must be reclaimable, not stuck forever")
	require.Nil(t, existing)
}
