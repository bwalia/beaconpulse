# Beacon Architecture

## Guiding principles

1. **Beacon is a control plane.** It does not implement probing, alerting or
   time-series storage. It manages Prometheus, Blackbox and Alertmanager by
   generating their configuration from the database and hot-reloading them. This
   buys battle-tested reliability and lets us focus on UX and orchestration.
2. **The database is the single source of truth.** All generated monitoring config
   is a pure, idempotent projection of it. Any reconcile rebuilds the *entire* set
   of scrape jobs / modules / rules, so there is no drift to repair incrementally.
3. **Clean architecture + DDD.** Dependencies point inward. Domain code depends
   only on interfaces it defines; infrastructure implements them. Nothing in a
   domain package imports a framework, a driver, or another adapter.

## Layers and the dependency rule

```
        transport/rest ─────────────┐         cmd/api, cmd/worker
        (chi handlers, DTOs)        │         (composition root: DI wiring)
                │                    │                 │
                ▼                    ▼                 ▼
        domain/{auth,project,monitor,audit}  ◄──────────────
        (entities, services, repository INTERFACES)
                ▲                    ▲
                │                    │
        adapter/{postgres,           platform/{config,logger,database,cache,
                 controlplane,queue}  crypto,validate,httpx,metrics,apperror,slug}
        (implement the interfaces)   (cross-cutting infrastructure)
```

- **domain/** — the heart. Each bounded context owns its aggregate(s), a `Service`
  with the use cases, and the repository interface(s) it needs. Pure Go; fully
  unit-tested with in-memory fakes.
- **adapter/** — concrete implementations: `postgres` (repositories),
  `controlplane` (config generation + reload), `queue` (Redis Streams).
- **transport/rest** — thin HTTP layer: decode → validate → call a service →
  present. No business logic.
- **platform/** — reusable infrastructure with no domain knowledge.
- **cmd/** — the composition root. `buildRouter`/`main` construct adapters, inject
  them into services, and wire handlers. Dependency injection is explicit and
  constructor-based; there is no global state or service locator.

## Request lifecycle (API)

```
HTTP request
  → RequestID          assign/propagate X-Request-ID, X-Correlation-ID
  → CORS               dashboard origin policy
  → Logging            request-scoped slog logger; one structured line per request
  → Metrics            beacon_http_* counters/histograms by route
  → Recover            convert panics into a logged 500
  → auth.Require       verify Bearer access token → Principal in context
  → handler            decode + validate DTO → domain service
      → service        authorize, enforce invariants, call repository, audit
        → postgres     parameterized SQL, tenant-scoped
      → (monitor/project mutations) enqueue control-plane sync
  → httpx envelope     consistent JSON success/error, request_id echoed
```

Errors are `*apperror.Error` with a stable machine `Code`; the transport maps them
to status codes and a uniform `{ "error": { code, message, fields, request_id } }`
body. 5xx causes are logged with the request id; clients see a generic message.

## Control-plane reconcile

```
UI: add/edit/pause/delete monitor
        │
        ▼
monitor.Service ── enqueue "controlplane.sync" ──► Redis Stream (beacon:jobs)
                                                          │
                                            worker Consumer (group, XREADGROUP,
                                            XAUTOCLAIM reclaim, retries, DLQ)
                                                          │
                                                          ▼
                                       controlplane.Syncer.Sync():
                                         1. ListAllEnabled() from Postgres
                                         2. Generate():
                                            - blackbox.yml   (1 module / monitor)
                                            - scrape_*.yml   (1 scrape job / monitor)
                                            - rules_*.yml    (MonitorDown, SSL, slow)
                                         3. writeAtomic (temp file + rename)
                                         4. POST /-/reload to Prometheus & Blackbox
        │
        └── also: worker runs a full resync every 2 min (safety net) and on boot,
            so the control plane converges even if a job is lost.
```

Each monitor maps to a Blackbox module derived from its type and settings (HTTP
method, valid status codes, body keyword regex, TLS-verify, headers; TCP; ICMP;
DNS query name/type). The scrape job passes `module` and `target` as params to
Blackbox `/probe` and attaches labels (`monitor_id`, `monitor_name`, `project_id`,
`org_id`, `instance`) that flow into every metric and alert.

`writeAtomic` guarantees Prometheus/Blackbox never read a half-written file. A
mutex serializes concurrent syncs within a process.

## Data model

All tables use UUID PKs, `created_at`/`updated_at` (trigger-maintained), and
soft-delete via `deleted_at`; unique constraints are partial on `deleted_at IS
NULL` so names are reusable after deletion. `created_by`/`updated_by` provide an
audit trail.

- **organizations** — tenant boundary.
- **users** — belong to one org; role ∈ {owner, admin, member, viewer}; unique
  email; bcrypt `password_hash`; 2FA columns reserved.
- **refresh_tokens** — HMAC-hashed opaque tokens, revocable, rotated on refresh.
- **projects** — org-scoped groupings; environment label.
- **monitors** — org- + project-scoped; `type`, `target`, `interval/timeout`,
  type-specific `config` JSONB, cached `last_status`.
- **audit_logs** — append-only action log for the Audit page.

Migrations are hand-written SQL (never auto-generated), embedded into the binary,
and applied transactionally in version order.

## Concurrency & resilience

- **Job queue** — Redis Streams with a consumer group. Unacked messages are
  reclaimed via `XAUTOCLAIM` after an idle window (surviving a crashed worker) and
  dead-lettered after `MaxRetries`. Handlers are idempotent.
- **Graceful shutdown** — SIGINT/SIGTERM cancels the root context; the API drains
  in-flight requests within a timeout; the worker stops its consumer and scheduler.
- **DB pool** — pgxpool sized by config; readiness probe pings DB and Redis.

## Why these choices

| Concern            | Choice                            | Rationale                                   |
|--------------------|-----------------------------------|---------------------------------------------|
| Probing            | Prometheus + Blackbox             | Don't reinvent proven protocols             |
| Dynamic config     | `scrape_config_files` + generated | Add monitors without editing base config    |
| Refresh tokens     | opaque + HMAC hash, rotated       | Revocable, unforgeable, no plaintext at rest|
| Secrets            | AES-256-GCM helper                | Authenticated encryption for credentials    |
| Errors             | typed `apperror` + central mapper | Uniform API, no internal leakage            |
| Async work         | Redis Streams consumer group      | Crash-resilient, retryable, at-least-once   |
```
