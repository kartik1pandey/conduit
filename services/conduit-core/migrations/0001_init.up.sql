CREATE TYPE payment_intent_status AS ENUM ('created', 'pending', 'succeeded', 'failed', 'refunded');

CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    -- Only the SHA-256 hash of the secret key is ever stored. Secret keys are
    -- high-entropy random tokens (not user-chosen passwords), so a fast hash
    -- is the right tool here — bcrypt/argon2 are for defending low-entropy
    -- human passwords against offline brute force, which isn't the threat
    -- model for a 256-bit random token.
    secret_key_hash TEXT NOT NULL UNIQUE,
    publishable_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE payment_intents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants (id),
    amount NUMERIC(20, 2) NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL,
    status payment_intent_status NOT NULL DEFAULT 'created',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payment_intents_merchant_id ON payment_intents (merchant_id);

-- Idempotency: "claim, do the work, fill" — see internal/idempotency.
-- response_status/response_body are NULL while a request is in flight, which
-- is how a crashed/slow request is distinguished from a genuinely completed
-- one (see the lease-based reclaim logic in internal/idempotency/store.go).
--
-- response_body is BYTEA, not JSONB: a replayed response must be
-- byte-identical to the original (Checkpoint 1.5). JSONB re-parses and
-- re-serializes on every round trip — different key order, different
-- whitespace — which is fine when you need to query into the JSON's
-- structure, and wrong here, since nothing ever does: this column is only
-- ever written once and replayed verbatim.
CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants (id),
    key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_status INT,
    response_body BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, key)
);
CREATE INDEX idx_idempotency_keys_merchant_id ON idempotency_keys (merchant_id);
