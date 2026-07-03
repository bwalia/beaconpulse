"use client";

import Link from "next/link";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useActiveAlerts, useMonitors, useOverview } from "@/lib/hooks";
import { VIZ, hhmm } from "@/lib/viz";
import type { MetricPoint, Monitor, MonitorUptime } from "@/lib/types";

export default function DashboardPage() {
  const { data: monitors } = useMonitors();
  const { data: alerts } = useActiveAlerts();
  const { data: overview } = useOverview();

  const list = monitors?.data ?? [];
  const enabled = list.filter((m) => m.enabled);
  const up = enabled.filter((m) => m.last_status === "up").length;
  const down = enabled.filter((m) => m.last_status === "down").length;
  const activeAlerts = alerts?.data ?? [];

  // Join Prometheus history (by monitor_id) onto the authoritative monitor list.
  const histById = new Map<string, MonitorUptime>();
  (overview?.monitors ?? []).forEach((m) => histById.set(m.monitor_id, m));

  return (
    <div className="mx-auto max-w-6xl space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <p className="text-sm text-slate-500">A live overview of your monitored infrastructure.</p>
      </div>

      {/* KPI tiles — each says exactly what it measures */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Kpi
          label="Up right now"
          value={`${up}/${enabled.length}`}
          sub={down > 0 ? `${down} down` : "all healthy"}
          accent={down > 0 ? VIZ.critical : VIZ.good}
        />
        <Kpi
          label="Avg uptime"
          value={overview ? `${overview.uptime_percent}%` : "—"}
          sub="last 24h · all monitors"
          accent={uptimeColor(overview?.uptime_percent)}
        />
        <Kpi
          label="Avg response"
          value={overview ? `${Math.round(overview.avg_response_ms)}ms` : "—"}
          sub="last 24h · all monitors"
        />
        <Kpi
          label="Active alerts"
          value={activeAlerts.length}
          sub={activeAlerts.length ? "needs attention" : "none firing"}
          accent={activeAlerts.length ? VIZ.critical : VIZ.good}
        />
      </div>

      {/* Trend charts */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel title="Availability" subtitle="Share of monitors passing, last 24h">
          <AvailabilityChart data={overview?.uptime_series ?? []} />
        </Panel>
        <Panel title="Response time" subtitle="Average across all monitors, last 24h">
          <ResponseChart data={overview?.response_series ?? []} />
        </Panel>
      </div>

      {/* Per-monitor status */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Monitors</h2>
            <p className="text-sm text-slate-500">Uptime and response over the last 24 hours.</p>
          </div>
          <Link href="/monitors" className="text-sm font-medium text-brand-600 hover:underline">
            Manage →
          </Link>
        </div>

        {list.length === 0 ? (
          <EmptyCard>
            No monitors yet.{" "}
            <Link href="/monitors" className="text-brand-600 hover:underline">
              Add your first
            </Link>
            .
          </EmptyCard>
        ) : (
          <div className="grid gap-3">
            {list.map((m) => (
              <MonitorCard key={m.id} monitor={m} hist={histById.get(m.id)} />
            ))}
          </div>
        )}
      </div>

      {/* Active alerts */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Active alerts</h2>
          <Link href="/alerts" className="text-sm font-medium text-brand-600 hover:underline">
            View all →
          </Link>
        </div>
        {activeAlerts.length === 0 ? (
          <EmptyCard>
            <span className="text-emerald-600">✅ All clear</span> — nothing firing for your organization.
          </EmptyCard>
        ) : (
          <div className="grid gap-2">
            {activeAlerts.slice(0, 6).map((a, i) => (
              <div
                key={i}
                className="flex items-center justify-between rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-sm dark:border-slate-800 dark:bg-slate-900"
              >
                <div>
                  <p className="text-sm font-medium">
                    {a.name} · {a.monitor_name}
                  </p>
                  <p className="font-mono text-xs text-slate-400">{a.target}</p>
                </div>
                <SeverityBadge severity={a.severity} />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/* ---------------- pieces ---------------- */

function uptimeColor(pct?: number): string | undefined {
  if (pct == null) return undefined;
  if (pct >= 99) return VIZ.good;
  if (pct >= 95) return VIZ.warning;
  return VIZ.critical;
}

function Kpi({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: string | number;
  sub: string;
  accent?: string;
}) {
  return (
    <div className="relative overflow-hidden rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <span
        className="absolute inset-y-0 left-0 w-1"
        style={{ background: accent ?? "transparent" }}
        aria-hidden
      />
      <p className="text-xs font-medium uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-1.5 text-3xl font-bold tabular-nums" style={accent ? { color: accent } : undefined}>
        {value}
      </p>
      <p className="mt-0.5 text-xs text-slate-400">{sub}</p>
    </div>
  );
}

function Panel({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div className="mb-3">
        <h3 className="text-sm font-semibold">{title}</h3>
        <p className="text-xs text-slate-400">{subtitle}</p>
      </div>
      {children}
    </div>
  );
}

function EmptyCard({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-white/50 px-4 py-8 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/50">
      {children}
    </div>
  );
}

function StatusDot({ status }: { status: string }) {
  const color =
    status === "up" ? VIZ.good : status === "down" ? VIZ.critical : status === "degraded" ? VIZ.warning : VIZ.noData;
  return (
    <span className="relative flex h-2.5 w-2.5">
      {status === "up" && (
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full opacity-60" style={{ background: color }} />
      )}
      <span className="relative inline-flex h-2.5 w-2.5 rounded-full" style={{ background: color }} />
    </span>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const critical = severity === "critical";
  return (
    <span
      className={`rounded-full px-2.5 py-0.5 text-xs font-medium uppercase ${
        critical
          ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
          : "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
      }`}
    >
      {severity}
    </span>
  );
}

function MonitorCard({ monitor, hist }: { monitor: Monitor; hist?: MonitorUptime }) {
  const status = monitor.enabled ? monitor.last_status : "paused";
  const pts = hist?.points ?? [];
  const uptimePct = pts.length ? Math.round((pts.filter((p) => p.v === 1).length / pts.length) * 1000) / 10 : null;
  const respMs = hist?.avg_response_ms ?? 0;

  return (
    <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm transition hover:shadow-md dark:border-slate-800 dark:bg-slate-900">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <StatusDot status={status} />
            <span className="truncate font-semibold">{monitor.name}</span>
            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium uppercase text-slate-500 dark:bg-slate-800 dark:text-slate-400">
              {monitor.type}
            </span>
          </div>
          <p className="mt-0.5 truncate font-mono text-xs text-slate-400">{monitor.target}</p>
        </div>
        <div className="flex shrink-0 items-center gap-5 text-right">
          <div>
            <p className="text-lg font-bold tabular-nums" style={{ color: uptimeColor(uptimePct ?? undefined) }}>
              {uptimePct != null ? `${uptimePct}%` : "—"}
            </p>
            <p className="text-[10px] uppercase text-slate-400">uptime 24h</p>
          </div>
          <div>
            <p className="text-lg font-bold tabular-nums">{respMs ? `${Math.round(respMs)}ms` : "—"}</p>
            <p className="text-[10px] uppercase text-slate-400">avg resp</p>
          </div>
        </div>
      </div>

      <div className="mt-3">
        <UptimeBar points={pts} />
        <div className="mt-1 flex justify-between text-[10px] text-slate-400">
          <span>24h ago</span>
          <span>now</span>
        </div>
      </div>
    </div>
  );
}

function UptimeBar({ points }: { points: MetricPoint[] }) {
  if (points.length === 0) {
    return (
      <div className="flex h-7 items-center justify-center rounded-md text-[11px] text-slate-400" style={{ background: "#f4f4f2" }}>
        collecting data…
      </div>
    );
  }
  return (
    <div className="flex h-7 gap-0.5 overflow-hidden rounded-md">
      {points.map((p, i) => (
        <div
          key={i}
          className="h-full flex-1 rounded-[2px] transition-opacity hover:opacity-70"
          style={{ background: p.v === 1 ? VIZ.good : VIZ.critical }}
          title={`${hhmm(p.t)} · ${p.v === 1 ? "operational" : "down"}`}
        />
      ))}
    </div>
  );
}

const tooltipStyle = {
  background: "#ffffff",
  border: `1px solid ${VIZ.grid}`,
  borderRadius: 8,
  fontSize: 12,
  color: VIZ.ink,
  boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
};

function toRows(points: MetricPoint[]) {
  return points.map((p) => ({ label: hhmm(p.t), v: p.v }));
}

function AvailabilityChart({ data }: { data: MetricPoint[] }) {
  if (data.length < 2) return <ChartEmpty />;
  const rows = toRows(data).map((r) => ({ ...r, v: Math.round(r.v * 10) / 10 }));
  return (
    <ResponsiveContainer width="100%" height={190}>
      <AreaChart data={rows} margin={{ top: 4, right: 10, left: 4, bottom: 0 }}>
        <defs>
          <linearGradient id="upFill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={VIZ.good} stopOpacity={0.28} />
            <stop offset="100%" stopColor={VIZ.good} stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid stroke={VIZ.grid} vertical={false} />
        <XAxis dataKey="label" tick={{ fill: VIZ.axis, fontSize: 11 }} tickLine={false} axisLine={{ stroke: VIZ.grid }} minTickGap={48} />
        <YAxis
          domain={[0, 100]}
          ticks={[0, 25, 50, 75, 100]}
          tickFormatter={(v) => `${v}%`}
          tick={{ fill: VIZ.axis, fontSize: 11 }}
          tickLine={false}
          axisLine={false}
          width={44}
        />
        <Tooltip contentStyle={tooltipStyle} formatter={(v: number) => [`${v}%`, "Availability"]} />
        <Area type="monotone" dataKey="v" stroke={VIZ.good} strokeWidth={2} fill="url(#upFill)" />
      </AreaChart>
    </ResponsiveContainer>
  );
}

function ResponseChart({ data }: { data: MetricPoint[] }) {
  if (data.length < 2) return <ChartEmpty />;
  const rows = toRows(data).map((r) => ({ ...r, v: Math.round(r.v) }));
  return (
    <ResponsiveContainer width="100%" height={190}>
      <LineChart data={rows} margin={{ top: 4, right: 10, left: 4, bottom: 0 }}>
        <CartesianGrid stroke={VIZ.grid} vertical={false} />
        <XAxis dataKey="label" tick={{ fill: VIZ.axis, fontSize: 11 }} tickLine={false} axisLine={{ stroke: VIZ.grid }} minTickGap={48} />
        <YAxis
          tickFormatter={(v) => `${v}ms`}
          tick={{ fill: VIZ.axis, fontSize: 11 }}
          tickLine={false}
          axisLine={false}
          width={52}
        />
        <Tooltip contentStyle={tooltipStyle} formatter={(v: number) => [`${v}ms`, "Response"]} />
        <Line type="monotone" dataKey="v" stroke={VIZ.blue} strokeWidth={2} dot={false} />
      </LineChart>
    </ResponsiveContainer>
  );
}

function ChartEmpty() {
  return <div className="flex h-[190px] items-center justify-center text-xs text-slate-400">Collecting data…</div>;
}
