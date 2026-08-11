// Package ratelimit implements a token-bucket limiter per API key
// (docs/ARCHITECTURE.md: "one bad actor should not degrade service for
// everyone else"), backed by Redis so it works correctly across multiple
// conduit-core instances, not just within one process's memory.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// checkAndConsume is a Lua script, not a Go-side read-then-write, because a
// token bucket has to be atomic under concurrency: two simultaneous requests
// from the same merchant both reading "3 tokens left" and both deciding to
// proceed would let through one more request than the bucket allows. Redis
// runs the whole script as a single atomic operation, so there's no window
// for two callers to observe the same pre-consumption state.
const checkAndConsumeScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_per_second = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local bucket = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
  tokens = capacity
  last_refill = now
end

local elapsed = math.max(0, now - last_refill)
tokens = math.min(capacity, tokens + elapsed * refill_per_second)

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call("HSET", key, "tokens", tokens, "last_refill", now)
redis.call("EXPIRE", key, 3600)

return allowed
`

var script = redis.NewScript(checkAndConsumeScript)

type Limiter struct {
	client          *redis.Client
	capacity        int
	refillPerSecond float64
}

// New creates a limiter where each key gets its own bucket of size
// requestsPerMinute, refilling continuously at requestsPerMinute/60 tokens
// per second (not "reset to full every 60 seconds" — a fixed-window reset
// lets a client burst its entire budget right at the window boundary twice
// in quick succession; continuous refill doesn't have that edge).
func New(client *redis.Client, requestsPerMinute int) *Limiter {
	return &Limiter{
		client:          client,
		capacity:        requestsPerMinute,
		refillPerSecond: float64(requestsPerMinute) / 60.0,
	}
}

// Allow consumes one token from key's bucket and reports whether it was
// available. key is expected to be the authenticated merchant_id — the
// limit is per API key/merchant, so one merchant being throttled must never
// affect another's bucket.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	now := float64(time.Now().UnixNano()) / 1e9
	result, err := script.Run(ctx, l.client, []string{"ratelimit:" + key}, l.capacity, l.refillPerSecond, now).Result()
	if err != nil {
		return false, fmt.Errorf("checking rate limit: %w", err)
	}
	allowed, _ := result.(int64)
	return allowed == 1, nil
}
