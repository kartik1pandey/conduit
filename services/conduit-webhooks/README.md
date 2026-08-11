# conduit-webhooks

Delivers events (`payment.succeeded`, `payment.failed`, etc.) to
merchant-registered URLs, HMAC-SHA256 signed in a `Conduit-Signature` header
(mirroring Stripe's `Stripe-Signature` scheme). Failed deliveries retry with
exponential backoff plus full jitter; repeated failures move to a
`dead_lettered` state. Retry scheduling ("redeliver at time T") lives in
Redis; delivery history and status are durable in Postgres.

## Running locally

```bash
# from the repo root
docker compose up -d postgres redis

export WEBHOOKS_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_webhooks
export REDIS_URL=redis://localhost:6379
export INTERNAL_JWT_SECRET=local-dev-secret-not-real

go run ./cmd/migrate
go run ./cmd/server    # serves on :8003, retry worker runs on a 1s ticker
```

## Testing

```bash
export WEBHOOKS_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_webhooks
export REDIS_URL=redis://localhost:6379
go test ./...
```

Integration tests are skipped (not failed) if either env var is unset. The
chaos suite (`internal/webhook/chaos_test.go`) drives a flaky
`httptest.Server` through drops connections, response delays, and a
process-but-drop-response scenario against the real retry/dead-letter
worker — no real network or Docker dependency, so it runs as a normal `go
test` both locally and in CI.

## API

All endpoints require `Authorization: Bearer <internal JWT>`, signed by
conduit-core and carrying a `merchant_id` claim.

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | 200 if Postgres and Redis are both reachable, 503 otherwise |
| `POST` | `/v1/webhook_endpoints` | Register a URL; generates and returns that endpoint's HMAC secret once |
| `GET` | `/v1/webhook_endpoints/{id}/deliveries` | Delivery history for one endpoint |
| `POST` | `/v1/events` | Emit an event; idempotent on `idempotency_key`, schedules one delivery per registered endpoint |
