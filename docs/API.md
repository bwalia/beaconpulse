# Beacon API (v1)

Base URL: `http://localhost:8080`. All application endpoints are under `/api/v1`.
Requests and responses are JSON. Authenticated endpoints require an
`Authorization: Bearer <credential>` header, where the credential is either a session
access token (dashboard) or an **API key** (machines — see below). Both resolve to the
same principal, so every endpoint accepts either and applies the same org scoping,
role checks and plan limits to both.

## Conventions

- **Success**: the resource or, for collections, `{ "data": [...], "pagination": {
  total, limit, offset } }`.
- **Error** (all non-2xx): `{ "error": { "code", "message", "fields"?, "request_id" } }`
  where `code` ∈ `validation | unauthorized | forbidden | not_found | conflict |
  rate_limited | internal | unavailable`. `fields` lists per-field validation
  messages.
- **Pagination**: `?limit=` (default 50, max 200) and `?offset=`.
- **Tenancy**: every resource is scoped to the caller's organization; other tenants'
  resources return `404`.
- **Roles**: mutations require a writer role (owner/admin/member); `viewer` is
  read-only.

## Operational (unauthenticated)

| Method | Path       | Description                                   |
|--------|------------|-----------------------------------------------|
| GET    | `/livez`   | Liveness — always 200 while running.          |
| GET    | `/readyz`  | Readiness — 200 only if Postgres + Redis OK.  |
| GET    | `/healthz` | Version and uptime.                           |
| GET    | `/metrics` | Prometheus metrics for the API itself.        |

## Auth

| Method | Path                   | Auth | Body / Notes |
|--------|------------------------|------|--------------|
| POST   | `/api/v1/auth/register`| —    | `{ org_name, name, email, password }` → creates org + owner, returns tokens. |
| POST   | `/api/v1/auth/login`   | —    | `{ email, password }` → tokens. |
| POST   | `/api/v1/auth/refresh` | —    | `{ refresh_token }` → new tokens (old refresh token is revoked). |
| POST   | `/api/v1/auth/logout`  | —    | `{ refresh_token }` → 204. |
| GET    | `/api/v1/me`           | ✓    | Current user. |

Auth response:

```json
{
  "access_token": "…jwt…",
  "refresh_token": "…opaque…",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": { "id","org_id","email","name","role","is_active","twofa_enabled","created_at" }
}
```

## API keys

Machine credentials. A key is an opaque secret that RESOLVES to an organization; it
carries no plan or balance of its own, so limits and credit are read live on every
request and an upgrade or top-up needs no new key.

| Method | Path                    | Auth      | Body / Notes |
|--------|-------------------------|-----------|--------------|
| GET    | `/api/v1/api-keys`      | session   | List. Secrets are not stored, so none are returned. |
| POST   | `/api/v1/api-keys`      | session   | `{ name, role?, expires_in_days? }` → `{ key, secret }`. **`secret` is returned once and is unrecoverable.** |
| DELETE | `/api/v1/api-keys/{id}` | session   | Revoke. Idempotent. |

- **Session-only, by design.** An API key cannot manage API keys, so a leaked key
  cannot mint a successor that survives revoking the original.
- `role` defaults to the creator's and is capped at it — a key can never out-rank the
  person who made it. Use `viewer` for read-only automation.
- Owner/admin only.

## Sync (declarative)

| Method | Path            | Auth | Body / Notes |
|--------|-----------------|------|--------------|
| POST   | `/api/v1/sync`  | ✓ writer | `{ project?, monitors[], prune?, dry_run? }` → per-item plan/outcome. |

Applies a desired set of monitors: creates what is new, updates what changed, leaves
the rest alone. **Idempotent** — safe to run on every CI push, which a plain `POST
/monitors` is not (it would duplicate on every re-run).

- `prune` defaults to **false**: undeclared monitors are *reported* (`would_remove`),
  never deleted. A workflow whose glob breaks declares zero monitors, and that must not
  wipe production monitoring.
- `dry_run` computes the plan and applies nothing.
- Returns **200 with per-item errors** rather than failing wholesale, so one rejected
  monitor does not discard the rest. **Check `failed` in the body**, not just the status.

See [AUTOMATION.md](AUTOMATION.md) for the GitHub Actions workflow.

