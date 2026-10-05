"use client";

// Shared uptime vocabulary and visualisations. The dashboard and the Monitors page
// both read a monitor's health the same way — a status pill and a right-anchored
// slot strip — so the pieces live here once. A fix to how "down" is drawn lands on
// every surface at the same time.

import { VIZ, fullStamp } from "@/lib/viz";
import { useMonitorRuns } from "@/lib/hooks";
import type { CheckWindow, GitHubRun, MetricPoint, Monitor, MonitorStatus } from "@/lib/types";

/* ---------------- status vocabulary ---------------- */

export type Tone = "good" | "warning" | "critical" | "neutral";

export const TONE_COLOR: Record<Tone, string> = {
  good: VIZ.good,
  warning: VIZ.warning,
  critical: VIZ.critical,
  neutral: VIZ.noData,
};

export const STATUS_LABEL: Record<string, { text: string; tone: Tone }> = {
  up: { text: "Up", tone: "good" },
  down: { text: "Down", tone: "critical" },
  degraded: { text: "Degraded", tone: "warning" },
  paused: { text: "Paused", tone: "neutral" },
  unknown: { text: "Unknown", tone: "neutral" },
};

/** A monitor's effective status: a paused (disabled) monitor reads as paused
 *  regardless of its last probe result. */
export const statusOf = (m: Monitor): MonitorStatus => (m.enabled ? m.last_status : "paused");

/** Status as a coloured dot + a word — never colour alone (good↔critical sits in
 *  the CVD floor band, so the label carries the meaning). */
export function StatusPill({ status }: { status: string }) {
  const { text, tone } = STATUS_LABEL[status] ?? STATUS_LABEL.unknown;
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: TONE_COLOR[tone] }} aria-hidden />
      <span className="text-xs font-semibold uppercase tracking-wide text-slate-600 dark:text-slate-300">{text}</span>
    </span>
  );
}

/* ---------------- the uptime strip ---------------- */

/** Matches `overviewBuckets` in the Go handler: the API reduces every window to this many samples. */
export const SLOT_COUNT = 48;

type SlotState = "up" | "partial" | "down" | "none";
type Slot = { state: SlotState; w?: CheckWindow };

const DOWN_FILL = `repeating-linear-gradient(45deg, ${VIZ.critical} 0 2px, rgba(255,255,255,0.5) 2px 4px)`;

const slotFill = (s: SlotState) =>
  s === "up" ? VIZ.good : s === "partial" ? VIZ.warning : s === "down" ? DOWN_FILL : VIZ.noData;

// v is the share of the window's checks that passed, so a single failed check
// turns the slot amber instead of vanishing into a green one.
const stateOf = (v: number): SlotState => (v >= 1 ? "up" : v <= 0 ? "down" : "partial");

/** Uptime % over a strip: the mean pass share of its windows (equal-length windows). */
export function uptimePercent(points: MetricPoint[]): number | null {
  if (!points.length) return null;
  const mean = points.reduce((sum, p) => sum + p.v, 0) / points.length;
  return Math.round(mean * 1000) / 10;
}

/**
 * Right-anchored slot grid, the way a status page renders it. Samples land at "now"
 * on the right; windows we have no data for stay explicitly neutral rather than
 * being stretched to fill the bar. Down slots also carry a hatch, because
 * good↔critical sits in the ΔE 8–12 CVD floor band and may not rely on hue alone.
 */
function toSlots(points: CheckWindow[]): Slot[] {
  const slots: Slot[] = Array.from({ length: SLOT_COUNT }, () => ({ state: "none" as SlotState }));
  const tail = points.slice(-SLOT_COUNT);
  const offset = SLOT_COUNT - tail.length;
  tail.forEach((w, i) => {
    slots[offset + i] = { state: stateOf(w.v), w };
  });
  return slots;
}

