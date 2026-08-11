// Package idempotency implements the Idempotency-Key contract required on
// every write endpoint (see CLAUDE.md's non-negotiables): a "claim, do the
// work, fill" pattern rather than a naive check-then-act, so it's correct
// even when two identical requests race each other, not just when they
// arrive one after another.
package idempotency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrKeyReused means the same Idempotency-Key was sent with different
// request parameters than the first time — a client bug, not a legitimate
// retry. Mirrors Stripe's own behavior for this case.
var ErrKeyReused = errors.New("idempotency key reused with different request parameters")

// ErrInFlight means another request with this key is still being processed
// (within the lease window) — the caller should retry shortly, not assume
// failure.
var ErrInFlight = errors.New("a request with this idempotency key is already being processed")

// leaseDuration bounds how long a claimed-but-unfilled key blocks retries.
// If the process handling the original request crashes before filling the
// response, a retry after this window reclaims the key and actually
// reprocesses the request — otherwise a crash would strand that
// Idempotency-Key permanently unusable.
const leaseDuration = 30 * time.Second

type Record struct {
	RequestHash    string
	ResponseStatus *int
	ResponseBody   []byte
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Claim atomically reserves key for merchantID, or reports why it couldn't:
// ErrKeyReused, ErrInFlight, or (if the key was already filled) the
// previously stored Record to replay verbatim.
func (s *Store) Claim(ctx context.Context, merchantID uuid.UUID, key, requestHash string) (claimed bool, existing *Record, err error) {
	var claimedID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		INSERT INTO idempotency_keys (merchant_id, key, request_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (merchant_id, key) DO NOTHING
		RETURNING id
	`, merchantID, key, requestHash).Scan(&claimedID)
	if err == nil {
		return true, nil, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, nil, fmt.Errorf("claiming idempotency key: %w", err)
	}

	var rec Record
	var createdAt time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body, created_at
		FROM idempotency_keys WHERE merchant_id = $1 AND key = $2
	`, merchantID, key).Scan(&rec.RequestHash, &rec.ResponseStatus, &rec.ResponseBody, &createdAt)
	if err != nil {
		return false, nil, fmt.Errorf("fetching existing idempotency key: %w", err)
	}

	if rec.ResponseStatus == nil && time.Since(createdAt) > leaseDuration {
		reclaimed, err := s.reclaimStale(ctx, merchantID, key, requestHash, createdAt)
		if err != nil {
			return false, nil, err
		}
		if reclaimed {
			return true, nil, nil
		}
		// Someone else reclaimed or filled it first; re-check current state.
		return s.Claim(ctx, merchantID, key, requestHash)
	}

	if rec.RequestHash != requestHash {
		return false, nil, ErrKeyReused
	}
	if rec.ResponseStatus == nil {
		return false, nil, ErrInFlight
	}
	return false, &rec, nil
}

func (s *Store) reclaimStale(ctx context.Context, merchantID uuid.UUID, key, requestHash string, expectedCreatedAt time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET request_hash = $3, created_at = now()
		WHERE merchant_id = $1 AND key = $2 AND response_status IS NULL AND created_at = $4
	`, merchantID, key, requestHash, expectedCreatedAt)
	if err != nil {
		return false, fmt.Errorf("reclaiming stale idempotency key: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Fill records the final response against key, so future requests with the
// same key replay it instead of reprocessing.
func (s *Store) Fill(ctx context.Context, merchantID uuid.UUID, key string, status int, body []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE idempotency_keys SET response_status = $3, response_body = $4
		WHERE merchant_id = $1 AND key = $2
	`, merchantID, key, status, body)
	if err != nil {
		return fmt.Errorf("filling idempotency key: %w", err)
	}
	return nil
}
