package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// CreateEndpoint registers a new endpoint and generates its per-endpoint
// HMAC secret, returned once — the caller (the HTTP handler) is responsible
// for making sure the plaintext secret is never persisted anywhere but this
// one row.
func (s *Store) CreateEndpoint(ctx context.Context, merchantID uuid.UUID, url string) (Endpoint, string, error) {
	secret, err := GenerateSecret()
	if err != nil {
		return Endpoint{}, "", fmt.Errorf("generating hmac secret: %w", err)
	}

	var e Endpoint
	err = s.pool.QueryRow(ctx, `
		INSERT INTO webhook_endpoints (merchant_id, url, hmac_secret)
		VALUES ($1, $2, $3)
		RETURNING id, merchant_id, url, created_at
	`, merchantID, url, secret).Scan(&e.ID, &e.MerchantID, &e.URL, &e.CreatedAt)
	if err != nil {
		return Endpoint{}, "", fmt.Errorf("creating endpoint: %w", err)
	}
	return e, secret, nil
}

func (s *Store) GetEndpoint(ctx context.Context, merchantID, id uuid.UUID) (Endpoint, error) {
	var e Endpoint
	err := s.pool.QueryRow(ctx, `
		SELECT id, merchant_id, url, created_at FROM webhook_endpoints
		WHERE id = $1 AND merchant_id = $2
	`, id, merchantID).Scan(&e.ID, &e.MerchantID, &e.URL, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrNotFound
	}
	if err != nil {
		return Endpoint{}, fmt.Errorf("querying endpoint: %w", err)
	}
	return e, nil
}

// ListEndpoints returns every endpoint registered for merchantID — used when
// emitting an event to determine who should receive a delivery attempt.
func (s *Store) ListEndpoints(ctx context.Context, merchantID uuid.UUID) ([]Endpoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, merchant_id, url, created_at FROM webhook_endpoints WHERE merchant_id = $1
	`, merchantID)
	if err != nil {
		return nil, fmt.Errorf("listing endpoints: %w", err)
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals to JSON `null`, not `[]` —
	// wrong for an API response a client expects to always be an array,
	// even an empty one (Checkpoint 4.4's dashboard views rely on this).
	endpoints := []Endpoint{}
	for rows.Next() {
		var e Endpoint
		if err := rows.Scan(&e.ID, &e.MerchantID, &e.URL, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning endpoint: %w", err)
		}
		endpoints = append(endpoints, e)
	}
	return endpoints, rows.Err()
}

// CreateEvent is idempotent on (merchant_id, idempotency_key): if the event
// was already emitted, the existing row is returned instead of creating a
// duplicate — this is what makes conduit-core's confirm handler safe to
// retry its call to POST /v1/events without double-emitting. wasNew reports
// whether this call actually inserted a new row (true) or returned an
// existing one (false) — the caller (createEvent handler) uses this to skip
// re-enqueuing deliveries on a replay, which would otherwise double-schedule
// every delivery for a retried emission.
//
// wasNew is computed via `xmax = 0`, a standard Postgres idiom: a row's
// xmax is 0 if and only if no transaction has ever deleted/updated it — for
// a row coming straight out of an INSERT ... ON CONFLICT DO UPDATE, that's
// true precisely when this statement performed the INSERT branch, not the
// UPDATE (conflict) branch.
func (s *Store) CreateEvent(ctx context.Context, merchantID uuid.UUID, eventType, idempotencyKey string, payload []byte) (Event, bool, error) {
	var e Event
	var wasNew bool
	err := s.pool.QueryRow(ctx, `
		INSERT INTO webhook_events (merchant_id, type, payload, idempotency_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (merchant_id, idempotency_key) DO UPDATE SET type = EXCLUDED.type
		RETURNING id, merchant_id, type, payload, idempotency_key, created_at, (xmax = 0) AS was_new
	`, merchantID, eventType, payload, idempotencyKey).Scan(
		&e.ID, &e.MerchantID, &e.Type, &e.Payload, &e.IdempotencyKey, &e.CreatedAt, &wasNew,
	)
	if err != nil {
		return Event{}, false, fmt.Errorf("creating event: %w", err)
	}
	return e, wasNew, nil
}

// CreateDeliveries inserts one pending delivery row per endpoint for event.
func (s *Store) CreateDeliveries(ctx context.Context, eventID uuid.UUID, endpointIDs []uuid.UUID) ([]Delivery, error) {
	deliveries := make([]Delivery, 0, len(endpointIDs))
	for _, endpointID := range endpointIDs {
		var d Delivery
		err := s.pool.QueryRow(ctx, `
			INSERT INTO webhook_deliveries (webhook_event_id, webhook_endpoint_id)
			VALUES ($1, $2)
			RETURNING id, webhook_event_id, webhook_endpoint_id, status, attempt_count, created_at
		`, eventID, endpointID).Scan(&d.ID, &d.WebhookEventID, &d.WebhookEndpointID, &d.Status, &d.AttemptCount, &d.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("creating delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, nil
}

// DeliveryJob is everything internal/delivery.Attempt needs to actually make
// an HTTP call — a join across deliveries, events, and endpoints, since the
// worker only ever operates on delivery IDs (that's what's in the Redis
// retry schedule).
type DeliveryJob struct {
	DeliveryID     uuid.UUID
	AttemptCount   int
	EndpointURL    string
	EndpointSecret string
	EventType      string
	EventPayload   []byte
}

func (s *Store) GetDeliveryJob(ctx context.Context, deliveryID uuid.UUID) (DeliveryJob, error) {
	var job DeliveryJob
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, d.attempt_count, ep.url, ep.hmac_secret, ev.type, ev.payload
		FROM webhook_deliveries d
		JOIN webhook_endpoints ep ON ep.id = d.webhook_endpoint_id
		JOIN webhook_events ev ON ev.id = d.webhook_event_id
		WHERE d.id = $1
	`, deliveryID).Scan(&job.DeliveryID, &job.AttemptCount, &job.EndpointURL, &job.EndpointSecret, &job.EventType, &job.EventPayload)
	if errors.Is(err, pgx.ErrNoRows) {
		return DeliveryJob{}, ErrNotFound
	}
	if err != nil {
		return DeliveryJob{}, fmt.Errorf("querying delivery job: %w", err)
	}
	return job, nil
}

