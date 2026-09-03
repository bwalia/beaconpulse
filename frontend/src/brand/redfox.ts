import { createElement } from "react";

import type { Brand } from "./types";

/**
 * RedFox Signals — the third brand, on redfoxsignals.com.
 *
 * Same product as Beacon and SysOps 24/7, shipped under its own name, mark and orange
 * ramp. Build with NEXT_PUBLIC_BRAND=redfox and the entire product — landing page,
 * dashboard, docs, page titles, focus rings, the logo — comes out as RedFox Signals
 * with no other file touched. See ./README.md and
 * deploy/helm/beacon/DEPLOY-A-BRAND.md.
 */

// Signal arcs broadcasting from a node — the "Signals" in the name, and a different
// idea from SysOps's concentric rings (those enclose the node; these radiate from one
// corner). Same 24×24 / 1.75-stroke geometry as every other icon, so it inherits
// currentColor and sits correctly beside the wordmark.
function RedFoxMark({ className }: { className?: string }) {
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
    // The transmitting node.
    createElement("circle", { cx: 6, cy: 18, r: 1.6, fill: "currentColor", stroke: "none" }),
    // Three arcs at increasing radius — the signal going out.
    createElement("path", { d: "M11 18A5 5 0 0 0 6 13" }),
    createElement("path", { d: "M16 18A10 10 0 0 0 6 8" }),
    createElement("path", { d: "M21 18A15 15 0 0 0 6 3" }),
  );
}

export const redfox: Brand = {
  name: "RedFox Signals",
  shortName: "RedFox",
  tagline: "Know before your users do.",
  description:
    "Uptime, performance and certificate monitoring with instant alerting and public status pages. Multi-tenant and self-hostable.",
  // Cosmetic only — the host shown in the docs' curl examples. The running app always
  // calls its API same-origin regardless.
  apiHost: "redfoxsignals.com",

  // The production apex — the canonical origin for SEO. See url in types.ts.
  url: "https://redfoxsignals.com",

  // Fox orange: warm and unmistakable against Beacon's blue and SysOps's green, and it
  // carries the name. Swap these ten values to recolour the entire product.
  primary: {
    50: "#fff7ed",
    100: "#ffedd5",
    200: "#fed7aa",
    300: "#fdba74",
    400: "#fb923c",
    500: "#f97316",
    600: "#ea580c",
    700: "#c2410c",
    800: "#9a3412",
    900: "#7c2d12",
  },

  Mark: RedFoxMark,

  // Read from NEXT_PUBLIC_GOOGLE_CLIENT_ID at build time, like every other brand —
  // public, but kept out of git with the rest of the credentials.
  googleClientId: process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID || "",
};
