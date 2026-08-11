# conduit-ledger

Double-entry bookkeeping service. Every transaction is a set of `ledger_entries`
that must net to zero — enforced by a deferred constraint trigger in Postgres
(see `migrations/0001_init.up.sql`), not just application code. Entries are
append-only.

## Running locally

```bash
# from the repo root
docker compose up -d postgres redis

export LEDGER_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_ledger
export INTERNAL_JWT_SECRET=local-dev-secret-not-real

go run ./cmd/migrate   # apply migrations
go run ./cmd/server    # serves on :8002
```

## Testing

```bash
export LEDGER_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_ledger
export INTERNAL_JWT_SECRET=local-dev-secret-not-real
go test ./...
```

Integration tests connect to a real Postgres instance and are skipped (not
failed) if `LEDGER_DATABASE_URL` is unset.

## API

All endpoints except `/health` require `Authorization: Bearer <internal JWT>`,
signed by conduit-core with the shared `INTERNAL_JWT_SECRET` and carrying a
`merchant_id` claim. Every query is scoped to that merchant_id — never to
anything supplied in the request body.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | 200 if the database is reachable, 503 otherwise |
| `POST` | `/v1/accounts` | Create (or fetch, if the name already exists) an account |
| `POST` | `/v1/transactions` | Post a balanced transaction; replays the original response if `idempotency_key` was already used |
| `GET` | `/v1/accounts/{id}/balance` | Current balance, computed from `ledger_entries` |