// RecordAttempt updates a delivery's status after an attempt. attemptedAt is
// passed in (rather than using now() in SQL) so tests can control it.
func (s *Store) RecordAttempt(ctx context.Context, deliveryID uuid.UUID, attemptedAt time.Time, responseStatus *int, newStatus DeliveryStatus) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET attempt_count = attempt_count + 1,
		    last_attempt_at = $2,
		    last_response_status = $3,
		    status = $4
		WHERE id = $1
	`, deliveryID, attemptedAt, responseStatus, newStatus)
	if err != nil {
		return fmt.Errorf("recording delivery attempt: %w", err)
	}
	return nil
}

func (s *Store) GetDelivery(ctx context.Context, id uuid.UUID) (Delivery, error) {
	var d Delivery
	err := s.pool.QueryRow(ctx, `
		SELECT id, webhook_event_id, webhook_endpoint_id, status, attempt_count,
		       last_attempt_at, last_response_status, created_at
		FROM webhook_deliveries WHERE id = $1
	`, id).Scan(&d.ID, &d.WebhookEventID, &d.WebhookEndpointID, &d.Status, &d.AttemptCount,
		&d.LastAttemptAt, &d.LastResponseStatus, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Delivery{}, ErrNotFound
	}
	if err != nil {
		return Delivery{}, fmt.Errorf("querying delivery: %w", err)
	}
	return d, nil
}

// ListDeliveriesForEndpoint is scoped through endpoint ownership: it only
// returns rows for an endpoint that actually belongs to merchantID, the same
// "never leak via someone else's ID" guarantee conduit-core's payment_intents
// lookups use.
func (s *Store) ListDeliveriesForEndpoint(ctx context.Context, merchantID, endpointID uuid.UUID) ([]Delivery, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.webhook_event_id, d.webhook_endpoint_id, d.status, d.attempt_count,
		       d.last_attempt_at, d.last_response_status, d.created_at
		FROM webhook_deliveries d
		JOIN webhook_endpoints ep ON ep.id = d.webhook_endpoint_id
		WHERE d.webhook_endpoint_id = $1 AND ep.merchant_id = $2
		ORDER BY d.created_at
	`, endpointID, merchantID)
	if err != nil {
		return nil, fmt.Errorf("listing deliveries: %w", err)
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals to JSON `null`, not `[]` —
	// wrong for an API response a client expects to always be an array,
	// even an empty one.
	deliveries := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.WebhookEventID, &d.WebhookEndpointID, &d.Status, &d.AttemptCount,
			&d.LastAttemptAt, &d.LastResponseStatus, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}
