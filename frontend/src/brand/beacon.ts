import { createElement } from "react";

import type { Brand } from "./types";

/**
 * Beacon Pulse — the default brand.
 *
 * Everything the product says about itself lives here. To ship a second brand, copy this
 * file, change the values, and select it with NEXT_PUBLIC_BRAND (see index.ts) — nothing
 * else in the app names the product or picks its colour.
 */

// The broadcast-and-pulse mark, as a bare SVG so it takes a className and inherits
// currentColor exactly like the rest of the icon set. Matches the shared Icon geometry
// (24×24, 1.75 stroke) so it sits right next to the wordmark.
function BeaconMark({ className }: { className?: string }) {
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
    createElement("path", { d: "M4.6 18.6a9 9 0 0 1 0-13.2" }),
    createElement("path", { d: "M19.4 5.4a9 9 0 0 1 0 13.2" }),
    createElement("path", { d: "M7.3 12h1.9l1.3-4 2.4 8 1.3-4h1.9" }),
  );
}

export const beacon: Brand = {
  name: "Beacon Pulse",
  shortName: "Beacon",
  tagline: "Know before your customers do.",
  description:
    "Uptime, latency, SSL and DNS monitoring with alerting and public status pages. Self-hosted and multi-tenant.",
  apiHost: "beaconpulse.net",

  // The original blue ramp, unchanged — this is now the single source every brand-*
  // class reads from.
  primary: {
    50: "#eef7ff",
    100: "#d9edff",
    200: "#bce0ff",
    300: "#8ecdff",
    400: "#59b0ff",
    500: "#328cff",
    600: "#1b6ef5",
    700: "#1657e1",
    800: "#1847b6",
    900: "#1a3f8f",
  },

  Mark: BeaconMark,
};
