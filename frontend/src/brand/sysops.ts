import { createElement } from "react";

import type { Brand } from "./types";

/**
 * SysOps — an example second brand, and the proof that white-labelling actually works.
 *
 * It is a complete, real alternative: different name, a violet ramp instead of blue, and
 * its own mark. Build with NEXT_PUBLIC_BRAND=sysops and the entire product — landing
 * page, dashboard, docs, titles, favicon accent — comes out as SysOps, with no other
 * file touched. Copy this to start a third.
 */

// A distinct mark so the switch is unmistakable: concentric monitoring rings around a
// central node. Same 24×24 / 1.75-stroke geometry as every other icon.
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
  name: "SysOps",
  shortName: "SysOps",
  tagline: "Operations, watched.",
  description:
    "Infrastructure and service monitoring with alerting and public status pages. Self-hosted and multi-tenant.",
  apiHost: "sysops.example.com",

  // A violet ramp — visibly not Beacon's blue, so a glance confirms the brand changed.
  primary: {
    50: "#f5f3ff",
    100: "#ede9fe",
    200: "#ddd6fe",
    300: "#c4b5fd",
    400: "#a78bfa",
    500: "#8b5cf6",
    600: "#7c3aed",
    700: "#6d28d9",
    800: "#5b21b6",
    900: "#4c1d95",
  },

  Mark: SysOpsMark,
};
