"use client";

// Tenant-scoped PromQL access for the Explore page.
//
// These call the gateway's /prometheus/api/* path, NOT the Beacon API. That path
// is fronted by prom-label-proxy, which rewrites every query to force
// `org_id="<caller>"` onto it. So a tenant physically cannot express a query that
// reads another tenant's series — the isolation is enforced by the proxy, not by
// anything we remember to do here.
//
// Auth travels on the httpOnly `beacon_proxy` cookie (set at login, Path=/). nginx
// validates it with an auth_request subrequest and injects X-Org-Id for the proxy.
// Hence `credentials: "same-origin"` and no Authorization header: the browser
// cannot read this cookie, which is the point.
//
// Only the endpoints below are reachable. The admin ones (/targets,
// /status/config, /status/tsdb, /alertmanagers) are deliberately NOT proxied —
// they are global, and exposing them would leak every tenant's monitors. That is
// also why we no longer ship the stock Prometheus UI to tenants: half of its
// pages are admin pages that can never work in a multi-tenant product.

const PROM = "/prometheus/api/v1";

export interface PromSample {
  metric: Record<string, string>;
  value?: [number, string];
  values?: [number, string][];
}

export interface PromResult {
  resultType: "vector" | "matrix" | "scalar" | "string";
  result: PromSample[];
}

async function promFetch<T>(path: string, params: Record<string, string>): Promise<T> {
  const qs = new URLSearchParams(params).toString();
  const res = await fetch(`${PROM}${path}?${qs}`, {
    method: "GET",
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });

  // Prometheus reports query errors as a 400 with a JSON body, which is far more
  // useful to show than "Request failed" — surface it verbatim.
  const body = await res.json().catch(() => null);
  if (!res.ok || body?.status === "error") {
    throw new Error(body?.error || `Query failed (HTTP ${res.status})`);
  }
  return body.data as T;
}

/** Instant query — the value of an expression at a single point in time. */
export function queryInstant(expr: string): Promise<PromResult> {
  return promFetch<PromResult>("/query", { query: expr });
}

/**
 * Range query — the expression evaluated over a window.
 *
 * The step is derived from the window so we always ask for roughly 240 points:
 * enough to draw a smooth line, few enough that a 30-day range does not pull
 * hundreds of thousands of samples into the browser and hang the tab.
 */
export function queryRange(expr: string, hours: number): Promise<PromResult> {
  const end = Math.floor(Date.now() / 1000);
  const start = end - hours * 3600;
  const step = Math.max(15, Math.round((hours * 3600) / 240));
  return promFetch<PromResult>("/query_range", {
    query: expr,
    start: String(start),
    end: String(end),
    step: String(step),
  });
}

/** Metric names available to THIS tenant — powers the autocomplete. */
export async function metricNames(): Promise<string[]> {
  const res = await fetch(`${PROM}/label/__name__/values`, {
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });
  if (!res.ok) return [];
  const body = await res.json().catch(() => null);
  return (body?.data as string[]) ?? [];
}

/** A readable one-line label for a series, e.g. `probe_success{monitor="API"}`. */
export function seriesLabel(metric: Record<string, string>): string {
  const name = metric.__name__ ?? "";
  // monitor_name is the only label a human actually recognises; lead with it.
  const friendly = metric.monitor_name ?? metric.instance;
  if (friendly) return name ? `${name} · ${friendly}` : friendly;

  const rest = Object.entries(metric)
    .filter(([k]) => k !== "__name__")
    .map(([k, v]) => `${k}="${v}"`)
    .join(", ");
  return rest ? `${name}{${rest}}` : name || "value";
}
