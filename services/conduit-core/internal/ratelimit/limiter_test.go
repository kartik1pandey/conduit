package ratelimit

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setupClient(t *testing.T) *redis.Client {
	t.Helper()
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration test")
	}
	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	return client
}

func TestLimiter_AllowsUpToCapacityThenBlocks(t *testing.T) {
	client := setupClient(t)
	limiter := New(client, 3) // capacity 3, refills at 3/60 = 0.05 tokens/sec — negligible over this test's duration
	ctx := context.Background()
	key := "test-key-" + t.Name()

	for i := 0; i < 3; i++ {
		allowed, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		require.True(t, allowed, "request %d should be allowed within capacity", i+1)
	}

	allowed, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.False(t, allowed, "request beyond capacity should be blocked")
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	client := setupClient(t)
	limiter := New(client, 1)
	ctx := context.Background()
	keyA := "test-key-a-" + t.Name()
	keyB := "test-key-b-" + t.Name()

	allowed, err := limiter.Allow(ctx, keyA)
	require.NoError(t, err)
	require.True(t, allowed)

	blocked, err := limiter.Allow(ctx, keyA)
	require.NoError(t, err)
	require.False(t, blocked, "keyA should now be exhausted")

	allowedB, err := limiter.Allow(ctx, keyB)
	require.NoError(t, err)
	require.True(t, allowedB, "keyB must be unaffected by keyA's exhausted bucket")
}
