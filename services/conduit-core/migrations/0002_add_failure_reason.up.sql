-- 0001_init.up.sql already shipped (Phase 1), so this is a new migration
-- rather than an edit to it — the same append-only discipline every
-- service's migrations follow once a migration has actually been applied
-- anywhere.
ALTER TABLE payment_intents ADD COLUMN failure_reason TEXT;
