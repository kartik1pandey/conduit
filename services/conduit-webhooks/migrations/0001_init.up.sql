CREATE TYPE delivery_status AS ENUM ('pending', 'delivered', 'dead_lettered');

-- merchant_id is a scoping column only, no foreign key — see conduit-ledger's
-- migration for why (merchants live in conduit-core's database).
CREATE TABLE webhook_endpoints (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    url         TEXT NOT NULL,
    -- Generated per endpoint at registration and shown to the merchant once,
    -- like a secret key — mirrors Stripe's per-endpoint signing secret. A
    -- single shared secret across all endpoints would mean one leak
    -- compromises every merchant's webhooks instead of just one endpoint's.
    hmac_secret TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_endpoints_merchant_id ON webhook_endpoints (merchant_id);

-- One row per emitted event. payload is BYTEA, not JSONB, for the same
-- reason conduit-core's idempotency_keys.response_body is BYTEA: the exact
-- bytes are what get HMAC-signed, and every delivery attempt (including
-- retries) must sign and send the identical bytes — jsonb re-parses and
-- re-serializes on every round trip, which would invalidate that.
CREATE TABLE webhook_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id     UUID NOT NULL,
    type            TEXT NOT NULL,
    payload         BYTEA NOT NULL,
    -- Dedupes event *emission* itself (e.g. conduit-core retrying its call
    -- to POST /v1/events after a confirm) — independent of delivery retries,
    -- which are tracked per-endpoint below.
    idempotency_key TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, idempotency_key)
);

-- One row per (event, endpoint) pair: an event with 2 registered endpoints
-- produces 2 deliveries, retried and dead-lettered independently.
CREATE TABLE webhook_deliveries (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_event_id      UUID NOT NULL REFERENCES webhook_events (id),
    webhook_endpoint_id   UUID NOT NULL REFERENCES webhook_endpoints (id),
    status                delivery_status NOT NULL DEFAULT 'pending',
    attempt_count         INT NOT NULL DEFAULT 0,
    last_attempt_at       TIMESTAMPTZ,
    last_response_status  INT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_deliveries_event_id ON webhook_deliveries (webhook_event_id);
CREATE INDEX idx_webhook_deliveries_endpoint_id ON webhook_deliveries (webhook_endpoint_id);

-- "What's due for retry at time T" lives in Redis (a sorted set), not here —
-- Postgres is the durable record of delivery history and current status;
-- Redis is the fast, disposable scheduling structure per
-- docs/ARCHITECTURE.md's caching design. If Redis were flushed, every
-- pending delivery's next-attempt time would need reconstructing from here,
-- which is exactly why this table still records status/attempt_count as the
-- durable source of truth.
