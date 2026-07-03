// Chart palette — the validated data-viz reference values, keyed by the JOB each
// color does (status vs. sequential), never by rank. Tuned for the light chart
// surface the app renders on.
export const VIZ = {
  good: "#0ca30c", // status: up / healthy
  critical: "#d03b3b", // status: down
  warning: "#fab219",
  blue: "#2a78d6", // sequential: response time
  grid: "#e1e0d9",
  axis: "#898781",
  ink: "#52514e",
  noData: "#d8d7d0",
};

// hhmm formats an ISO timestamp to a compact HH:MM axis/tooltip label.
export function hhmm(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}
