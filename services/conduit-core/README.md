# conduit-core

The payment API. Owns `payment_intents` and their state machine
(`created → pending → succeeded | failed | refunded`). Authenticates
merchants via `sk_test_...` secret keys, enforces `Idempotency-Key` on every
write, and calls conduit-ledger synchronously on confirm.

## Running locally

```bash
# from the repo root
docker compose up -d postgres redis
# conduit-ledger must also be running — see services/conduit-ledger/README.md

export CORE_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_core
export INTERNAL_JWT_SECRET=local-dev-secret-not-real   # must match conduit-ledger's
export CONDUIT_LEDGER_URL=http://localhost:8002

go run ./cmd/migrate   # apply migrations
go run ./cmd/server    # serves on :8000
```

## Testing

```bash
export CORE_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_core
go test ./...
```

Integration tests connect to a real Postgres instance and are skipped (not
failed) if `CORE_DATABASE_URL` is unset. They don't need a real
conduit-ledger running — a fake one is spun up in-process per test.

## API

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | none | 200 if the database is reachable, 503 otherwise |
| `POST` | `/v1/merchants` | none | Bootstrap a test-mode merchant; returns its secret key once (see note below) |
| `POST` | `/v1/payment_intents` | `Bearer sk_test_...` + `Idempotency-Key` | Create a payment intent, status `created` |
| `GET` | `/v1/payment_intents/{id}` | `Bearer sk_test_...` | Fetch a payment intent (404 if it belongs to another merchant) |
| `POST` | `/v1/payment_intents/{id}/confirm` | `Bearer sk_test_...` + `Idempotency-Key` | Post a balanced transaction to conduit-ledger and mark the intent `succeeded`/`failed` |

**On `POST /v1/merchants` being unauthenticated:** dashboard-based merchant
onboarding doesn't exist yet (that's Phase 4). This is a test-mode-only
bootstrap path and should be gated or removed once real onboarding exists.
