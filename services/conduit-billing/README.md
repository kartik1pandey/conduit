# conduit-billing

Meters API calls per merchant and computes a tiered invoice on a schedule.
Lower priority per `docs/CHECKPOINTS.md` — the first thing to cut if time
runs short, since Phases 0–3 already form a complete, defensible system on
their own.

## Running locally

```bash
# from the repo root
docker compose up -d postgres

export BILLING_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_billing
export INTERNAL_JWT_SECRET=local-dev-secret-not-real

go run ./cmd/migrate
go run ./cmd/server    # serves on :8004
```

## Testing

```bash
export BILLING_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_billing
export INTERNAL_JWT_SECRET=local-dev-secret-not-real
go test ./...
```

`internal/billing/pricing_test.go`'s cases are hand-calculated, not derived
from running the code — that's the literal Checkpoint 4.2 verification.

## Generating invoices

```bash
export BILLING_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_billing
export INTERNAL_JWT_SECRET=local-dev-secret-not-real
go run ./cmd/generate-invoices              # bills the previous calendar month
go run ./cmd/generate-invoices -period 2026-07   # bills a specific month
```

This is a plain CLI command, not an in-process scheduler — the "run this on
the 1st of every month" part is an external cron entry (or a PaaS's own
scheduled-task feature) invoking this binary, matching every other
"scheduled job" description in `docs/ARCHITECTURE.md`. The Docker image
builds both `server` and `generate-invoices` into the same image; a
deployment runs `server` as the long-lived container and invokes
`generate-invoices` via a separate scheduled task pointed at the same
image, overriding the entrypoint.

Re-running for an already-invoiced period is a safe no-op per merchant
(logged, not an error) — an invoice is never overwritten once created.

## API

Requires `Authorization: Bearer <internal JWT>`, signed by conduit-core.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | 200 if Postgres is reachable, 503 otherwise |
| `POST` | `/v1/usage/record` | Increments the authenticated merchant's call counter for the current month |
| `GET` | `/v1/usage/current` | The authenticated merchant's usage for the current month |
| `GET` | `/v1/invoices/{period}` | An invoice for a given month (`period` as `YYYY-MM-DD`, the first day of that month) |
