-- conduit-dashboard's only table: a dashboard login, distinct from a
-- merchant's own sk_test_.../pk_test_... API key pair. merchant_id has no
-- foreign key, same reason as every other service's merchant_id column in
-- this project: merchants live in conduit-core's database, and no service
-- reads another's database directly. A user is created either by
-- verify-secret signup (owner, password_hash set) or an owner's invite
-- (developer/read-only, password_hash may be set later at first login or
-- left null for an OAuth-only account).
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id   UUID NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    role          TEXT NOT NULL CHECK (role IN ('owner', 'developer', 'read-only')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_merchant_id ON users (merchant_id);
