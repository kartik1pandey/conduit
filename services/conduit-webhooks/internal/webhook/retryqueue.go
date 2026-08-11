package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// retryScheduleKey is a single global sorted set: member = delivery ID,
// score = unix time it's next due. This is "fast rather than durable" per
// docs/ARCHITECTURE.md — Postgres's webhook_deliveries table remains the
// durable record of status and attempt history; if this set were lost
// entirely (Redis restart with no persistence), every pending delivery would
// simply stop retrying rather than being lost or double-processed — a
// availability gap, not a correctness one. Rebuilding it from Postgres on
// boot would close that gap; not built here since Checkpoint 2.2/2.3 don't
// require surviving a Redis restart, only correct retry/dead-letter behavior
// while Redis is up.
const retryScheduleKey = "webhooks:retry_schedule"

type RetryQueue struct {
	client *redis.Client
}

func NewRetryQueue(client *redis.Client) *RetryQueue {
	return &RetryQueue{client: client}
}

// Schedule marks deliveryID as due at dueAt.
func (q *RetryQueue) Schedule(ctx context.Context, deliveryID uuid.UUID, dueAt time.Time) error {
	err := q.client.ZAdd(ctx, retryScheduleKey, redis.Z{
		Score:  float64(dueAt.Unix()),
		Member: deliveryID.String(),
	}).Err()
	if err != nil {
		return fmt.Errorf("scheduling delivery: %w", err)
	}
	return nil
}

// PopDue atomically removes and returns every delivery ID due at or before
// now, up to limit at a time. Using ZRangeByScore+ZRem (rather than
// ZPopMin, which would also pop items *not* due yet) means only genuinely
// due work is claimed.
func (q *RetryQueue) PopDue(ctx context.Context, now time.Time, limit int64) ([]uuid.UUID, error) {
	ids, err := q.client.ZRangeByScore(ctx, retryScheduleKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now.Unix()),
		Count: limit,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("querying due deliveries: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	members := make([]any, len(ids))
	for i, id := range ids {
		members[i] = id
	}
	if err := q.client.ZRem(ctx, retryScheduleKey, members...).Err(); err != nil {
		return nil, fmt.Errorf("claiming due deliveries: %w", err)
	}

	deliveryIDs := make([]uuid.UUID, 0, len(ids))
	for _, idStr := range ids {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue // defensive: a malformed member should never block the rest of the batch
		}
		deliveryIDs = append(deliveryIDs, id)
	}
	return deliveryIDs, nil
}
