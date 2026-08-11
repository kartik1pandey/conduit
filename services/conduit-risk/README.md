# conduit-risk

AEGIS, repurposed as an internal service (Python/FastAPI). Wraps the existing
two-stage classifier and OPA/Rego policy layer behind a `/score` endpoint that
conduit-core calls synchronously on every payment confirmation.

Scaffolded in Phase 0. Implementation begins Phase 3 — see `docs/CHECKPOINTS.md`.
