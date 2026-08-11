# conduit-webhooks

Delivers signed event payloads to merchant-registered URLs (Go). HMAC-SHA256
signatures in a `Conduit-Signature` header. Retries with exponential backoff
plus jitter; repeated failures move to a dead-letter state.

Scaffolded in Phase 0. Implementation begins Phase 2 — see `docs/CHECKPOINTS.md`.
