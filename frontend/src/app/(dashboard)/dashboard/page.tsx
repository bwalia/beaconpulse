"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useActiveAlerts, useMonitors, useOverview } from "@/lib/hooks";
import { useChartTheme } from "@/lib/use-chart-theme";
import {
  RANGES,
  VIZ,
  fullStamp,
  slotDuration,
  tickLabel,
  windowLabel,
  type ChartSurface,
  type RangeHours,
} from "@/lib/viz";
import type { MetricPoint, Monitor, MonitorUptime } from "@/lib/types";
import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CheckCircleIcon,
  TrendDownIcon,
  TrendUpIcon,
} from "@/components/icons";

/** Uptime target drawn as a reference line on the availability chart. */
const SLO_PERCENT = 99;
/** Matches `overviewBuckets` in the Go handler: the API reduces every window to this many samples. */
const SLOT_COUNT = 48;

export default function DashboardPage() {
  const [hours, setHours] = useState<RangeHours>(24);

  const { data: monitors } = useMonitors();
  const { data: alerts } = useActiveAlerts();
  const { data: overview, isLoading: loadingOverview, isError: overviewFailed, isFetching } = useOverview(hours);

  const list = monitors?.data ?? [];
  const enabled = list.filter((m) => m.enabled);
  const up = enabled.filter((m) => m.last_status === "up").length;
  const downMonitors = enabled.filter((m) => m.last_status === "down");
  const activeAlerts = alerts?.data ?? [];

  const histById = new Map<string, MonitorUptime>();
  (overview?.monitors ?? []).forEach((m) => histById.set(m.monitor_id, m));

  // Triage order: what's broken floats to the top. Nobody should scroll to find an outage.
  const triaged = [...list].sort((a, b) => severityRank(a) - severityRank(b) || a.name.localeCompare(b.name));

  const uptimeSeries = overview?.uptime_series ?? [];
  const responseSeries = overview?.response_series ?? [];
  const win = windowLabel(hours); // "last 24h"
  const winShort = win.replace("last ", ""); // "24h"

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            A live overview of your monitored infrastructure.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <RangeSwitcher value={hours} onChange={setHours} busy={isFetching} />
          <LivePill />
        </div>
      </div>

      <StatusHero
        total={enabled.length}
        up={up}
        downNames={downMonitors.map((m) => m.name)}
        alertCount={activeAlerts.length}
      />

      {/* Stat tiles. The number wears a text token; the rail, dot and delta chip
          carry status, and each is labelled in words — never colour alone. */}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile
          label="Up right now"
          value={`${up}/${enabled.length}`}
          sub={downMonitors.length ? `${downMonitors.length} down` : "all healthy"}
          tone={downMonitors.length ? "critical" : "good"}
          series={uptimeSeries}
          seriesColor={VIZ.good}
        />
        <StatTile
          label="Avg uptime"
          value={overview ? `${overview.uptime_percent}%` : "—"}
          sub={`${win} · all monitors`}
          tone={uptimeTone(overview?.uptime_percent)}
          series={uptimeSeries}
          seriesColor={VIZ.good}
          delta={delta(uptimeSeries)}
          deltaUnit="pts"
          deltaWindow={winShort}
          higherIsBetter
          loading={loadingOverview}
        />
        <StatTile
          label="Avg response"
          value={overview ? `${Math.round(overview.avg_response_ms)}ms` : "—"}
          sub={`${win} · all monitors`}
          series={responseSeries}
          seriesColor={VIZ.blue}
          delta={delta(responseSeries)}
          deltaUnit="ms"
          deltaWindow={winShort}
          higherIsBetter={false}
          loading={loadingOverview}
        />
        <StatTile
          label="Active alerts"
          value={activeAlerts.length}
          sub={activeAlerts.length ? "needs attention" : "none firing"}
          tone={activeAlerts.length ? "critical" : "good"}
        />
      </div>

      {/* Trend charts — the current value is direct-labelled in the header so the eye
          doesn't have to travel to the last pixel of the line. */}
      <div className="grid gap-4 xl:grid-cols-2">
        <Panel
          title="Availability"
          subtitle={`Share of monitors passing · SLO ${SLO_PERCENT}%`}
          value={lastOf(uptimeSeries) != null ? `${round1(lastOf(uptimeSeries)!)}%` : undefined}
        >
          <AvailabilityChart data={uptimeSeries} hours={hours} loading={loadingOverview} failed={overviewFailed} />
        </Panel>
        <Panel
          title="Response time"
          subtitle={`Average across all monitors, ${win}`}
          value={lastOf(responseSeries) != null ? `${Math.round(lastOf(responseSeries)!)}ms` : undefined}
        >
          <ResponseChart data={responseSeries} hours={hours} loading={loadingOverview} failed={overviewFailed} />
        </Panel>
      </div>

      {/* Per-monitor status */}
      <section>
        <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold">Monitors</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {win.charAt(0).toUpperCase() + win.slice(1)}, worst first. Each tick is a{" "}
              {slotDuration(hours, SLOT_COUNT)} window.
            </p>
          </div>
          <div className="flex items-center gap-4">
            <StripLegend />
            <NavLink href="/monitors">Manage</NavLink>
          </div>
        </div>

        {list.length === 0 ? (
          <EmptyCard>
            No monitors yet.{" "}
            <Link href="/monitors" className="font-medium text-brand-700 underline dark:text-brand-400">
              Add your first
            </Link>
            .
          </EmptyCard>
        ) : (
          <div className="grid gap-2.5">
            {triaged.map((m) => (
              <MonitorRow key={m.id} monitor={m} hist={histById.get(m.id)} winShort={winShort} />
            ))}
          </div>
        )}
      </section>

      {/* Active alerts */}
      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Active alerts</h2>
          <NavLink href="/alerts">View all</NavLink>
        </div>
        {activeAlerts.length === 0 ? (
          <EmptyCard>
            <span className="inline-flex items-center gap-1.5 font-medium text-emerald-700 dark:text-emerald-400">
              <CheckCircleIcon className="h-4 w-4" />
              All clear
            </span>{" "}
            — nothing firing for your organization.
          </EmptyCard>
        ) : (
          <ul className="grid gap-2">
            {activeAlerts.slice(0, 6).map((a, i) => (
              <li
                key={i}
                className="flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-white px-4 py-3 shadow-sm dark:border-slate-800 dark:bg-slate-900"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">
                    {a.name} · {a.monitor_name}
                  </p>
                  <p className="truncate font-mono text-xs text-slate-500 dark:text-slate-400">{a.target}</p>
                </div>
                <SeverityBadge severity={a.severity} />
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

/* ---------------- status vocabulary ---------------- */

type Tone = "good" | "warning" | "critical" | "neutral";

const TONE_COLOR: Record<Tone, string> = {
  good: VIZ.good,
  warning: VIZ.warning,
  critical: VIZ.critical,
  neutral: VIZ.noData,
};

const STATUS_LABEL: Record<string, { text: string; tone: Tone }> = {
  up: { text: "Up", tone: "good" },
  down: { text: "Down", tone: "critical" },
  degraded: { text: "Degraded", tone: "warning" },
  paused: { text: "Paused", tone: "neutral" },
  unknown: { text: "Unknown", tone: "neutral" },
};

const TRIAGE_ORDER: Record<string, number> = { down: 0, degraded: 1, unknown: 2, up: 3, paused: 4 };

const statusOf = (m: Monitor) => (m.enabled ? m.last_status : "paused");
const severityRank = (m: Monitor) => TRIAGE_ORDER[statusOf(m)] ?? 9;

function uptimeTone(pct?: number): Tone | undefined {
  if (pct == null) return undefined;
  if (pct >= SLO_PERCENT) return "good";
  if (pct >= 95) return "warning";
  return "critical";
}

/* ---------------- small maths ---------------- */

const round1 = (n: number) => Math.round(n * 10) / 10;
const lastOf = (s: MetricPoint[]) => (s.length ? s[s.length - 1].v : null);
/** Change across the visible window: last sample minus first. Derived, not invented. */
const delta = (s: MetricPoint[]) => (s.length >= 2 ? s[s.length - 1].v - s[0].v : null);

/* ---------------- controls ---------------- */

function RangeSwitcher({
  value,
  onChange,
  busy,
}: {
  value: RangeHours;
  onChange: (h: RangeHours) => void;
  busy?: boolean;
}) {
  return (
    <div
      role="group"
      aria-label="Time range"
      aria-busy={busy}
      className="inline-flex items-center rounded-lg border border-slate-200 bg-white p-0.5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      {RANGES.map((r) => {
        const active = r.hours === value;
        return (
          <button
            key={r.hours}
            type="button"
            onClick={() => onChange(r.hours)}
            aria-pressed={active}
            className={`h-8 rounded-md px-3 text-xs font-semibold tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 motion-reduce:transition-none ${
              active
                ? "bg-brand-600 text-white shadow-sm"
                : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
            }`}
          >
            {r.label}
          </button>
        );
      })}
    </div>
  );
}

function LivePill() {
  return (
    <span className="inline-flex h-9 items-center gap-2 rounded-full border border-slate-200 bg-white px-3 text-xs font-medium text-slate-600 shadow-sm dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300">
      <span className="relative flex h-2 w-2">
        <span
          className="absolute inline-flex h-full w-full rounded-full opacity-60 motion-safe:animate-ping"
          style={{ background: VIZ.good }}
        />
        <span className="relative inline-flex h-2 w-2 rounded-full" style={{ background: VIZ.good }} />
      </span>
      Live
    </span>
  );
}

/* ---------------- pieces ---------------- */

/**
 * The one thing an on-call engineer should read in under a second. Tone comes with
 * an icon and a sentence, so state never rides on colour alone.
 */
function StatusHero({
  total,
  up,
  downNames,
  alertCount,
}: {
  total: number;
  up: number;
  downNames: string[];
  alertCount: number;
}) {
  const down = downNames.length;
  const tone: Tone = down > 0 ? "critical" : alertCount > 0 ? "warning" : "good";

  const skin = {
    good: "from-emerald-50 to-white ring-emerald-200 dark:from-emerald-950/40 dark:to-slate-900 dark:ring-emerald-900",
    warning: "from-amber-50 to-white ring-amber-200 dark:from-amber-950/40 dark:to-slate-900 dark:ring-amber-900",
    critical: "from-red-50 to-white ring-red-200 dark:from-red-950/40 dark:to-slate-900 dark:ring-red-900",
    neutral: "from-slate-50 to-white ring-slate-200 dark:from-slate-900 dark:to-slate-900 dark:ring-slate-800",
  }[tone];

  const headline =
    down > 0
      ? `${down} of ${total} monitors ${down === 1 ? "is" : "are"} down`
      : alertCount > 0
        ? `${alertCount} alert${alertCount === 1 ? "" : "s"} firing`
        : "All systems operational";

  const detail =
    down > 0
      ? `${downNames.slice(0, 2).join(", ")}${down > 2 ? ` and ${down - 2} more` : ""} failing checks` +
        (alertCount ? ` · ${alertCount} alert${alertCount === 1 ? "" : "s"} firing` : "")
      : alertCount > 0
        ? "All monitors are passing, but alerts are still active."
        : `${up} monitor${up === 1 ? "" : "s"} passing · no alerts firing`;

  return (
    <section
      className={`relative overflow-hidden rounded-2xl bg-gradient-to-r p-5 ring-1 sm:p-6 ${skin}`}
      aria-label="Overall system health"
    >
      <span className="absolute inset-y-0 left-0 w-1.5" style={{ background: TONE_COLOR[tone] }} aria-hidden />
      <div className="flex flex-wrap items-center gap-4 pl-2">
        <span
          className="grid h-12 w-12 shrink-0 place-items-center rounded-full text-white"
          style={{ background: TONE_COLOR[tone] }}
        >
          {tone === "good" ? <CheckCircleIcon className="h-6 w-6" /> : <AlertTriangleIcon className="h-6 w-6" />}
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-xl font-bold tracking-tight text-slate-900 sm:text-2xl dark:text-slate-50">{headline}</h2>
          <p className="mt-0.5 truncate text-sm text-slate-600 dark:text-slate-300">{detail}</p>
        </div>
        <dl className="flex shrink-0 gap-6 pr-1">
          <HeroStat label="Passing" value={`${up}/${total}`} />
          <HeroStat label="Alerts" value={alertCount} />
        </dl>
      </div>
    </section>
  );
}

function HeroStat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="text-right">
      <dt className="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">{label}</dt>
      <dd className="text-xl font-bold tabular-nums text-slate-900 dark:text-slate-50">{value}</dd>
    </div>
  );
}

/** Sparkline: trend shape only. The tile's big number is the direct label and the
 *  full chart below carries the hover layer, so this stays decorative to AT. */
function Sparkline({ series, color }: { series: MetricPoint[]; color: string }) {
  if (series.length < 2) return <div className="h-10" />;
  const rows = series.map((p) => ({ v: p.v }));
  const id = `spark-${color.replace("#", "")}`;
  return (
    <div className="h-10" aria-hidden>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={rows} margin={{ top: 2, right: 0, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.3} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <Area type="monotone" dataKey="v" stroke={color} strokeWidth={1.75} fill={`url(#${id})`} isAnimationActive={false} />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

function DeltaChip({
  value,
  unit,
  window: win,
  higherIsBetter,
}: {
  value: number;
  unit: string;
  window: string;
  higherIsBetter: boolean;
}) {
  const rounded = unit === "ms" ? Math.round(value) : round1(value);
  if (rounded === 0) return null;
  const rising = rounded > 0;
  const good = rising === higherIsBetter;
  const Arrow = rising ? TrendUpIcon : TrendDownIcon;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-semibold tabular-nums ${
        good
          ? "bg-emerald-50 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300"
          : "bg-red-50 text-red-800 dark:bg-red-950/60 dark:text-red-300"
      }`}
      title={`${rising ? "Up" : "Down"} ${Math.abs(rounded)}${unit} vs ${win} ago`}
    >
      <Arrow className="h-3 w-3" />
      {rising ? "+" : "−"}
      {Math.abs(rounded)}
      {unit}
    </span>
  );
}

function StatTile({
  label,
  value,
  sub,
  tone,
  series,
  seriesColor,
  delta: d,
  deltaUnit,
  deltaWindow,
  higherIsBetter = true,
  loading,
}: {
  label: string;
  value: string | number;
  sub: string;
  tone?: Tone;
  series?: MetricPoint[];
  seriesColor?: string;
  delta?: number | null;
  deltaUnit?: string;
  deltaWindow?: string;
  higherIsBetter?: boolean;
  loading?: boolean;
}) {
  const accent = tone ? TONE_COLOR[tone] : undefined;
  return (
    <div className="relative flex flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm transition-shadow hover:shadow-md motion-reduce:transition-none dark:border-slate-800 dark:bg-slate-900">
      <span className="absolute inset-y-0 left-0 w-1" style={{ background: accent ?? "transparent" }} aria-hidden />
      <div className="p-4 pb-1">
        <div className="flex items-center gap-1.5">
          {accent && <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: accent }} aria-hidden />}
          <p className="text-xs font-medium uppercase tracking-wide text-slate-500 dark:text-slate-400">{label}</p>
        </div>
        {loading ? (
          <div className="mt-2 h-9 w-24 rounded bg-slate-100 motion-safe:animate-pulse dark:bg-slate-800" />
        ) : (
          <div className="mt-1 flex items-baseline gap-2">
            <p className="text-3xl font-bold tabular-nums text-slate-900 dark:text-slate-50">{value}</p>
            {d != null && deltaUnit && deltaWindow && (
              <DeltaChip value={d} unit={deltaUnit} window={deltaWindow} higherIsBetter={higherIsBetter} />
            )}
          </div>
        )}
        <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">{sub}</p>
      </div>
      {series && seriesColor ? <Sparkline series={series} color={seriesColor} /> : <div className="h-4" />}
    </div>
  );
}

function Panel({
  title,
  subtitle,
  value,
  children,
}: {
  title: string;
  subtitle: string;
  value?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div className="mb-3 flex items-start justify-between gap-4">
        <div>
          <h3 className="text-sm font-semibold">{title}</h3>
          <p className="text-xs text-slate-500 dark:text-slate-400">{subtitle}</p>
        </div>
        {value && <p className="shrink-0 text-2xl font-bold tabular-nums text-slate-900 dark:text-slate-50">{value}</p>}
      </div>
      {children}
    </div>
  );
}

function EmptyCard({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-white/50 px-4 py-8 text-center text-sm text-slate-600 dark:border-slate-700 dark:bg-slate-900/50 dark:text-slate-300">
      {children}
    </div>
  );
}

function NavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      className="group inline-flex items-center gap-1 rounded text-sm font-medium text-brand-700 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 dark:text-brand-400 dark:focus-visible:ring-offset-slate-950"
    >
      {children}
      <ArrowRightIcon className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
    </Link>
  );
}

function StatusPill({ status }: { status: string }) {
  const { text, tone } = STATUS_LABEL[status] ?? STATUS_LABEL.unknown;
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: TONE_COLOR[tone] }} aria-hidden />
      <span className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-300">{text}</span>
    </span>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const critical = severity === "critical";
  return (
    <span
      className={`shrink-0 rounded-full px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wide ${
        critical
          ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200"
          : "bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200"
      }`}
    >
      {severity}
    </span>
  );
}

/* ---------------- the uptime strip ---------------- */

type SlotState = "up" | "down" | "none";
type Slot = { state: SlotState; t?: string };

const DOWN_FILL = `repeating-linear-gradient(45deg, ${VIZ.critical} 0 2px, rgba(255,255,255,0.5) 2px 4px)`;

const slotFill = (s: SlotState) => (s === "up" ? VIZ.good : s === "down" ? DOWN_FILL : VIZ.noData);

/**
 * Right-anchored slot grid, the way a status page renders it. Samples land at "now"
 * on the right; windows we have no data for stay explicitly neutral rather than
 * being stretched to fill the bar. Down slots also carry a hatch, because
 * good↔critical sits in the ΔE 8–12 CVD floor band and may not rely on hue alone.
 */
function toSlots(points: MetricPoint[]): Slot[] {
  const slots: Slot[] = Array.from({ length: SLOT_COUNT }, () => ({ state: "none" as SlotState }));
  const tail = points.slice(-SLOT_COUNT);
  const offset = SLOT_COUNT - tail.length;
  tail.forEach((p, i) => {
    slots[offset + i] = { state: p.v === 1 ? "up" : "down", t: p.t };
  });
  return slots;
}

function StripLegend() {
  const items: [SlotState, string][] = [
    ["up", "Up"],
    ["down", "Down"],
    ["none", "No data"],
  ];
  return (
    <ul className="flex items-center gap-3">
      {items.map(([state, label]) => (
        <li key={state} className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300">
          <span
            className="h-2.5 w-2.5 rounded-[2px]"
            style={{ background: slotFill(state), opacity: state === "none" ? 0.35 : 1 }}
            aria-hidden
          />
          {label}
        </li>
      ))}
    </ul>
  );
}

function UptimeStrip({
  points,
  uptimePct,
  winShort,
}: {
  points: MetricPoint[];
  uptimePct: number | null;
  winShort: string;
}) {
  const slots = toSlots(points);
  const passed = points.filter((p) => p.v === 1).length;
  const failed = points.length - passed;
  const summary = points.length
    ? `${winShort} history: ${passed} of ${points.length} checks passed (${uptimePct ?? 0}% uptime), ${failed} failed. ${SLOT_COUNT - points.length} windows have no data.`
    : `${winShort} history: no data collected yet.`;

  return (
    <div className="flex h-8 gap-[2px]" role="img" aria-label={summary}>
      {slots.map((slot, i) => (
        <div
          key={i}
          className="h-full flex-1 rounded-[2px] transition-opacity hover:opacity-70 motion-reduce:transition-none"
          style={{ background: slotFill(slot.state), opacity: slot.state === "none" ? 0.35 : 1 }}
          title={
            slot.t
              ? `${fullStamp(slot.t)} · ${slot.state === "up" ? "operational" : "down"}`
              : "no data for this window"
          }
        />
      ))}
    </div>
  );
}

function MonitorRow({
  monitor,
  hist,
  winShort,
}: {
  monitor: Monitor;
  hist?: MonitorUptime;
  winShort: string;
}) {
  const status = statusOf(monitor);
  const pts = hist?.points ?? [];
  const passed = pts.filter((p) => p.v === 1).length;
  const uptimePct = pts.length ? round1((passed / pts.length) * 100) : null;
  const respMs = hist?.avg_response_ms ?? 0;
  const isDown = status === "down";

  return (
    <article
      className={`rounded-xl border bg-white p-3.5 shadow-sm transition-shadow hover:shadow-md motion-reduce:transition-none dark:bg-slate-900 ${
        isDown ? "border-red-200 dark:border-red-900/60" : "border-slate-200 dark:border-slate-800"
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <StatusPill status={status} />
            <span className="truncate font-semibold">{monitor.name}</span>
            <span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs font-medium uppercase text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              {monitor.type}
            </span>
          </div>
          <p className="mt-0.5 truncate font-mono text-xs text-slate-500 dark:text-slate-400">{monitor.target}</p>
        </div>
        <div className="flex shrink-0 items-center gap-6 text-right">
          <div>
            <p className="text-lg font-bold tabular-nums text-slate-900 dark:text-slate-50">
              {uptimePct != null ? `${uptimePct}%` : "—"}
            </p>
            <p className="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">uptime {winShort}</p>
          </div>
          <div>
            <p className="text-lg font-bold tabular-nums text-slate-900 dark:text-slate-50">
              {respMs ? `${Math.round(respMs)}ms` : "—"}
            </p>
            <p className="text-xs uppercase tracking-wide text-slate-500 dark:text-slate-400">avg resp</p>
          </div>
        </div>
      </div>

      <div className="mt-3">
        <UptimeStrip points={pts} uptimePct={uptimePct} winShort={winShort} />
        <div className="mt-1 flex justify-between text-xs text-slate-500 dark:text-slate-400">
          <span>{winShort} ago</span>
          <span>now</span>
        </div>
      </div>
    </article>
  );
}

/* ---------------- charts ---------------- */

const CHART_HEIGHT = 200;

type TipItem = { payload?: { full?: string } };

function tooltipStyle(theme: ChartSurface) {
  return {
    background: theme.tooltipBg,
    border: `1px solid ${theme.tooltipBorder}`,
    borderRadius: 8,
    fontSize: 12,
    color: theme.ink,
    boxShadow: "0 4px 12px rgba(0,0,0,0.08)",
  };
}

/** Ticks abbreviate with the window; `full` keeps the exact stamp for the tooltip. */
const toRows = (points: MetricPoint[], hours: number) =>
  points.map((p) => ({ label: tickLabel(p.t, hours), full: fullStamp(p.t), v: p.v }));

const tipLabel = (_: unknown, items: TipItem[]) => items?.[0]?.payload?.full ?? "";

function ChartState({ loading, failed, empty }: { loading?: boolean; failed?: boolean; empty?: boolean }) {
  if (loading)
    return <div className="rounded-md bg-slate-100 motion-safe:animate-pulse dark:bg-slate-800" style={{ height: CHART_HEIGHT }} />;
  if (failed)
    return (
      <div className="flex items-center justify-center text-xs text-red-700 dark:text-red-300" style={{ height: CHART_HEIGHT }}>
        Couldn’t load metrics. They’ll retry automatically.
      </div>
    );
  if (empty)
    return (
      <div className="flex items-center justify-center text-xs text-slate-500 dark:text-slate-400" style={{ height: CHART_HEIGHT }}>
        Collecting data…
      </div>
    );
  return null;
}

function AvailabilityChart({
  data,
  hours,
  loading,
  failed,
}: {
  data: MetricPoint[];
  hours: number;
  loading?: boolean;
  failed?: boolean;
}) {
  const theme = useChartTheme();
  if (loading || failed || data.length < 2)
    return <ChartState loading={loading} failed={failed} empty={data.length < 2} />;

  const rows = toRows(data, hours).map((r) => ({ ...r, v: round1(r.v) }));
  const last = rows[rows.length - 1];

  return (
    <div
      role="img"
      aria-label={`Availability over the ${windowLabel(hours)}, currently ${last.v}% of monitors passing. Target is ${SLO_PERCENT}%.`}
    >
      <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
        <AreaChart data={rows} margin={{ top: 8, right: 12, left: 4, bottom: 0 }}>
          <defs>
            <linearGradient id="upFill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={VIZ.good} stopOpacity={0.3} />
              <stop offset="100%" stopColor={VIZ.good} stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={theme.grid} vertical={false} />
          <XAxis dataKey="label" tick={{ fill: theme.axis, fontSize: 11 }} tickLine={false} axisLine={{ stroke: theme.grid }} minTickGap={48} />
          <YAxis domain={[0, 100]} ticks={[0, 25, 50, 75, 100]} tickFormatter={(v) => `${v}%`} tick={{ fill: theme.axis, fontSize: 11 }} tickLine={false} axisLine={false} width={44} />
          <ReferenceLine
            y={SLO_PERCENT}
            stroke={theme.axis}
            strokeDasharray="4 4"
            label={{ value: `SLO ${SLO_PERCENT}%`, position: "insideTopRight", fill: theme.axis, fontSize: 11 }}
          />
          <Tooltip
            contentStyle={tooltipStyle(theme)}
            cursor={{ stroke: theme.axis, strokeDasharray: "3 3" }}
            labelFormatter={tipLabel}
            formatter={(v: number) => [`${v}%`, "Availability"]}
          />
          <Area type="monotone" dataKey="v" stroke={VIZ.good} strokeWidth={2} fill="url(#upFill)" isAnimationActive={false} />
          {/* 2px surface ring so the anchor reads on top of the fill. */}
          <ReferenceDot x={last.label} y={last.v} r={3.5} fill={VIZ.good} stroke={theme.tooltipBg} strokeWidth={2} isFront />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

function ResponseChart({
  data,
  hours,
  loading,
  failed,
}: {
  data: MetricPoint[];
  hours: number;
  loading?: boolean;
  failed?: boolean;
}) {
  const theme = useChartTheme();
  if (loading || failed || data.length < 2)
    return <ChartState loading={loading} failed={failed} empty={data.length < 2} />;

  const rows = toRows(data, hours).map((r) => ({ ...r, v: Math.round(r.v) }));
  const last = rows[rows.length - 1];
  const mean = Math.round(rows.reduce((a, r) => a + r.v, 0) / rows.length);

  return (
    <div
      role="img"
      aria-label={`Average response time over the ${windowLabel(hours)}, currently ${last.v} milliseconds, window average ${mean} milliseconds.`}
    >
      <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
        <LineChart data={rows} margin={{ top: 8, right: 12, left: 4, bottom: 0 }}>
          <CartesianGrid stroke={theme.grid} vertical={false} />
          <XAxis dataKey="label" tick={{ fill: theme.axis, fontSize: 11 }} tickLine={false} axisLine={{ stroke: theme.grid }} minTickGap={48} />
          <YAxis tickFormatter={(v) => `${v}ms`} tick={{ fill: theme.axis, fontSize: 11 }} tickLine={false} axisLine={false} width={56} />
          <ReferenceLine
            y={mean}
            stroke={theme.axis}
            strokeDasharray="4 4"
            label={{ value: `avg ${mean}ms`, position: "insideTopRight", fill: theme.axis, fontSize: 11 }}
          />
          <Tooltip
            contentStyle={tooltipStyle(theme)}
            cursor={{ stroke: theme.axis, strokeDasharray: "3 3" }}
            labelFormatter={tipLabel}
            formatter={(v: number) => [`${v}ms`, "Response"]}
          />
          <Line type="monotone" dataKey="v" stroke={VIZ.blue} strokeWidth={2} dot={false} isAnimationActive={false} />
          <ReferenceDot x={last.label} y={last.v} r={3.5} fill={VIZ.blue} stroke={theme.tooltipBg} strokeWidth={2} isFront />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
