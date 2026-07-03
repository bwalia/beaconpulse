# Beacon Roadmap

Beacon is being built **incrementally**: each module is designed, implemented,
tested and documented as a working vertical slice before the next begins. This
file tracks the sequence toward the full platform described in the product brief.

## ✅ Session 1 — Foundation + first vertical slice (done)

Repo scaffold; clean-architecture Go backend; config/logging/errors/crypto;
Postgres + embedded migrations; Redis; reliable job queue; **Auth**, **Projects**,
and **Monitors** (HTTP/HTTPS/SSL/TCP/ICMP/DNS); the **Prometheus/Blackbox control
plane** with alert-rule generation and hot-reload; the background worker;
docker-compose stack (Postgres, Redis, Prometheus, Blackbox, Alertmanager, API,
worker, dashboard); Next.js dashboard (login, projects, monitors); unit tests.

## ✅ Session 2 (part) — Telegram notifications + live status (done)

- **Status feedback loop** — the worker reads `probe_success` from Prometheus
  every 30s and writes each monitor's live `up`/`down` back to the DB
  (`promapi` client + `monitor-status-sync` task), so the dashboard reflects
  reality.
- **Notification channels** CRUD with **AES-256-GCM-encrypted credentials**
  (`notification_channels` table; secrets never returned by the API).
- **Telegram** end-to-end: create bot → paste token + chat id → save → **Send
  test**. Alertmanager forwards every alert (firing + resolved) to Beacon's
  webhook, which routes to the org's channels and delivers **rich HTML messages**
  (severity, monitor, target, project, environment, timestamp, duration, dashboard
  link, and ✅ recovery).

## Planned modules

Each item follows the same process used so far.

### 2. Notifications (remaining channels)
- Slack, Discord, Email (SMTP), Microsoft Teams, generic Webhook — each a new
  `Notifier` in the registry (the domain, dispatcher, webhook and UI are already
  in place; only per-type notifiers + config fields remain).
- Per-severity / per-project routing rules, silencing UI, escalation policies,
  rate limiting and delivery-history view.

### 3. Incidents
- Ingest Alertmanager webhooks → open/resolve incidents; timeline, acknowledge,
  root cause, comments, attachments, duration.

### 4. Advanced HTTP/API monitoring
- Auth (Bearer/Basic/API-key), POST/PUT/DELETE/PATCH bodies, GraphQL, expected-JSON
  assertions, redirect validation, response-size checks (per-monitor generated
  Blackbox modules already support headers/keyword/status — extend the UI + probes).

### 5. Server monitoring
- Node Exporter onboarding (install script + token), CPU/memory/disk/filesystem/
  network/load/swap/processes/temperature, Docker via cAdvisor, container health.
- Threshold-based alert rules authored in a no-PromQL UI (CPU > 90%, disk > 85%, …).

### 6. Kubernetes / k3s monitoring
- kube-state-metrics + cAdvisor onboarding; nodes/pods/deployments/DaemonSets/
  StatefulSets/PVC/namespaces; CrashLoopBackOff, failed scheduling, NotReady,
  restart-count alerts.

### 7. Domain & DNS depth
- WHOIS expiry checks (worker job), MX/A/AAAA/TXT/NS/CAA records, resolution-time
  tracking and alerts.

### 8. RBAC, Teams & Org management
- Teams, granular permissions, membership, invites, password reset, 2FA (columns
  reserved), full audit-log UI.

### 9. Maintenance, tags & organization
- Maintenance windows (suppress alerts), tags/groups/environment labels, bulk
  actions.

### 10. Dashboard depth & real-time
- All admin pages (Incidents, Notifications, Users, Roles, Teams, Audit, Settings,
  Reports, System Health, Jobs, Activity); charts backed by Prometheus queries;
  WebSocket/SSE live status; search everywhere, keyboard shortcuts, skeletons,
  optimistic updates.

### 11. Reporting & SLA
- Uptime/SLA reports, status pages, scheduled report generation.

### 12. Platform hardening & DevOps
- OpenAPI/Swagger spec + generated docs; OpenTelemetry tracing; per-endpoint rate
  limiting; Helm chart, Kubernetes manifests, Argo CD; GitHub Actions CI/CD;
  integration/E2E test suites.

## Known scope notes for Session 1

- **Monitor types** in the control plane today: `http, https, ssl, tcp, icmp, dns`.
  The DB schema already permits the remaining types (`server, kubernetes, api,
  domain, grafana, prometheus, gatus`); the API rejects them with a clear message
  until their module lands.
- **Alertmanager** forwards all alerts to Beacon's webhook (Beacon owns routing +
  delivery). It still handles grouping, de-duplication, inhibition and repeat
  intervals. Per-channel routing rules are a later enhancement.
- **Secret encryption** (AES-256-GCM) is now in use for notification credentials.
