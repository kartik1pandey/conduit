-- merchant_id has no foreign key, same reason as every other service's
-- merchant_id column in this project: merchants live in conduit-core's
-- database, and no service reads another's database directly.
--
-- period is the first day of the billing month (e.g. 2026-08-01 for
-- August 2026) — monthly granularity matches how the tiered pricing in
-- internal/billing/pricing.go is meant to be read: "calls this month."
CREATE TABLE usage_counters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    period      DATE NOT NULL,
    call_count  BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, period)
);
CREATE INDEX idx_usage_counters_merchant_id ON usage_counters (merchant_id);

-- Append-only in spirit (a real invoice is never mutated after creation —
-- see internal/billing/repository.go's CreateInvoice, which errors on a
-- duplicate (merchant_id, period) rather than overwriting one).
CREATE TABLE invoices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id  UUID NOT NULL,
    period       DATE NOT NULL,
    call_count   BIGINT NOT NULL,
    total_amount NUMERIC(20, 2) NOT NULL, -- never float, same rule as every amount in this project
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, period)
);
CREATE INDEX idx_invoices_merchant_id ON invoices (merchant_id);
