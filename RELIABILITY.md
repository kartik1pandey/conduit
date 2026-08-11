# Reliability: how idempotency and chaos testing were actually verified

This is a portfolio artifact, not a design doc — it documents what was
*proven*, with the specific test that proved it, not just what was
*intended*. Full architecture: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
(kept local, not pushed — this file is the public summary of the parts
that matter for reliability specifically).

## Idempotency

Every write endpoint in Conduit requires an `Idempotency-Key` header. This
isn't a checkbox — it's the mechanism that makes retries (a client's own
retry logic, a load balancer replaying a request after a timeout, a
crashed process resuming) safe by construction instead of "safe as long as
nothing goes wrong."

**The pattern:** claim → do the work → fill.

```sql
INSERT INTO idempotency_keys (merchant_id, key, status)
VALUES ($1, $2, 'in_progress')
ON CONFLICT DO NOTHING
RETURNING id
```

If the insert returns a row, this request won the race and proceeds. If it
doesn't, another request (or a retry of this same request) already claimed
this key — the response is either the original result (if it finished) or
a 409 telling the client to retry after a short lease window (if the
original attempt is still in flight, or crashed before finishing). A
30-second lease reclaims a key whose owning request died mid-flight,
so a crash doesn't permanently wedge that idempotency key.

**Why the response body is stored as `BYTEA`, not `JSONB`.** A replayed
request must get back the *exact* original response, byte for byte —
Postgres reformats JSON on round-trip through a `JSONB` column (key
order, whitespace), which would make two calls with the same key return
textually different (though semantically equivalent) bodies. Storing the
raw response bytes instead means replay is provably identical, not just
"close enough."

**What's actually tested, not just asserted:**
- `TestRequireKey_SecondRequestReplaysFirstResponseWithoutRerunningHandler`
  (conduit-core) sends the same request twice with the same key, asserts
  the two response bodies are byte-identical *and* that the underlying
  handler ran exactly once — not just "the responses look the same," but
  "the second request never did the work at all." `TestClaimAndFill_FreshKeyThenReplay`
  and `TestClaim_InFlightKeyBlocksConcurrentRetry` cover the same property
  one layer down, at the store, including the concurrent-in-flight case.
- `TestPaymentIntentLifecycle`'s "re-confirming an already-succeeded intent
  is a no-op, not a second ledger post" and the equivalent refund test
  assert the *ledger call count*, not just the HTTP response — a retried
  confirm/refund must never post a second transaction, which is a
  stronger claim than "the API looks idempotent."
- `TestConfirm_RiskDeclineBlocksChargeWithNoLedgerEntry` is Checkpoint
  3.2's literal verification: force a decline, assert the payment intent
  ends `failed` *and* the ledger transaction count stays at zero.
- Idempotency-key lookups check Redis before Postgres (`idempotency.RequireKey`),
  with a 24-hour TTL — verified via `redis-cli TTL` showing expiry is
  actually set, and `TestFill_SetsRedisTTL` covering it as an automated
  assertion, not just a manual check.

## The ledger's own invariant: enforced by Postgres, not application code

Every ledger transaction's debit and credit entries must sum to zero. This
is enforced with a **deferred constraint trigger** — a database-level
check that only runs at `COMMIT`, after every entry in a transaction is
visible — not a plain `CHECK` constraint (which can't span multiple rows)
and not an application-level sum-and-compare (which a bug, a bypassed code
path, or a direct `psql` session could silently violate).

`TestPostTransaction_UnbalancedIsRejectedWithNoPartialWrite` proves both
halves of that claim in one test: posting a $100 debit against a $50
credit returns `ErrUnbalanced`, *and* a follow-up lookup by the same
idempotency key confirms zero rows were left behind — the rejection isn't
just an error return with a half-written transaction still sitting in the
table underneath it.

## Chaos testing

`services/conduit-webhooks/internal/webhook/chaos_test.go` is a real test
harness — `flakyServer` — that simulates the actual failure modes a
receiving webhook endpoint exhibits in production: dropped connections
(via `http.Hijacker`, closing the TCP connection mid-response), delayed
responses, and duplicate delivery (processing a request but dropping the
response, so the sender sees a timeout and retries something the receiver
already handled — the same "at least once delivery" ambiguity every
webhook system has to survive).

**What it actually proves, per scenario:**
- **Backoff behavior** (`TestChaos_RetriesWithBackoffThenSucceeds`): a
  fake, test-controlled clock is only advanced past each attempt's actual
  backoff-cap boundary before the worker is asked to process again — a
  retry that fired before its scheduled time would make the test's own
  `advancePast(Cap(attempt))` step meaningless, so this proves the delay
  schedule is honored, not just that a retry eventually happens.
- **Dead-letter transition** (`TestChaos_DeadLettersAfterMaxAttempts_NoFurtherAttemptsFire`):
  forcing `maxAttempts` consecutive failures moves the delivery to
  `dead_lettered`; advancing the clock far past that and processing again
  is asserted to fire zero further HTTP calls — checking the *absence* of
  a fourth attempt, not just the presence of the status field.
- **Signature validity** (`TestChaos_SucceedsOnFirstAttempt_SignatureVerifies`):
  the payload's `Conduit-Signature` header (HMAC-SHA256, `hmac.Equal` for
  constant-time comparison) is independently recomputed and checked by the
  test's own receiver, not just decoded — a real forgery attempt would
  fail this the same way a bad merchant-supplied signature would in
  production.

**A real bug this suite caught, not a hypothetical one:** `flakyServer`
itself had an index-out-of-range panic (indexing into its own configured
`behaviors` slice before checking bounds) — found by reading raw test
output during development, not by the test silently passing. Fixed before
it could mask what it was supposed to be testing.

This suite runs in CI on every PR (`test-go-services (conduit-webhooks)`),
against a real Postgres and Redis, not mocks — the same emphasis on
"prove it against the real thing" that this project applies everywhere:
the dashboard's RBAC checks are proven with a real headless browser
against a real deployed container (see
[services/conduit-dashboard/README.md](services/conduit-dashboard/README.md)),
conduit-risk's policy is proven against a real OPA server, and the ledger
invariant is proven against a real Postgres constraint — not against a
description of what any of them are supposed to do.

## Multi-tenancy

Every query in every service is scoped by `merchant_id` at the query
layer — "we checked auth at the top of the request" is explicitly treated
as insufficient on its own throughout this codebase. Cross-tenant access
returns 404, never 403 — a 403 would confirm the resource exists at all,
which is itself information leakage. Verified per resource type, each
with two real merchants created in the test and one's real credentials
asserted against the other's data — not spot-checked on one endpoint and
assumed to hold everywhere else:

- `TestGetIsScopedPerMerchant` (conduit-core) — Checkpoint 1.7's literal
  scenario: merchant B's context reading merchant A's payment intent
  returns `ErrNotFound`, the same error a nonexistent ID would produce.
- `TestDeliveriesAreScopedPerMerchant` and `TestListEndpointsIsScopedPerMerchant`
  (conduit-webhooks) — merchant B sees zero of merchant A's deliveries or
  endpoints, never a filtered view that hints at their existence.
- `test_risk_decisions_never_returns_another_merchants_history`
  (conduit-risk) — merchant B's risk-decision history is asserted to be
  exactly `[]` after merchant A has real scored events, not just "shorter."
- `merchant A's dashboard shows its own transaction and never merchant B's`
  and the read-only-refund-403 test (conduit-dashboard, Playwright, run
  against the real deployed container) close the loop up through the
  actual browser-facing UI, not just the API layer underneath it.
