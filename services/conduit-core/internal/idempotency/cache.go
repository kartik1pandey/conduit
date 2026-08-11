package idempotency

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// redisCache is the fast-path read-through cache in front of Postgres,
// exactly as docs/ARCHITECTURE.md specifies: "check Redis before Postgres
// on every write." Postgres remains the durable source of truth — every
// write still goes through Store.Claim/Fill's Postgres logic first; this
// only short-circuits the *read* path for a key that's already been filled
// and cached, which is the common case for a genuine retry within the TTL.
// A cache miss (including a cold cache, or a filled key that expired) falls
// straight through to the unchanged Phase 1 Postgres logic — this cache can
// never be the reason a request is handled incorrectly, only the reason a
// correct answer arrives faster.
type redisCache struct {
	client *redis.Client
	ttl    time.Duration
	hits   atomic.Int64
}

func newRedisCache(client *redis.Client, ttl time.Duration) *redisCache {
	return &redisCache{client: client, ttl: ttl}
}

// HitCount is exposed for the checkpoint 2.4 observability requirement
// ("observable via logging or a cache-hit metric") and for tests to assert
// the cache path was actually taken, not just that the end-to-end behavior
// happened to be correct.
func (c *redisCache) HitCount() int64 {
	return c.hits.Load()
}

func cacheKey(merchantID uuid.UUID, key string) string {
	return fmt.Sprintf("idem:%s:%s", merchantID, key)
}

// get returns the cached Record for (merchantID, key), or nil if there is
// no cache entry (a genuine miss — never treated as an error, since a miss
// is an expected, normal outcome that just means "ask Postgres").
func (c *redisCache) get(ctx context.Context, merchantID uuid.UUID, key string) (*Record, error) {
	values, err := c.client.HGetAll(ctx, cacheKey(merchantID, key)).Result()
	if err != nil {
		return nil, fmt.Errorf("reading idempotency cache: %w", err)
	}
	if len(values) == 0 {
		return nil, nil
	}

	status, err := strconv.Atoi(values["status"])
	if err != nil {
		return nil, fmt.Errorf("parsing cached status: %w", err)
	}

	c.hits.Add(1)
	log.Printf("idempotency: cache hit for key %q", key)

	return &Record{
		RequestHash:    values["request_hash"],
		ResponseStatus: &status,
		ResponseBody:   []byte(values["body"]),
	}, nil
}

// set caches a just-filled response. Called after the durable Postgres
// write already succeeded, and failures here are logged, not propagated —
// losing a cache write only costs the next retry a slightly slower
// Postgres round trip, never correctness.
func (c *redisCache) set(ctx context.Context, merchantID uuid.UUID, key, requestHash string, status int, body []byte) {
	k := cacheKey(merchantID, key)
	pipe := c.client.TxPipeline()
	pipe.HSet(ctx, k, map[string]any{
		"request_hash": requestHash,
		"status":       status,
		"body":         body,
	})
	pipe.Expire(ctx, k, c.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("idempotency: failed to cache key %q: %v", key, err)
	}
}
