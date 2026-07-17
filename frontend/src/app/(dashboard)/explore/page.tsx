"use client";

import { motion } from "framer-motion";
import { useId, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { AlertTriangleIcon, ChartLineIcon, LockIcon, SearchIcon } from "@/components/icons";
import { Button, Card, EmptyState, PageHeader } from "@/components/ui";
import { useChartTheme } from "@/lib/use-chart-theme";
import { DUR, useRevealVariants, useStaggerVariants } from "@/lib/motion";
import {
  metricNames,
  queryInstant,
  queryRange,
  seriesLabel,
  type PromResult,
} from "@/lib/promql";
import { RANGES, VIZ, tickLabel } from "@/lib/viz";

/**
 * Explore — tenant-scoped PromQL.
 *
 * This replaces handing customers the stock Prometheus UI. That UI is an OPERATOR
 * tool: half its pages (Targets, Config, TSDB, Service Discovery, Alertmanagers)
 * read global state shared by every tenant, so in a multi-tenant product they can
 * never work — they either error, or they leak. Ours exposes only what is
 * genuinely per-tenant: run a query, see your series.
 *
 * Isolation is not enforced here. prom-label-proxy rewrites every expression to
 * pin org_id to the caller, so a hand-typed query cannot reach another tenant's
 * data even if this page tried.
 */

const EXAMPLES = [
  { label: "Are my monitors up?", expr: "probe_success" },
  { label: "Response time (seconds)", expr: "probe_duration_seconds" },
  { label: "HTTP status codes", expr: "probe_http_status_code" },
  { label: "Days until TLS expiry", expr: "(probe_ssl_earliest_cert_expiry - time()) / 86400" },
  { label: "Availability, last 24h", expr: "avg_over_time(probe_success[24h])" },
];

// Distinct series colours from the validated palette, in a stable order.
const SERIES_COLORS = [VIZ.blue, VIZ.good, VIZ.warning, VIZ.critical, "#7c3aed", "#0891b2"];

type Mode = "instant" | "range";

// What the user actually asked for, as opposed to what they are still typing. It is
// the query key, so "Execute" is just "publish the form", and the default value is
// why the page has a chart on arrival instead of a blank slate — no mount effect.
interface Submitted {
  expr: string;
  mode: Mode;
  hours: number;
}

export default function ExplorePage() {
  const [expr, setExpr] = useState("probe_success");
  const [mode, setMode] = useState<Mode>("range");
  const [hours, setHours] = useState<number>(1);
  const [submitted, setSubmitted] = useState<Submitted>({ expr: "probe_success", mode: "range", hours: 1 });

  const listId = useId();
  const theme = useChartTheme();
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants();

  // Metric names for autocomplete — already scoped to this tenant by the proxy.
  const { data: names = [] } = useQuery({
    queryKey: ["metric-names"],
    queryFn: metricNames,
    staleTime: 5 * 60_000,
  });

  // Keyed on the SUBMITTED query, never on `expr`, so editing the box does not fire
  // a request per keystroke. Re-running an identical query is served from cache.
  const {
    data = null,
    error: queryError,
    isFetching: running,
  } = useQuery<PromResult>({
    queryKey: ["explore", submitted],
    queryFn: () =>
      submitted.mode === "range"
        ? queryRange(submitted.expr, submitted.hours)
        : queryInstant(submitted.expr),
    retry: false, // a PromQL syntax error is not worth retrying
    placeholderData: (previous) => previous,
  });
  const error = queryError ? (queryError instanceof Error ? queryError.message : "Query failed") : null;

  const run = (e?: React.FormEvent) => {
    e?.preventDefault();
    const q = expr.trim();
    if (!q) return;
    setSubmitted({ expr: q, mode, hours });
  };

  // Reshape matrix results into recharts rows: one row per timestamp, one key per
  // series. Done in a memo because it is O(series × points) and must not re-run on
  // every unrelated render.
  const chart = useMemo(() => {
    if (!data || data.resultType !== "matrix") return { rows: [], keys: [] as string[] };
    const byTime = new Map<number, Record<string, number | string>>();
    const keys: string[] = [];

    for (const s of data.result) {
      const key = seriesLabel(s.metric);
      keys.push(key);
      for (const [ts, v] of s.values ?? []) {
        const row = byTime.get(ts) ?? { t: ts };
        row[key] = Number(v);
        byTime.set(ts, row);
      }
    }
    const rows = [...byTime.entries()]
      .sort(([a], [b]) => a - b)
      .map(([, row]) => row);
    return { rows, keys };
  }, [data]);

  const hasResult = !!data && data.result.length > 0;

  return (
    <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-6">
      <PageHeader
        title="Explore"
        subtitle="Run PromQL against your own metrics. Every query is automatically scoped to your organization."
      />

      {/* ---- Query bar ---- */}
      <motion.div variants={reveal}>
        <Card className="p-5">
          <form onSubmit={run} className="space-y-4">
            <label htmlFor={`${listId}-expr`} className="sr-only">
              PromQL expression
            </label>
            <div className="flex flex-col gap-3 sm:flex-row">
              <div className="relative flex-1">
                <SearchIcon className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
                <input
                  id={`${listId}-expr`}
                  list={listId}
                  value={expr}
                  onChange={(e) => setExpr(e.target.value)}
                  spellCheck={false}
                  autoComplete="off"
                  placeholder="probe_success"
                  className="w-full rounded-xl border border-slate-300 bg-white py-3 pl-11 pr-4 font-mono text-base text-slate-900 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
                />
                <datalist id={listId}>
                  {names.map((n) => (
                    <option key={n} value={n} />
                  ))}
                </datalist>
              </div>
              <Button type="submit" size="lg" disabled={running} className="shrink-0">
                {running ? "Running…" : "Execute"}
              </Button>
            </div>

            <div className="flex flex-wrap items-center gap-4">
              {/* instant vs range */}
              <div
                role="group"
                aria-label="Query type"
                className="flex rounded-lg bg-slate-100 p-1 dark:bg-slate-800"
              >
                {(["range", "instant"] as const).map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setMode(m)}
                    aria-pressed={mode === m}
                    className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors motion-reduce:transition-none ${
                      mode === m
                        ? "bg-white text-slate-900 shadow-sm dark:bg-slate-700 dark:text-white"
                        : "text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white"
                    }`}
                    style={{ transitionDuration: `${DUR.micro}s` }}
                  >
                    {m === "range" ? "Over time" : "Right now"}
                  </button>
                ))}
              </div>

              {mode === "range" && (
                <div role="group" aria-label="Time range" className="flex gap-1">
                  {RANGES.map((r) => (
                    <button
                      key={r.hours}
                      type="button"
                      onClick={() => setHours(r.hours)}
                      aria-pressed={hours === r.hours}
                      className={`rounded-md px-2.5 py-1.5 text-sm font-medium tabular-nums transition-colors motion-reduce:transition-none ${
                        hours === r.hours
                          ? "bg-blue-600 text-white"
                          : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
                      }`}
                    >
                      {r.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </form>

          {/* Examples: PromQL has a blank-page problem — most people cannot write
              a query cold. These are the five that answer real questions. */}
          <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-slate-200 pt-4 dark:border-slate-800">
            <span className="text-sm text-slate-500 dark:text-slate-400">Try:</span>
            {EXAMPLES.map((ex) => (
              <button
                key={ex.expr}
                type="button"
                onClick={() => setExpr(ex.expr)}
                className="rounded-lg border border-slate-200 px-2.5 py-1 text-sm text-slate-700 transition-colors hover:border-blue-600 hover:text-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:border-slate-700 dark:text-slate-300 dark:hover:border-blue-400 dark:hover:text-blue-400"
              >
                {ex.label}
              </button>
            ))}
          </div>
        </Card>
      </motion.div>

      {/* ---- Error ---- */}
      {error && (
        <motion.div variants={reveal}>
          <Card className="border-red-300 p-5 dark:border-red-900">
            <p role="alert" className="flex items-start gap-3 text-red-700 dark:text-red-400">
              <AlertTriangleIcon className="mt-0.5 h-5 w-5 shrink-0" />
              {/* Prometheus's own parse error is far more useful than a generic
                  message — show it verbatim, in mono. */}
              <span className="font-mono text-sm">{error}</span>
            </p>
          </Card>
        </motion.div>
      )}

      {/* ---- Graph (range queries) ---- */}
      {!error && mode === "range" && chart.rows.length > 0 && (
        <motion.div variants={reveal}>
          <Card className="p-5">
            <h2 className="mb-4 font-semibold text-slate-900 dark:text-white">Over time</h2>
            <div className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chart.rows} margin={{ top: 4, right: 8, bottom: 4, left: 0 }}>
                  <CartesianGrid stroke={theme.grid} strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="t"
                    tick={{ fill: theme.axis, fontSize: 12 }}
                    tickLine={false}
                    axisLine={{ stroke: theme.grid }}
                    tickFormatter={(t: number) => tickLabel(new Date(t * 1000).toISOString(), hours)}
                    minTickGap={40}
                  />
                  <YAxis
                    tick={{ fill: theme.axis, fontSize: 12 }}
                    tickLine={false}
                    axisLine={{ stroke: theme.grid }}
                    width={52}
                  />
                  <Tooltip
                    contentStyle={{
                      background: theme.track,
                      border: `1px solid ${theme.grid}`,
                      borderRadius: 8,
                      color: theme.ink,
                    }}
                    labelFormatter={(t) => new Date(Number(t) * 1000).toLocaleString()}
                  />
                  {chart.keys.map((k, i) => (
                    <Line
                      key={k}
                      type="monotone"
                      dataKey={k}
                      stroke={SERIES_COLORS[i % SERIES_COLORS.length]}
                      strokeWidth={2}
                      dot={false}
                      isAnimationActive={false}
                    />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
          </Card>
        </motion.div>
      )}

      {/* ---- Table (always, when there are results) ---- */}
      {!error && hasResult && (
        <motion.div variants={reveal}>
          <Card className="overflow-hidden">
            <div className="flex items-center justify-between border-b border-slate-200 px-5 py-4 dark:border-slate-800">
              <h2 className="font-semibold text-slate-900 dark:text-white">
                {data!.result.length} series
              </h2>
              <span className="font-mono text-sm text-slate-500 dark:text-slate-400">
                {data!.resultType}
              </span>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-left">
                <thead className="border-b border-slate-200 text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
                  <tr>
                    <th scope="col" className="px-5 py-3 font-medium">Series</th>
                    <th scope="col" className="px-5 py-3 text-right font-medium">
                      {mode === "range" ? "Latest" : "Value"}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                  {data!.result.map((s, i) => {
                    const v =
                      s.value?.[1] ?? (s.values?.length ? s.values[s.values.length - 1][1] : "—");
                    return (
                      <tr key={i}>
                        <td className="px-5 py-3">
                          <span className="font-mono text-sm text-slate-700 dark:text-slate-200">
                            {seriesLabel(s.metric)}
                          </span>
                        </td>
                        <td className="px-5 py-3 text-right font-mono text-sm tabular-nums text-slate-900 dark:text-white">
                          {v}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </Card>
        </motion.div>
      )}

      {/* ---- Empty ---- */}
      {!error && !running && data && data.result.length === 0 && (
        <motion.div variants={reveal}>
          <Card className="p-5">
            <EmptyState icon={<ChartLineIcon className="h-8 w-8" />} title="No series matched">
              The query is valid but returned nothing. Check the metric name, or widen the
              time range.
            </EmptyState>
          </Card>
        </motion.div>
      )}

      <motion.p
        variants={reveal}
        className="flex items-center justify-center gap-2 pb-2 text-sm text-slate-500 dark:text-slate-400"
      >
        <LockIcon className="h-4 w-4" />
        Queries are scoped to your organization automatically — you cannot see another tenant&apos;s data.
      </motion.p>
    </motion.div>
  );
}
