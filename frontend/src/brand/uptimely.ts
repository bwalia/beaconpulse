import { createElement } from "react";

import type { Brand } from "./types";

/**
 * Uptimely — the third brand, on uptimely.org.
 *
 * Same product as Beacon and SysOps 24/7, shipped under its own name, mark and amber
 * ramp. Build with NEXT_PUBLIC_BRAND=uptimely and the entire product — landing page,
 * dashboard, docs, page titles, focus rings, the logo — comes out as Uptimely with no
 * other file touched. See ./README.md and deploy/helm/beacon/DEPLOY-A-BRAND.md.
 */

// A rising uptime curve breaking upward past a threshold — "trending up, staying up".
// Deliberately unlike Beacon's mark and SysOps's concentric rings, so the brand reads
// as different at a glance. Same 24×24 / 1.75-stroke geometry as every other icon, so
// it inherits currentColor and sits correctly beside the wordmark.
function UptimelyMark({ className }: { className?: string }) {
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
    // The climb.
    createElement("path", { d: "M3.5 17.5L9 12l3.5 3.5L20.5 7" }),
    // Its arrowhead.
    createElement("path", { d: "M15.5 7h5v5" }),
    // The baseline it stays above.
    createElement("path", { d: "M3.5 20.5h17" }),
  );
}

export const uptimely: Brand = {
  name: "Uptimely",
  shortName: "Uptimely",
  tagline: "Know before your users do.",
  description:
    "Uptime, performance and certificate monitoring with instant alerting and public status pages. Multi-tenant and self-hostable.",
  // Cosmetic only — the host shown in the docs' curl examples. The running app always
  // calls its API same-origin regardless.
  apiHost: "uptimely.org",

  // Amber: warm and high-contrast against Beacon's blue and SysOps's green, so all
  // three brands are distinguishable at a glance. Swap these ten values to recolour
  // the entire product.
  primary: {
    50: "#fffbeb",
    100: "#fef3c7",
    200: "#fde68a",
    300: "#fcd34d",
    400: "#fbbf24",
    500: "#f59e0b",
    600: "#d97706",
    700: "#b45309",
    800: "#92400e",
    900: "#78350f",
  },

  Mark: UptimelyMark,

  // Read from NEXT_PUBLIC_GOOGLE_CLIENT_ID at build time, like every other brand —
  // public, but kept out of git with the rest of the credentials.
  googleClientId: process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "",
};
