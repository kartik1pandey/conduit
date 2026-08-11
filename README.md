# CONDUIT

A scoped-but-real payments infrastructure platform: a merchant payment request
is scored for risk in real time, written to an immutable double-entry ledger,
and confirmed via a reliably-delivered, HMAC-signed webhook.

Built as a demonstration of payments-infrastructure fundamentals — idempotency,
exact-decimal ledger accounting, multi-tenant data isolation, and reliable
webhook delivery under adversarial network conditions — over feature breadth.
Test mode only; there is no live-money path anywhere in this project.

## Status

Early scaffolding. See [`docs/CHECKPOINTS.md`](docs/CHECKPOINTS.md) for the
phase-by-phase build plan and what's actually verified so far.

## Architecture

Six independently deployable services. Full design (data models, API
contracts, auth, caching strategy, deployment plan) in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

| Service | Language | Responsibility |
|---|---|---|
| `conduit-core` | Go | Payment API, `payment_intents` state machine, idempotency |
| `conduit-ledger` | Go | Double-entry bookkeeping, debit=credit invariant |
| `conduit-risk` | Python/FastAPI | Risk scoring (wraps AEGIS classifier + OPA/Rego policy) |
| `conduit-webhooks` | Go | Signed, retried webhook delivery |
| `conduit-billing` | Go | Usage metering and invoicing |
| `conduit-dashboard` | Next.js | Merchant-facing UI with RBAC |

```
merchant → conduit-core → conduit-risk (score)
                        → conduit-ledger (post balanced entry)
                        → conduit-webhooks (notify merchant)
```

## Non-negotiables

- Every write endpoint requires and honors an `Idempotency-Key` header.
- No floating point for money — `numeric`/`decimal` only.
- Every ledger transaction's entries net to zero, enforced at the database layer.
- No service reads another service's database directly.
- Every query is scoped to the authenticated `merchant_id`.
- Secrets come from environment variables — see [`.env.example`](.env.example).

## Local development

```bash
docker compose up postgres redis
cp .env.example .env   # fill in real local values
```

Per-service run/test instructions land in each service's own README as it's
built.

## CI

Every PR runs [`pre-commit`](.pre-commit-config.yaml) (hygiene + secret
scanning) plus each service's test suite. See
[`.github/workflows/ci.yml`](.github/workflows/ci.yml).
