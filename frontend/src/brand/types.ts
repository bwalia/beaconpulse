import type { ReactElement } from "react";

/**
 * The shape every brand must fill in.
 *
 * This is the ONE place the product's identity is defined. Nothing in the app hardcodes
 * a name, a colour, or a logo — it all reads from the active Brand — so shipping the
 * same product under a second name is editing one file and rebuilding, not hunting a
 * word through two hundred components.
 */
export interface Brand {
  /** The full product name, e.g. "Beacon Pulse". Shown in titles, headers, footers. */
  name: string;
  /** A short form for tight spaces, e.g. "Beacon". Falls back to `name` if you like. */
  shortName: string;
  /** One line under the name on the landing page. */
  tagline: string;
  /** The meta description and the fallback social description. */
  description: string;
  /**
   * The public API host shown in documentation examples, e.g. "beaconpulse.net". Only
   * cosmetic — it makes the curl snippets read as this brand rather than pointing every
   * white-label's docs at beaconpulse.net. The running app calls the API same-origin
   * regardless.
   */
  apiHost: string;

  /**
   * The canonical, absolute origin of this brand's public site, e.g.
   * "https://beaconpulse.net". Used for SEO: metadataBase, canonical links, OG/Twitter
   * card URLs, the sitemap and robots. This is the brand's PRODUCTION apex; a non-prod
   * deployment (staging/preview) overrides it with `NEXT_PUBLIC_SITE_URL` and should set
   * `NEXT_PUBLIC_NOINDEX=true`. See `src/lib/site.ts`. Falls back to `https://<apiHost>`
   * if omitted.
   */
  url?: string;

  /**
   * The accent colour, as a full 50–900 ramp of hex values.
   *
   * A ramp rather than a single hue because good scales are hand-tuned, not linearly
   * derived — a computed scale reads flat and muddy in the mid-tones. These become CSS
   * variables at runtime, so every `brand-*` Tailwind class re-tints from this block
   * alone. Swap the eleven values and the whole product changes colour.
   */
  primary: ColorRamp;

  /**
   * The logo mark — an SVG, sized by a `className`. Kept a component rather than a file
   * path so it inherits `currentColor` and scales crisply; a brand supplies its own.
   */
  Mark: (props: { className?: string }) => ReactElement;

  /** Outbound links used in marketing and docs. Any may be omitted. */
  links?: {
    /** Where "support" / "contact" points. A mailto: or a URL. */
    support?: string;
    /** Public company/site URL, if different from this app. */
    website?: string;
  };

  /** Google OAuth 2.0 Client ID for "Sign in with Google". Empty = Google sign-in hidden. Per-brand so each white-label uses its own Google app. */
  googleClientId?: string;
}

/** A Tailwind-style colour ramp, 50 (lightest) to 900 (darkest), as hex. */
export interface ColorRamp {
  50: string;
  100: string;
  200: string;
  300: string;
  400: string;
  500: string;
  600: string;
  700: string;
  800: string;
  900: string;
}
