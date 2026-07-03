# 🛰️ Beacon — Self-Hosted Infrastructure Monitoring Platform

Beacon is an open, self-hosted monitoring platform in the spirit of Better Stack,
Uptime Kuma, Pingdom and Grafana Cloud — but it **manages battle-tested tooling
(Prometheus, Blackbox, Alertmanager) as a control plane** instead of reinventing
probing. You manage everything from a dashboard; Beacon generates and hot-reloads
the underlying config. A non-technical user can add a website and get alerted when
it goes down, without ever touching Prometheus or YAML.

> **Status — Session 1 vertical slice.** This is the first increment of a much
> larger platform, built end-to-end so the whole stack works: **Auth →
> Organizations/Projects → Website/API/SSL/TCP/Ping/DNS monitoring → Prometheus
> control plane → alerting**, plus the dashboard. It compiles, is tested, and runs
> with one command. See [docs/ROADMAP.md](docs/ROADMAP.md) for what comes next
> (Telegram & other notifications, servers/Kubernetes, incidents, RBAC/teams, …).

---

## What works today

- **Authentication** — register (creates an organization + owner), login, JWT
  access tokens with rotating, hashed refresh tokens, `/me`, logout. bcrypt
  password hashing; role-based write protection (owner/admin/member/viewer).
- **Projects** — CRUD, environment labels (production/staging/development),
  per-organization isolation, soft delete (cascades to monitors).
- **Monitors** — HTTP, HTTPS, SSL certificate, TCP port, ICMP ping and DNS. Create
  from the UI; Beacon validates input, then **generates a Blackbox module, a
  Prometheus scrape job and alert rules per monitor and hot-reloads both
  services**. Pause/resume and delete update the control plane automatically.
- **Live status & analytics dashboard** — the worker reads probe results from
  Prometheus every 30s; the dashboard shows KPI tiles (uptime %, avg response,
  up/down, active alerts), **availability + response-time charts** (24h), and
  per-monitor **status bars** — all org-scoped.
- **Plans & billing** — a pricing page (`free`/`starter`/`pro`) where owners
  switch plans; limits update immediately. (Self-serve switch now; drop-in point
  for Stripe Checkout.)
- **Alerting** — auto-generated rules: *MonitorDown*, *SSLCertExpiringSoon* (default
  30 days), *SlowResponse* (when a response-time threshold is set).
- **Telegram notifications** — add a channel (bot token + chat id, token encrypted
  at rest), send a test message, and receive **rich firing + recovery alerts**
  (severity, monitor, target, project, environment, duration, dashboard link).
  Alertmanager forwards every alert to Beacon's webhook, which routes to the org's
  channels.
- **Control plane** — the whole Prometheus/Blackbox config is a projection of the
  database, regenerated idempotently on every change and reconciled periodically by
  a crash-resilient worker.
- **Platform** — structured JSON logging with request/correlation IDs, centralized
  error handling, Prometheus self-metrics at `/metrics`, liveness/readiness probes,
  a reliable Redis-Streams job queue, graceful shutdown, and DB migrations.
- **Dashboard** — Next.js (App Router) + TanStack Query + React Hook Form + Zod +
  Tailwind: login/register, projects, and a live monitors table with create/pause/
  resume/delete. Light & dark aware.

---

## Quick start (Docker)

Requires Docker with Compose v2.

```bash
make up          # or: docker compose -f deploy/docker-compose.yml up -d --build
```

This starts Postgres, Redis, Prometheus, Blackbox, Alertmanager, the Beacon API,
the Beacon worker, and the dashboard. The API applies migrations on startup.

Everything is served through a single **nginx gateway** — open one URL:

### → http://localhost:8090

| Path                                | Serves                          |
|-------------------------------------|---------------------------------|
| `/`                                 | Dashboard                       |
| `/api/…`                            | Beacon API (same origin, no CORS) |
| `/prometheus/`                      | Prometheus UI                   |
| `/alertmanager/`                    | Alertmanager UI                 |
| `/blackbox/`                        | Blackbox exporter               |

**Multi-tenant by design.** Every user sees only their own organization's data —
including in the *raw tools*:

- The dashboard's **Alerts** page and per-monitor **Metrics** are read from
  Prometheus filtered by your `org_id`.
