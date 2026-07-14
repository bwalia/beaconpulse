# Design 0001 — Notification channels, Heartbeat monitors, Maintenance windows

Status: **accepted** · Author: engineering · Supersedes: none

Three features, specified together because they share one spine: the alert
pipeline that already exists.

> **Guiding constraint.** None of these is allowed to introduce a second way of
> doing something the system already does. Alerts already flow
> `Prometheus rule → Alertmanager → Beacon webhook → Dispatcher → Notifier`.
> Every feature below plugs into that path rather than growing a parallel one.
> A second alerting path is how monitoring systems rot.

---

## Current state (verified in code, not assumed)

| Thing | Reality |
| --- | --- |
| Channel types **declared** | telegram, slack, discord, email, webhook, teams |
| Channel types **working** | **telegram only** (`SupportedTypes` map) |
| Monitor types **allowed by DB** | http, https, tcp, icmp, ssl, dns, domain, api, server, kubernetes, health |
| Monitor types **implemented** | http, https, ssl, tcp, icmp, dns (**6**) |
| Status flow | `StatusSync` worker queries `probe_success` → bulk-updates `monitors.last_status` |
| Alert flow | Prometheus rule → Alertmanager → `POST /api/v1/alerts/webhook` → `Dispatcher.DispatchAlerts` |
| Notifier contract | `Type() ChannelType` · `Send(ctx, ch Decrypted, msg Message) error` |
| Channel storage | non-secret `config` JSONB + AES-256-GCM `secret_encrypted` (never returned by the API) |
| API metrics | `platform/metrics` owns a `prometheus.Registry`, served at `/metrics`, scraped as job `beacon-api` |

That last row is load-bearing: **Beacon can already export its own metrics into
its own Prometheus.** Heartbeats are built on it.

---

## F1 — Notification channels (Slack, Email, Webhook)  ✅ SHIPPED

### Why first
The product is currently unusable for most teams, and the marketing overclaims.
This is not a feature; it is a defect.

### Design

Three new `Notifier` implementations behind the existing interface. **No changes
to `Dispatcher`, no changes to the alert path.** Register them in the existing
registry (`cmd/api/main.go`) and flip the `SupportedTypes` map.

| Channel | Config (JSONB, plaintext) | Secret (AES-256-GCM) |
| --- | --- | --- |
| `slack` | — | incoming webhook URL |
| `webhook` | `url`, `method` (POST default) | HMAC signing key (optional) |
| `email` | `host`, `port`, `from`, `to[]`, `starttls` | SMTP password |

The Slack webhook URL is a **secret**, not config: possession of it grants posting
rights to the channel. It goes in the encrypted column, and — like every secret —
is never returned by the API.

### 🔴 The part that matters: SSRF

`webhook` (and `slack`) let a **tenant supply a URL that our server will fetch.**
That is a server-side request-forgery primitive, and Beacon runs inside a cluster
with an internal network. Naively implemented, a tenant could point a webhook at:

- `http://169.254.169.254/latest/meta-data/iam/…` — cloud instance credentials
- `http://beacon-api:8080/…`, `http://postgres:5432` — internal services
- `http://127.0.0.1`, `http://[::1]`, `http://10.x`, `http://192.168.x` — loopback / RFC-1918
- `http://vault.vault.svc.cluster.local:8200` — the secret store itself
- A public hostname whose DNS **resolves** to any of the above (DNS-rebinding)

This is the single most important line of code in F1. The mitigation is a shared
**guarded HTTP client** (`internal/platform/safehttp`) used by every notifier that
fetches a tenant-controlled URL:

1. **Scheme allow-list** — `https` only (`http` allowed only when an env flag
   enables it for local dev). No `file://`, `gopher://`, etc.
2. **Resolve, then vet, then dial the resolved IP.** The check must happen on the
   IP we actually connect to, not the hostname — otherwise DNS rebinding walks
   straight past it. Implemented with a custom `DialContext` that resolves the
   host, rejects the connection if **any** resolved IP is in a blocked range, and
   dials the vetted IP.
3. **Blocked ranges:** loopback, link-local (incl. `169.254.169.254`), RFC-1918
   private, unique-local IPv6, multicast, unspecified, and IPv4-mapped IPv6.
   Allow-list override via env for operators who genuinely need an internal
   webhook (off by default).
4. **Re-vet on redirect.** A 302 to an internal address must be re-checked, not
   followed blindly. Cap redirects (≤3).
5. **Bounded everything:** 10 s timeout, response body capped at 64 KiB (we only
   read it to surface an error), no connection reuse across tenants.

`safehttp` is its own package with its own table-driven tests (v4/v6 loopback,
link-local, private, rebinding, redirect-to-internal). It is the security boundary
and is tested as one.

### Signing (webhook)
When a signing key is set, sign the **exact bytes** of the request body:

```
X-Beacon-Signature: t=<unix>,v1=<hex(hmac_sha256(key, "<t>." + body))>
X-Beacon-Event:     alert.firing | alert.resolved
```

