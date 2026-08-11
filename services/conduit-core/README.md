# conduit-core

The payment API (Go). Owns `payment_intents` and their state machine
(`created → pending → succeeded | failed | refunded`). Authenticates merchants,
enforces idempotency, and orchestrates calls to conduit-risk and conduit-ledger.

Scaffolded in Phase 0. Implementation begins Phase 1 — see `docs/CHECKPOINTS.md`.