- The **raw Prometheus and Alertmanager UIs** (System page) are fronted by
  [`prom-label-proxy`](https://github.com/prometheus-community/prom-label-proxy):
  the gateway authenticates your session (an httpOnly `beacon_proxy` cookie),
  derives your `org_id`, and the proxy injects `{org_id="<you>"}` into **every**
  PromQL query and filters all rules/alerts. A wildcard query like `probe_success`
  returns only your series, and attempts to query another tenant's `org_id` are
  rejected with a 400. Blackbox has no query API and stays operator-only.

Postgres `:5432` and Redis `:6379` are also published for local debugging.

**Per-tenant quotas.** Each organization is on a plan (`free` / `starter` / `pro`)
that caps its **monitor count** and **minimum check interval**, protecting the
shared Prometheus/Blackbox from a single noisy tenant. Limits live in code
(`internal/domain/plan`) and are enforced on create/update (a violation returns
`402 quota_exceeded`); the Monitors page shows live usage (`used / limit`). Plans
default to `free`; a billing integration would set the `organizations.plan` column.

| Plan | Max monitors | Min interval |
|------|--------------|--------------|
| free | 10 | 60s |
| starter | 50 | 30s |
| pro | 500 | 10s |

Then, in the dashboard:

1. **Create account** (this creates your organization).
2. **Projects → New project** (e.g. "Production").
3. **Monitors → Add monitor** → pick *HTTPS website*, paste a URL, choose an
   interval, **Create**.
4. Within a few seconds the worker regenerates config and reloads Prometheus &
   Blackbox. Open Prometheus → *Status → Targets* to see the new `mon_…` job, and
   *Alerts* to see the generated rules. Kill/point the monitor at a bad host to
   watch `MonitorDown` fire into Alertmanager.
5. **Telegram alerts (optional):** create a bot with `@BotFather`, get your chat id
   from `@userinfobot`, then **Notifications → Add Telegram**, paste both, **Save**,
   and **Send test**. When a monitor goes down you'll get a rich alert, and a ✅
   recovery message when it comes back.

Tear down with `make down`.

### Port already in use?

Only **one** web port is published now (the gateway), so coexisting with other
stacks is easy — just move it. Set `BEACON_GATEWAY_PORT` (in `deploy/.env` or
inline):

```bash
BEACON_GATEWAY_PORT=9000 make up      # everything now on http://localhost:9000
```

If you change the gateway port, `make up` rebuilds the frontend and updates the
Prometheus/Alertmanager external URLs automatically. `BEACON_POSTGRES_PORT` and
`BEACON_REDIS_PORT` are also overridable. Defaults live in `deploy/.env.example`.

### Monitoring a service on the same machine (`localhost`)

Probes run **inside the Blackbox container**, so a target of `http://localhost`
means "the container's own port 80", not your host — it will show as **down**.
To monitor a service on the Docker host, target **`http://host.docker.internal`**
(Docker Desktop on macOS/Windows). On Linux, add
`extra_hosts: ["host.docker.internal:host-gateway"]` to the `blackbox` service
first. Public URLs (real domains) work from anywhere and need none of this.

## Quick start (local, without Docker)

You still need Postgres and Redis (the compose file is the easiest way to get
them). Then:

```bash
cd backend
cp .env.example .env          # adjust DSNs if needed; set real secrets
set -a && source .env && set +a
go run ./cmd/api              # applies migrations, serves on :8080
go run ./cmd/worker           # in another terminal
```

Frontend:

```bash
cd frontend
npm install
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev   # :3000
```

---

## Repository layout

```
backend/                 Go (clean architecture, DDD)
  cmd/api                REST API + `migrate` subcommand
  cmd/worker             background worker (control-plane reconcile, cleanup)
  internal/
    config               env-driven, validated configuration
    domain/              bounded contexts: auth, project, monitor, audit
                         (entities + repository interfaces + services)
    adapter/             interface implementations:
      postgres           repositories (hand-written, parameterized SQL)
      controlplane       Prometheus/Blackbox config generation + reload
      queue              Redis-Streams job queue + sync enqueuer
    platform/            cross-cutting infra: logger, database, cache, crypto,
                         validate, httpx, metrics, apperror, slug
    transport/rest       chi handlers, DTOs, middleware
    worker               periodic task scheduler
  migrations/            versioned SQL migrations (embedded)
deploy/                  docker-compose, Prometheus/Blackbox/Alertmanager config,
                         Dockerfiles
frontend/                Next.js dashboard
docs/                    architecture, API, roadmap
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the design in depth and
[docs/API.md](docs/API.md) for the endpoint reference.

---

## Development

```bash
make help          # list targets
make build         # build api + worker
make test          # run the Go test suite
make check         # fmt + vet + test
make run-api       # run the API locally
make migrate-status
```

### Testing note (macOS)

On recent macOS the Go internal linker can emit test binaries that the system
loader rejects (`missing LC_UUID … signal: abort trap`). `make test` works around
this by forcing external linking on Darwin; this has no effect on Linux/CI.

---

## Design highlights

- **Control plane, not a reimplementation.** Beacon never probes targets itself; it
  drives Prometheus + Blackbox. The generated config is a pure function of the
  database, so it is always consistent and reconciliation is idempotent.
- **Clean architecture / DDD.** Domain services depend only on interfaces; all
  infrastructure (Postgres, Redis, Prometheus, HTTP) lives behind adapters. This is
  what makes the domain unit-testable without a database.
- **Security.** Parameterized SQL, bcrypt, rotating hashed refresh tokens,
  role-gated writes, per-tenant scoping on every query, strict JSON decoding with
  body limits, AES-256-GCM available for secret encryption, config validation that
  refuses insecure defaults in production.
- **Operability.** JSON logs with request/correlation IDs, `/metrics`, `/livez`,
  `/readyz`, graceful shutdown, crash-resilient queue with retries + dead-letter.

## License

Provided as-is for the requested build. Add a license of your choosing before
distribution.
