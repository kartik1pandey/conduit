# conduit-ledger

Double-entry bookkeeping service (Go). Every transaction's debit and credit
entries must net to zero, enforced at the database layer. Entries are
append-only. Runs a scheduled reconciliation job against a mock settlement feed.

Scaffolded in Phase 0. Implementation begins Phase 1 — see `docs/CHECKPOINTS.md`.
