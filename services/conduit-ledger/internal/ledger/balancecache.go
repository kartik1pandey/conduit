package ledger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// balanceCache is docs/ARCHITECTURE.md's balance cache: "cache the current
// balance, update on write, let the reconciliation job be the source of
// truth that catches drift between cache and a full recompute." Postgres
// (summed straight from ledger_entries) remains authoritative; this cache
// only ever saves a read from having to do that sum.
//
// It stores balances as integer minor units (cents), not decimal strings,
// specifically so it can use Redis's atomic INCRBY to apply a write's delta
// without a read-modify-write round trip. The alternative — Redis's
// INCRBYFLOAT — is IEEE-754 floating point internally, and CLAUDE.md's
// "never use floating point for money" doesn't carve out an exception for
// "only in the cache layer." Integer cents sidesteps the question entirely:
// there's no float anywhere in this file.
type balanceCache struct {
	client *redis.Client
}

func newBalanceCache(client *redis.Client) *balanceCache {
	return &balanceCache{client: client}
}

func balanceCacheKey(merchantID, accountID uuid.UUID) string {
	return fmt.Sprintf("balance:%s:%s", merchantID, accountID)
}

// toMinorUnits converts a NUMERIC(20,2) amount to integer cents. It's exact,
// not an approximation: amounts in this schema always have exactly 2
// decimal places, so multiplying by 100 and taking the integer part never
// discards a fractional cent.
func toMinorUnits(amount decimal.Decimal) int64 {
	return amount.Mul(decimal.New(100, 0)).IntPart()
}

func fromMinorUnits(units int64) decimal.Decimal {
	return decimal.New(units, -2)
}

// get returns the cached balance, or ok=false on a cache miss (cold cache,
// evicted key, or a merchant/account never cached before) — always treated
// as "go compute it live," never as an error.
func (c *balanceCache) get(ctx context.Context, merchantID, accountID uuid.UUID) (decimal.Decimal, bool, error) {
	units, err := c.client.Get(ctx, balanceCacheKey(merchantID, accountID)).Int64()
	if err == redis.Nil {
		return decimal.Zero, false, nil
	}
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("reading balance cache: %w", err)
	}
	return fromMinorUnits(units), true, nil
}

// set overwrites the cached balance outright — used once, right after a
// live recompute on a cache miss, to populate a cold cache.
func (c *balanceCache) set(ctx context.Context, merchantID, accountID uuid.UUID, balance decimal.Decimal) error {
	if err := c.client.Set(ctx, balanceCacheKey(merchantID, accountID), toMinorUnits(balance), 0).Err(); err != nil {
		return fmt.Errorf("writing balance cache: %w", err)
	}
	return nil
}

// incrBy atomically applies a signed delta to the cached balance — this is
// the "update on write" half of the design, letting a hot cache stay hot
// across writes instead of being invalidated (and needing a live recompute)
// on every single transaction.
func (c *balanceCache) incrBy(ctx context.Context, merchantID, accountID uuid.UUID, deltaMinorUnits int64) error {
	if err := c.client.IncrBy(ctx, balanceCacheKey(merchantID, accountID), deltaMinorUnits).Err(); err != nil {
		return fmt.Errorf("updating balance cache: %w", err)
	}
	return nil
}
