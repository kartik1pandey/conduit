CREATE TYPE account_type AS ENUM ('asset', 'liability', 'revenue', 'expense');
CREATE TYPE entry_direction AS ENUM ('debit', 'credit');
CREATE TYPE transaction_status AS ENUM ('pending', 'posted', 'reversed');

-- merchant_id is a scoping column only, not a foreign key: merchants live in
-- conduit-core's database, and services never reach into each other's schema.
-- Its validity is enforced by the internal-JWT auth layer, not by the DB.
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    name TEXT NOT NULL,
    type account_type NOT NULL,
    currency TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, name)
);
CREATE INDEX idx_accounts_merchant_id ON accounts (merchant_id);

CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,
    idempotency_key TEXT NOT NULL,
    status transaction_status NOT NULL DEFAULT 'posted',
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, idempotency_key)
);
CREATE INDEX idx_transactions_merchant_id ON transactions (merchant_id);

CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions (id),
    account_id UUID NOT NULL REFERENCES accounts (id),
    amount NUMERIC(20, 2) NOT NULL CHECK (amount > 0),
    direction entry_direction NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ledger_entries_transaction_id ON ledger_entries (transaction_id);
CREATE INDEX idx_ledger_entries_account_id ON ledger_entries (account_id);

-- Entries are append-only: the repository layer (internal/ledger) exposes no
-- Update/Delete on ledger_entries, only Insert and Select. A stricter version
-- of this would connect as a least-privileged Postgres role with no
-- UPDATE/DELETE grant on the table at all, separate from the role that owns
-- and migrates the schema — deliberately out of scope for now, since it means
-- managing a second credential in every environment for a guarantee the
-- application layer already provides today.

-- Debit=credit invariant, enforced at the database layer.
--
-- This can't be a plain CHECK constraint because the invariant spans multiple
-- rows (every entry in a transaction, not one row in isolation). Instead it's
-- a deferred constraint trigger: it fires once per inserted row, but
-- DEFERRABLE INITIALLY DEFERRED means the actual check doesn't run until
-- COMMIT, by which point every entry in a multi-entry INSERT is visible to
-- it. A balanced transaction (posted as several INSERTs inside one DB
-- transaction) commits normally; an unbalanced one fails at COMMIT and every
-- entry in it is rolled back atomically — never a partial write.
CREATE OR REPLACE FUNCTION check_transaction_balance() RETURNS TRIGGER AS $$
DECLARE
    debit_total NUMERIC;
    credit_total NUMERIC;
BEGIN
    SELECT
        COALESCE(SUM(amount) FILTER (WHERE direction = 'debit'), 0),
        COALESCE(SUM(amount) FILTER (WHERE direction = 'credit'), 0)
    INTO debit_total, credit_total
    FROM ledger_entries
    WHERE transaction_id = NEW.transaction_id;

    IF debit_total <> credit_total THEN
        RAISE EXCEPTION 'transaction % is unbalanced: debits=% credits=%',
            NEW.transaction_id, debit_total, credit_total
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_check_transaction_balance
    AFTER INSERT ON ledger_entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_transaction_balance();
