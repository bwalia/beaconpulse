import { createElement } from "react";

import type { Brand } from "./types";

/**
 * SysOps 24/7 — a complete second brand, and the proof that white-labelling works.
 *
 * It is a real alternative, not a stub: its own name, a cyan ramp instead of Beacon's
 * blue, and its own mark. Build with NEXT_PUBLIC_BRAND=sysops and the entire product —
 * landing page, dashboard, docs, page titles, focus rings, the logo — comes out as
 * SysOps 24/7 with no other file touched. Copy this to start a third brand.
 *
 * To recolour: replace the eleven `primary` hex values with any hand-tuned 50–900 ramp
 * (e.g. paste a Tailwind palette). Everything `brand-*` re-tints from this block alone.
 */

// A distinct mark so the switch is unmistakable: concentric monitoring rings around a
// central node — "watching, around the clock". Same 24×24 / 1.75-stroke geometry as
// every other icon, so it inherits currentColor and sits right next to the wordmark.
function SysOpsMark({ className }: { className?: string }) {
  return createElement(
    "svg",
    {
      viewBox: "0 0 24 24",
      fill: "none",
      stroke: "currentColor",
      strokeWidth: 1.75,
      strokeLinecap: "round",
      strokeLinejoin: "round",
      className,
      "aria-hidden": true,
      focusable: "false",
    },
    createElement("circle", { cx: 12, cy: 12, r: 2.5 }),
    createElement("path", { d: "M12 4.5v2M12 17.5v2M4.5 12h2M17.5 12h2" }),
    createElement("path", { d: "M6.7 6.7l1.4 1.4M15.9 15.9l1.4 1.4M17.3 6.7l-1.4 1.4M8.1 15.9l-1.4 1.4" }),
  );
}

export const sysops: Brand = {
  name: "SysOps 24/7",
  shortName: "SysOps",
  tagline: "Always on. Always watching.",
  description:
    "Round-the-clock infrastructure and service monitoring with alerting and public status pages. Self-hosted and multi-tenant.",
  // Cosmetic only — the host shown in the docs' curl examples. Set this to your real
  // domain; the running app always calls its API same-origin regardless.
  apiHost: "sysops247.io",

  // A green ramp — visibly not Beacon's blue, so a glance confirms the brand changed.
  // Swap these eleven values for any palette to recolour the whole product.
  primary: {
    50: "#ecfdf5",
    100: "#d1fae5",
    200: "#a7f3d0",
    300: "#6ee7b7",
    400: "#34d399",
    500: "#10b981",
    600: "#059669",
    700: "#047857",
    800: "#065f46",
    900: "#064e3b",
  },

  Mark: SysOpsMark,
};
