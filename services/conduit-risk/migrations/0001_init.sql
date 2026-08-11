-- conduit-risk maintains its own minimal event history, independent of
-- conduit-core's payment_intents table — "no service reads another
-- service's database directly" means velocity features (how many times has
-- this merchant been scored recently, how much money) have to come from
-- risk's own recorded history, not a query into Core's data.
--
-- merchant_id has no foreign key, same reason as every other service's
-- merchant_id column in this project: merchants live in conduit-core's
-- database.
CREATE TABLE scoring_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id       UUID NOT NULL,
    payment_intent_id UUID NOT NULL,
    amount            NUMERIC(20, 2) NOT NULL,
    currency          TEXT NOT NULL,
    risk_score        NUMERIC(5, 4) NOT NULL,
    decision          TEXT NOT NULL CHECK (decision IN ('allow', 'decline')),
    stage             TEXT NOT NULL CHECK (stage IN ('rules', 'model')),
    reasons           TEXT[] NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scoring_events_merchant_id_created_at
    ON scoring_events (merchant_id, created_at DESC);