## Projects

| Method | Path                    | Auth | Notes |
|--------|-------------------------|------|-------|
| GET    | `/api/v1/projects`      | ✓    | `?search=&environment=&limit=&offset=` |
| POST   | `/api/v1/projects`      | writer | `{ name, description?, environment?, is_active? }` |
| GET    | `/api/v1/projects/{id}` | ✓    | |
| PATCH  | `/api/v1/projects/{id}` | writer | Partial: any of `name, description, environment, is_active` |
| DELETE | `/api/v1/projects/{id}` | writer | Soft delete; cascades to the project's monitors. |

## Monitors

| Method | Path                            | Auth | Notes |
|--------|---------------------------------|------|-------|
| GET    | `/api/v1/monitors`              | ✓    | `?project_id=&type=&status=&enabled=&search=&limit=&offset=` |
| POST   | `/api/v1/monitors`              | writer | see below |
| GET    | `/api/v1/monitors/{id}`         | ✓    | |
| PATCH  | `/api/v1/monitors/{id}`         | writer | Partial update; re-syncs control plane |
| DELETE | `/api/v1/monitors/{id}`         | writer | Soft delete; stops probing |
| POST   | `/api/v1/monitors/{id}/pause`   | writer | Disable + re-sync |
| POST   | `/api/v1/monitors/{id}/resume`  | writer | Enable + re-sync |

Create body:

```json
{
  "project_id": "uuid",
  "name": "Marketing site",
  "type": "http | https | ssl | tcp | icmp | dns",
  "target": "https://example.com",       // URL, host:port (tcp), host (icmp), or domain (dns)
  "interval_seconds": 60,                  // 10–86400, default 60
  "timeout_seconds": 10,                   // 1–300, ≤ interval, default 10
  "settings": {
    "method": "GET",                       // http/https/ssl
    "valid_status_codes": [200],
    "body_keyword": "OK",                  // body must contain
    "follow_redirects": true,
    "skip_tls_verify": false,
    "ssl_expiry_warning_days": 30,         // alert threshold (https/ssl)
    "response_time_warning_ms": 2000,      // slow-response alert threshold
    "dns_query_name": "example.com",       // dns
    "dns_query_type": "A"                  // A|AAAA|CNAME|MX|TXT|NS|SOA|CAA
  }
}
```

Creating, updating, pausing, resuming or deleting a monitor enqueues a
control-plane sync; within seconds the worker regenerates the Blackbox module,
Prometheus scrape job and alert rules for it and reloads both services.

## Notification channels

Channel secrets (e.g. a Telegram bot token) are **encrypted at rest** and never
returned; responses expose only a `has_secret` boolean.

| Method | Path                                       | Auth | Notes |
|--------|--------------------------------------------|------|-------|
| GET    | `/api/v1/notification-channels`            | ✓    | `{ "data": [channel…] }` |
| POST   | `/api/v1/notification-channels`            | writer | create (see below) |
| GET    | `/api/v1/notification-channels/{id}`       | ✓    | |
| PATCH  | `/api/v1/notification-channels/{id}`       | writer | partial; a non-empty `secret` replaces the credential |
| DELETE | `/api/v1/notification-channels/{id}`       | writer | soft delete |
| POST   | `/api/v1/notification-channels/{id}/test`  | writer | delivers a test message immediately |

Create body (Telegram):

```json
{
  "name": "Ops Telegram",
  "type": "telegram",
  "config": { "chat_id": "123456789" },
  "secret": "123456:ABC-DEF…"        // bot token; stored encrypted
}
```

Supported `type` today: `telegram` (others accepted by the schema, rejected by the
service until their notifier ships).

## Alertmanager webhook (internal)

| Method | Path                       | Auth | Notes |
|--------|----------------------------|------|-------|
| POST   | `/api/v1/alerts/webhook`   | shared secret | Alertmanager delivers alerts here; not JWT-authed |

Authenticated by a bearer token that must equal `BEACON_WEBHOOK_TOKEN` (matching
the credentials in `deploy/alertmanager/alertmanager.yml`). Beacon parses the
Alertmanager payload, attributes each alert to an organization via its `org_id`
label, and dispatches firing/resolved notifications to that org's enabled
channels.
