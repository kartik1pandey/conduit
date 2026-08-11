# conduit-dashboard

Merchant-facing dashboard: transactions, webhook delivery status, and risk
decisions with their reasons, plus login and `owner`/`developer`/`read-only`
role-based access control. Next.js (App Router) + TypeScript.

## Running locally

```bash
# from the repo root
docker compose up -d postgres

# create the database once (compose's postgres-init script only runs on a
# fresh volume — see infra/postgres-init/01-create-databases.sql)
psql postgres://conduit:conduit@localhost:5432/postgres -c "CREATE DATABASE conduit_dashboard;"

export DASHBOARD_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_dashboard
export AUTH_SECRET=local-dev-secret-not-real
export AUTH_GITHUB_ID=your-github-oauth-app-id       # optional for credentials-only testing
export AUTH_GITHUB_SECRET=your-github-oauth-app-secret
export CONDUIT_CORE_URL=http://localhost:8000
export DASHBOARD_SESSION_SECRET=local-dev-secret-not-real   # must match conduit-core's

npm install
npm run migrate
npm run dev    # serves on :3000
```

## Testing

```bash
export DASHBOARD_DATABASE_URL=postgres://conduit:conduit@localhost:5432/conduit_dashboard
npm test                    # vitest — unit + integration, skips cleanly if DASHBOARD_DATABASE_URL/
                             # DASHBOARD_OPA_URL/CONDUIT_CORE_URL are unset, same pattern every
                             # other service's integration tests use
npm run test:e2e            # playwright — drives a real browser against the full running stack
                             # (docker compose up -d), including the real Checkpoint 4.3 scenario:
                             # a read-only session gets 403 on a refund action
```

## Authentication

Two ways to log in:

- **Email/password** — argon2id-hashed, verified against this service's
  own `users` table.
- **GitHub OAuth** — only links to an _existing_ dashboard account matched
  by email; there's no fresh signup via OAuth. A dashboard account is
  created either by [claiming a merchant](#signup) or by an owner's invite.

### Signup

The first dashboard user for a merchant proves ownership by supplying the
merchant's `sk_test_...` secret key once (verified against conduit-core's
`POST /v1/merchants/verify-secret`, never stored raw) and becomes that
merchant's `owner`. An owner can then invite `developer`/`read-only`
teammates from `/dashboard/team`, optionally setting an initial password
(there's no email-sending infrastructure in this test-mode project) or
leaving it blank for a GitHub-only account.

### Authorization

Every gated action (`refund`, `invite_user`) is checked against a real OPA
server evaluating `policies/dashboard.rego` — the same policy-engine
pattern `docs/ARCHITECTURE.md` calls for reusing from conduit-risk, on its
own separate OPA instance (this service stays self-contained; it doesn't
share conduit-risk's). The check happens server-side in the action itself,
not just as a hidden UI element — see
`src/app/dashboard/transactions/[id]/actions.ts`.

## Authorization boundary with conduit-core

conduit-core accepts two independent credentials, resolving both to the
same merchant-scoped request context:

- `Authorization: Bearer sk_test_...` — a merchant's own API key.
- `X-Dashboard-Session: <jwt>` — a 60-second JWT this service signs per
  request with `DASHBOARD_SESSION_SECRET`, verified by
  `authn.RequireMerchantContext` on the core side.

This service never stores a merchant's raw secret key past the one-time
signup verification — it can't use it later even if it wanted to, by
design.

## Pages

| Path                           | Description                                                   |
| ------------------------------ | ------------------------------------------------------------- |
| `/signup`                      | Claim a merchant via its secret key, create the owner account |
| `/login`                       | Email/password or GitHub                                      |
| `/dashboard`                   | Overview: signed-in user, merchant, role                      |
| `/dashboard/transactions`      | Payment intents, newest first                                 |
| `/dashboard/transactions/[id]` | Detail + refund action (role-gated)                           |
| `/dashboard/webhooks`          | Registered webhook endpoints                                  |
| `/dashboard/webhooks/[id]`     | Delivery log for one endpoint                                 |
| `/dashboard/risk`              | Risk decisions with their reasons                             |
| `/dashboard/team`              | Team members; invite form (owner-only)                        |
| `/health`                      | 200 if Postgres is reachable, 503 otherwise                   |