export function StripLegend() {
  const items: [SlotState, string][] = [
    ["up", "Up"],
    ["partial", "Some checks failed"],
    ["down", "Down"],
    ["none", "No data"],
  ];
  return (
    <ul className="flex flex-wrap items-center gap-3">
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

/* ---------------- what a window's hover explains ---------------- */

const HTTP_TYPES = new Set(["http", "https", "ssl"]);

const dur = (secs = 0) => (secs % 3600 === 0 && secs ? `${secs / 3600}h` : secs % 60 === 0 && secs ? `${secs / 60}m` : `${secs}s`);

function codesLabel(codes?: number[]): string {
  if (!codes?.length) return "2xx or 3xx";
  // The API's default list is every common 2xx/3xx — name it rather than listing 13 codes.
  if (codes.length >= 10 && codes.every((c) => c >= 200 && c < 400)) return "2xx or 3xx";
  return codes.join(", ");
}

/** What one check of this monitor tests — the "how we mark it" half of a hover. */
export function describeChecks(m: Monitor): string[] {
  const s = m.settings ?? {};
  const within = `within ${m.timeout_seconds}s`;
  switch (m.type) {
    case "http":
    case "https":
    case "ssl": {
      const lines = [`${s.method ?? "GET"} ${m.target}`, `• expects HTTP ${codesLabel(s.valid_status_codes)}`];
      if (s.body_keyword) lines.push(`• page must contain "${s.body_keyword}"`);
      if (s.body_not_keyword) lines.push(`• page must not contain "${s.body_not_keyword}"`);
      if (s.headers && Object.keys(s.headers).length) lines.push(`• sends ${Object.keys(s.headers).join(", ")}`);
      if (s.follow_redirects) lines.push("• follows redirects");
      lines.push(`• must answer ${within}`);
      return lines;
    }
    case "tcp":
      return [`Open a TCP connection to ${m.target}`, `• must connect ${within}`];
    case "icmp":
      return [`Ping ${m.target}`, `• must reply ${within}`];
    case "dns":
      return [`Resolve ${s.dns_query_name || m.target} (${s.dns_query_type || "A"} record)`, `• must answer ${within}`];
    case "heartbeat":
      return [`Expects a ping every ${dur(m.interval_seconds)} (+${dur(m.grace_seconds)} grace)`];
    default:
      return [];
  }
}

/** Why a window's failed checks failed, from what the probe recorded. */
function failureReasons(w: CheckWindow, m: Monitor): string[] {
  if (!HTTP_TYPES.has(m.type)) {
    return [m.type === "tcp" ? "connection refused or timed out" : m.type === "icmp" ? "no reply" : "no valid answer"];
  }
  const s = m.settings ?? {};
  const reasons: string[] = [];
  const cmin = w.code_min ?? 0;
  const cmax = w.code_max ?? 0;
  if (w.kw_failed) {
    reasons.push(
      s.body_keyword && !s.body_not_keyword
        ? `page didn't contain "${s.body_keyword}"`
        : s.body_not_keyword && !s.body_keyword
          ? `page contained "${s.body_not_keyword}"`
          : "keyword check failed",
    );
  }
  if (cmin === 0) reasons.push("no response (timeout, DNS, TLS or connection error)");
  const valid = s.valid_status_codes;
  if (cmax > 0 && (valid?.length ? !valid.includes(cmax) : cmax >= 400)) {
    reasons.push(`got HTTP ${cmax}, expected ${codesLabel(valid)}`);
  }
  return reasons.length ? reasons : ["check failed"];
}

function windowTooltip(w: CheckWindow, m: Monitor | undefined, slotMs: number): string {
  const end = new Date(w.t);
  const lines = [`${new Date(end.getTime() - slotMs).toLocaleString()} – ${end.toLocaleTimeString()}`];

  if (m?.type === "heartbeat") {
    const p = w.pings ?? 0;
    const pings = `${p} ping${p === 1 ? "" : "s"} received`;
    lines.push(w.v >= 1 ? `✓ On time — ${pings}` : `✗ Missed — a ping was late (${pings})`);
  } else {
    const n = w.n ?? 0;
    const failed = Math.round(n * (1 - w.v));
    lines.push(
      !n
        ? w.v >= 1 ? "✓ Passed" : "✗ Failed"
        : failed === 0
          ? `✓ All ${n} checks passed`
          : failed >= n
            ? `✗ All ${n} checks failed`
            : `◐ ${failed} of ${n} checks failed`,
    );
    if (m && failed > 0) lines.push(`Why: ${failureReasons(w, m).join("; ")}`);
    const result: string[] = [];
    if (m && HTTP_TYPES.has(m.type) && failed < n && (w.code_max ?? 0) > 0) {
      const lo = w.code_min || w.code_max;
      result.push(lo === w.code_max ? `HTTP ${w.code_max}` : `HTTP ${lo}–${w.code_max}`);
    }
    if (w.ms) result.push(`avg ${Math.round(w.ms)} ms`);
    if (result.length) lines.push(`Result: ${result.join(" · ")}`);
    if (w.ssl_expiry) {
      const days = Math.floor((w.ssl_expiry * 1000 - end.getTime()) / 86_400_000);
      lines.push(days >= 0 ? `SSL certificate valid for ${days} more days` : "SSL certificate expired");
    }
  }

  if (m) {
    const how = describeChecks(m);
    if (how.length) lines.push("", "How it's checked:", ...how);
  }
  return lines.join("\n");
}

export function UptimeStrip({
  points,
  uptimePct,
  winShort,
  hours,
  monitor,
}: {
  points: CheckWindow[];
  uptimePct: number | null;
  winShort: string;
  /** The strip's window, so each slot's hover can show the span it covers. */
  hours: number;
  /** Lets each slot's hover say what was checked, not just whether it passed. */
  monitor?: Monitor;
}) {
  const slots = toSlots(points);
  const slotMs = (hours * 3_600_000) / SLOT_COUNT;
  const failing = points.filter((p) => p.v < 1).length;
  const summary = points.length
    ? `${winShort} history: ${uptimePct ?? 0}% uptime; ${failing} of ${points.length} windows had failed checks. ${SLOT_COUNT - points.length} windows have no data.`
    : `${winShort} history: no data collected yet.`;

  return (
    <div className="flex h-8 gap-[2px]" role="img" aria-label={summary}>
      {slots.map((slot, i) => (
        <div
          key={i}
          className="h-full flex-1 rounded-[2px] transition-opacity hover:opacity-70 motion-reduce:transition-none"
          style={{ background: slotFill(slot.state), opacity: slot.state === "none" ? 0.35 : 1 }}
          title={slot.w ? windowTooltip(slot.w, monitor, slotMs) : "No data collected for this window"}
        />
      ))}
    </div>
  );
}

/* ---------------- github_actions run history ---------------- */

const RUN_HATCH = `repeating-linear-gradient(45deg, ${VIZ.critical} 0 2px, rgba(255,255,255,0.5) 2px 4px)`;

// runFill colours a run box by our classification: green pass, hatched-red fail
// (texture, not hue alone — good↔critical sits in the CVD floor band), amber for an
// outcome we don't score (cancelled, skipped).
function runFill(status: string): string {
  if (status === "up") return VIZ.good;
  if (status === "down") return RUN_HATCH;
  return VIZ.warning;
}

// runTooltip is the full story of one run, shown on hover — "what, and how we marked
// it", exactly the detail the dashboard otherwise hides behind a single pass/fail word.
function runTooltip(run: GitHubRun): string {
  const lines: string[] = [];
  const outcome = run.conclusion ? run.conclusion.replace(/_/g, " ") : run.status;
  lines.push(run.run_number ? `#${run.run_number} · ${outcome}` : outcome);
  if (run.workflow) lines.push(run.workflow);
  const ctx = [run.branch, run.event_name].filter(Boolean).join(" · ");
  if (ctx) lines.push(ctx);
  if (run.actor) lines.push(`by ${run.actor}`);
  if (run.sha) lines.push(run.sha.slice(0, 7));
  lines.push(fullStamp(run.created_at));
  if (run.run_url) lines.push("— click to open on GitHub");
  return lines.join("\n");
}

// RunHistoryStrip draws a github_actions monitor's run history: one right-anchored
// box per reported run, newest on the right, each hover-revealing the full outcome
// and linking back to the run. This is what the plain "Passing/Failing" headline
// hides — a red run stays red here even after the next one passes, so the trend is
// visible and the owner can act on it.
export function RunHistoryStrip({ monitorId }: { monitorId: string }) {
  const { data, isLoading } = useMonitorRuns(monitorId);
  const runs = data?.runs ?? [];

  if (isLoading && !data) {
    return <div className="mt-2 h-6 animate-pulse rounded bg-slate-100 motion-reduce:animate-none dark:bg-slate-800" />;
  }
  if (runs.length === 0) {
    return (
      <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
        No runs recorded yet — they&apos;ll appear here as workflows report in.
      </p>
    );
  }

  const passed = runs.filter((r) => r.status === "up").length;
  const failed = runs.filter((r) => r.status === "down").length;
  // API returns newest-first; show oldest→newest so "now" sits on the right.
  const chronological = [...runs].reverse();

  return (
    <div className="mt-2">
      <div
        className="flex h-6 justify-end gap-[2px]"
        role="img"
        aria-label={`Last ${runs.length} runs: ${passed} passed, ${failed} failed. Newest on the right.`}
      >
        {chronological.map((run, i) => {
          const common = {
            className: "h-full w-2 shrink-0 rounded-[2px] transition-opacity hover:opacity-70 motion-reduce:transition-none",
            style: { background: runFill(run.status) },
            title: runTooltip(run),
          };
          return run.run_url ? (
            <a key={i} href={run.run_url} target="_blank" rel="noopener noreferrer" aria-label={`Run ${run.run_number || i + 1} — open on GitHub`} {...common} />
          ) : (
            <div key={i} {...common} />
          );
        })}
      </div>
      <div className="mt-1 flex justify-between text-xs text-slate-500 dark:text-slate-400">
        <span>
          {passed} passed · {failed} failed
        </span>
        <span>latest →</span>
      </div>
    </div>
  );
}
