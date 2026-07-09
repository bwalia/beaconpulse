// Chart palette — colors are keyed by the JOB each one does (status vs. sequential),
// never by rank.
//
// VALIDATED, don't eyeball. These four data colors were checked with the dataviz
// validator against both chart surfaces:
//
//   validate_palette.js "#0ca30c,#d03b3b,#d97706,#2a78d6" --mode light|dark
//   → ALL CHECKS PASS (lightness band, chroma floor, CVD separation, contrast)
//
// Re-run it before changing any value here. Notes from that run:
//   • warning was #fab219 — it FAILED the lightness band (L 0.811) and had only
//     1.79:1 contrast on the light surface. #d97706 passes on both surfaces.
//   • good↔critical is the worst CVD pair (ΔE 12.4 deutan) — just over the floor.
//     That is legal ONLY with secondary encoding, so every place these two are
//     used must also carry a text label, icon, or texture. Never color alone.
export const VIZ = {
  good: "#0ca30c", // status: up / healthy
  critical: "#d03b3b", // status: down
  warning: "#d97706", // status: degraded (validated replacement for #fab219)
  blue: "#2a78d6", // sequential: response time
  noData: "#d8d7d0", // status: unknown / paused
};

// Surface-dependent chart chrome. Dark mode is *selected* — its own values checked
// against the dark surface — not an automatic flip of the light values.
//
// Text contrast (WCAG, small text needs 4.5:1):
//   light axis  #64748b on #ffffff = 4.76:1  PASS  (was #898781 = 3.59:1 FAIL)
//   light ink   #52514e on #ffffff = 7.94:1  PASS
//   dark  axis  #94a3b8 on #0f172a = 6.96:1  PASS
// Gridlines carry no text, so they stay deliberately recessive.
export type ChartSurface = {
  grid: string;
  axis: string;
  ink: string;
  tooltipBg: string;
  tooltipBorder: string;
  track: string;
};

export const CHART_SURFACE: Record<"light" | "dark", ChartSurface> = {
  light: {
    grid: "#e1e0d9",
    axis: "#64748b",
    ink: "#52514e",
    tooltipBg: "#ffffff",
    tooltipBorder: "#e1e0d9",
    track: "#f1f5f9",
  },
  dark: {
    grid: "#1e293b",
    axis: "#94a3b8",
    ink: "#e2e8f0",
    tooltipBg: "#0f172a",
    tooltipBorder: "#334155",
    track: "#1e293b",
  },
};

// hhmm formats an ISO timestamp to a compact HH:MM axis/tooltip label.
export function hhmm(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

/** Windows the API accepts (see parseWindowHours in the Go handler's allowlist). */
export const RANGES = [
  { hours: 1, label: "1h" },
  { hours: 6, label: "6h" },
  { hours: 24, label: "24h" },
  { hours: 168, label: "7d" },
  { hours: 720, label: "30d" },
] as const;

export type RangeHours = (typeof RANGES)[number]["hours"];

export function windowLabel(hours: number): string {
  return `last ${RANGES.find((r) => r.hours === hours)?.label ?? `${hours}h`}`;
}

/** Axis ticks scale with the window: clock time when short, dates when long. */
export function tickLabel(iso: string, hours: number): string {
  const d = new Date(iso);
  if (hours <= 24) return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (hours <= 168) return d.toLocaleString([], { weekday: "short", hour: "2-digit" });
  return d.toLocaleDateString([], { day: "numeric", month: "short" });
}

/** Locale-aware full timestamp, used in tooltips where the tick is abbreviated. */
export function fullStamp(iso: string): string {
  return new Date(iso).toLocaleString();
}

/** Compact duration, e.g. 75s / 8m / 3.5h / 15h. */
export function humanDuration(seconds: number): string {
  if (seconds < 90) return `${Math.round(seconds)}s`;
  const minutes = seconds / 60;
  if (minutes < 90) return `${Math.round(minutes)}m`;
  const hours = seconds / 3600;
  return `${Number.isInteger(hours) ? hours : Math.round(hours * 10) / 10}h`;
}

/** How much wall-clock one slot of the uptime strip covers, for the caption. */
export function slotDuration(windowHours: number, slots: number): string {
  return humanDuration((windowHours * 3600) / slots);
}