Timestamp is inside the signed payload so a captured request cannot be replayed
against a later window. This is the Stripe-style scheme; document it so customers
can verify.

### Rendering
`Message` already carries everything (status, severity, monitor, project,
duration, dashboard URL). Each notifier renders natively:
- **Slack** → Block Kit (colour attachment by severity, fields, a dashboard button).
- **Email** → multipart text + minimal HTML; subject `[BEACON] <status> — <monitor>`.
- **Webhook** → stable JSON envelope (documented, versioned `"version": 1`).

### Validation & test
`Service.Create` already validates and `SendTest` already exists — both extend to
the new types for free. Add per-type config validation (a Slack URL must be a
Slack host; an SMTP port must be sane) so a misconfigured channel fails at create
time, not at 3 a.m. during an outage.

### Effort: **S.** The hard 20% is `safehttp`; the notifiers themselves are small.

---

## F2 — Heartbeat monitors (dead-man's switch)  ✅ SHIPPED

### Why
The one thing black-box probing **structurally cannot do.** A probe says the site
answers; it can never say the nightly backup silently stopped six weeks ago. It is
also the stickiest feature we can ship — once a line is in someone's crontab it
never leaves — and the cheapest, because it reuses the whole alert spine.

### Model — a monitor, not a new concept

A heartbeat **is** a monitor (`type = "heartbeat"`). It already has org, project,
interval, alert routing. This avoids a parallel object with its own dashboards,
its own status rollup, its own everything.

Two properties differ from a probed monitor, and the code must honour both:

1. **It is inbound (push), not probed (pull).** The generator MUST skip heartbeat
   monitors when emitting Blackbox modules and scrape jobs — there is nothing to
   probe. (One `switch` guard in `controlplane.Generate`.)
2. **Its liveness signal is a timestamp we record, not a `probe_success` series.**

### The ping URL is a capability

```
POST|GET  /api/v1/ping/{token}
```

- **`token` is NOT the monitor UUID.** UUIDs are enumerable and leak the internal
  id. It is a separate high-entropy opaque value (32 random bytes, base58url →
  ~43 chars), stored per monitor, rotatable.
- **Unauthenticated by design** — the URL *is* the credential (a "capability URL").
  This is the same model as Healthchecks.io, Cronitor, GitHub webhook URLs.
- Accepts **GET and POST** (cron/curl/wget/PowerShell all differ), returns `200`
  with a tiny body, fast. Never 500s on a valid token — a heartbeat endpoint that
  flakes is worse than none.
- Optional sub-actions, healthchecks-style, same token:
  - `/api/v1/ping/{token}` — success (the default)
  - `/api/v1/ping/{token}/fail` — the job ran and **failed** → alert immediately
  - `/api/v1/ping/{token}/start` — job started (enables duration measurement)
  MVP ships success + `/fail`; `/start` is a fast-follow.

### 🔑 How it reaches the alert pipeline (the design decision)

**Export a Prometheus gauge and let Prometheus alert on it — do NOT invent a
second alert path.**

```
beacon_heartbeat_last_ping_timestamp_seconds{monitor_id, org_id, project_id}
```

On a ping: persist `last_ping_at` to the DB **and** set the gauge to `now`. The
control-plane generator emits, for each heartbeat monitor, a rule of the same
shape as `MonitorDown`, carrying the same tenant labels (see the `ruleLabels`
fix in Design-adjacent work):

```yaml
- alert: HeartbeatMissed
  expr: time() - beacon_heartbeat_last_ping_timestamp_seconds{monitor_id="…"} > <interval + grace>
  for: 0s
  labels: { severity: critical, org_id: …, project_id: …, monitor_id: …, monitor_type: heartbeat }
```

The alert then flows through the **exact same** Alertmanager → webhook →
Dispatcher → Notifier path as everything else. Recovery (a ping arrives) resolves
it the same way. Heartbeats get maintenance-window suppression, AI enrichment,
grouping and every future channel **for free**, because they are not special.

### Durability — the one correctness trap

An in-memory gauge is lost on API restart, which would fire a false
`HeartbeatMissed` for every heartbeat the instant we deploy. Two rules close it:

1. **Write-through:** the ping handler persists `last_ping_at` to the DB *before*
   updating the gauge. The DB is the source of truth.
2. **Re-seed on boot:** on startup, load every heartbeat's `last_ping_at` from the
   DB and set its gauge, *before* the API begins serving `/metrics`. Restart is
   then invisible to alerting.

### Grace period
`grace_seconds` per monitor (default = one interval, min 30 s). Alert when
`now − last_ping > interval + grace`. This absorbs a slightly-late cron without
crying wolf.

### Scaling the ingest path
Pings are the only high-QPS surface here (though realistically low: jobs ping
every 1–60 min, not per second).

- Handler is **O(1):** token → monitor lookup on a **unique index**, one gauge set,
  one row update. No joins, no fan-out.
- **Rate-limit per token** (leaky bucket) so a leaked URL cannot be used to hammer
  the DB. A valid token pinging 1000×/s is abuse, not a heartbeat.
