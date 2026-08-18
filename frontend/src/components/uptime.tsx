"use client";

// Shared uptime vocabulary and visualisations. The dashboard and the Monitors page
// both read a monitor's health the same way — a status pill and a right-anchored
// slot strip — so the pieces live here once. A fix to how "down" is drawn lands on
// every surface at the same time.

import { VIZ, fullStamp } from "@/lib/viz";
import type { MetricPoint, Monitor, MonitorStatus } from "@/lib/types";

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

export function StripLegend() {
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

export function UptimeStrip({
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