- Scaling path if pings ever get hot (documented, **not** built now): debounce DB
  writes (gauge is live; flush `last_ping_at` every N seconds per monitor). Only
  worth it past thousands of pings/sec. MVP writes through — correct and simple.

### Enumeration / privacy
- The token space is 256-bit; brute force is infeasible.
- A wrong token returns `404` in **constant time** (no "valid but disabled" vs
  "unknown" distinction), so the endpoint is not an oracle.
- Rotating a token invalidates the old URL immediately.

### Effort: **M.** Ingest endpoint + gauge + generator guard + re-seed + rule.

---

## F3 — Maintenance windows

### Why now (not later)
We just shipped public status pages. Without maintenance windows, every planned
deploy **pages the whole on-call rotation** and makes the customer's public status
page scream **"Major outage"** at *their* customers during routine work. The
feature we just launched actively damages trust until this exists.

### Model

```
maintenance_windows
  id            uuid pk
  org_id        uuid  → organizations (fk, cascade)
  title         text  (shown on the status page)
  description   text
  starts_at     timestamptz
  ends_at       timestamptz   (CHECK ends_at > starts_at)
  scope         text  CHECK IN ('org','project','monitor')
  scope_ids     uuid[]        (project ids or monitor ids; empty for 'org')
  created_by / updated_by / timestamps / deleted_at
```

- `scope='org'` → every monitor. `scope='project'` → monitors in the listed
  projects. `scope='monitor'` → the listed monitors. One model, three grains.
- **One-off only in v1.** Recurrence (RRULE) is a real feature with real edge
  cases (DST, "third Tuesday") and is explicitly deferred to a v2, noted here so
  the column set does not have to change to add it later.

### Effect 1 — suppress notifications (the important one)

Suppress at the **Dispatcher**, in one place, before `Send`:

```
Dispatcher.DispatchAlerts:
    for each alert event:
        if maintenance.IsSuppressed(ctx, orgID, monitorID, event.Time):
            record "suppressed_by_maintenance" (audit/metric)   # visible, not silent
            continue
        … existing dispatch …
```

Why here and not Alertmanager silences:
- **One source of truth.** Suppression then covers *every* alert source uniformly —
  probed monitors and heartbeats alike — with no Alertmanager API to keep in sync.
- **Observable.** A suppressed alert is recorded, not vanished. "We had 3 alerts
  during the window, all suppressed" is answerable.
- The check is a single indexed query: "is there an active window at time T whose
  scope covers this monitor?" Cache per dispatch batch.

### Effect 2 — reflect on the status page
The public status projection (`domain/statuspage`) joins active windows. A monitor
covered by an active window renders **"Under maintenance"** (a distinct, neutral
state — not up, not down) and the page shows a **"Scheduled maintenance"** banner
with the title/time. Precedence: **maintenance overrides down** in the headline, so
planned work never shows as an outage. The individual row still shows the true
probe state underneath, so we are not hiding a real failure that coincides.

### Effect 3 — dashboard
A monitor in a window gets a "Maintenance" chip; the alerts view labels suppressed
alerts. No surprises about why it did not page.

### 🔒 Abuse / precedence
A maintenance window is a **loaded gun**: a permanent org-wide window blinds the
whole org silently. Therefore:
- **Writer role required** to create/modify; **audited** like a security change.
- Warn in the UI on windows longer than (say) 24 h; no hard cap, but visible.
- The status page shows maintenance **transparently** — you cannot use it to hide
  an outage from your own customers, only to relabel planned work.

### Effort: **M.** Table + CRUD + one dispatcher hook + status-page join + UI.

---

## Cross-cutting (applies to all three)

- **Migrations are additive and backward-compatible.** `0005_notification_channels`
  is already-declared types becoming real (no schema change — just code + the
  `SupportedTypes` flip). `0006_heartbeats` adds `monitors.ping_token`,
  `last_ping_at`, `grace_seconds`. `0007_maintenance_windows` adds the new table.
  Each ships with a tested `.down.sql`.
- **Tenant isolation** is preserved everywhere: every new query is org-scoped; the
  new Prometheus rule and gauge carry `org_id`; the ping endpoint is scoped by the
  token's monitor.
- **Secrets** (SMTP password, HMAC key, Slack URL) reuse the existing AES-256-GCM
  column and are never returned by the API.
- **Testing bar:** `safehttp` and `Summarise`-style pure logic get table-driven
  unit tests; each feature is then driven end-to-end against the running stack
  (real ping → real gauge → real rule → real alert; real window → real suppression)
  before it is called done. Static types passing is not "done" — this codebase has
  already been bitten twice by bugs `tsc`/`go build` could not see.

## Build order
1. **F1 notification channels** — unblocks adoption, corrects the marketing claim. Smallest.
2. **F2 heartbeats** — highest stickiness-per-line; reuses the F1-era alert spine.
3. **F3 maintenance windows** — protects the status page we already shipped.

Each lands as its own PR, each green end-to-end, before the next starts.
