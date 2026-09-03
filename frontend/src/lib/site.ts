import { brand } from "@/brand";

// The canonical, absolute origin of THIS deployment's public site — the base for
// canonical links, OG/Twitter card URLs, the sitemap and robots. Resolved once at
// build time (NEXT_PUBLIC_* is inlined by Next), in order:
//   1. NEXT_PUBLIC_SITE_URL  — per-deployment override (set on staging/preview/custom)
//   2. brand.url             — the brand's production apex; the right default per brand
//   3. https://<apiHost>     — last-ditch fallback so URLs are still absolute
// Returned without a trailing slash so `${siteUrl}${path}` never doubles the separator.
function resolveSiteUrl(): string {
  const raw = process.env.NEXT_PUBLIC_SITE_URL || brand.url || `https://${brand.apiHost}`;
  return raw.replace(/\/+$/, "");
}

/** Absolute origin of the public site, e.g. "https://beaconpulse.net". No trailing slash. */
export const siteUrl = resolveSiteUrl();

// Non-production deployments run on real subdomains (int.beaconpulse.net, …) that would
// otherwise get crawled and compete with production. Set NEXT_PUBLIC_NOINDEX=true there
// to keep them out of search results — robots.ts and the root robots meta both read this.
export const noindex = process.env.NEXT_PUBLIC_NOINDEX === "true";

/** Turn a site-relative path into an absolute URL for metadata that needs one. */
export function absoluteUrl(path = "/"): string {
  return `${siteUrl}${path.startsWith("/") ? path : `/${path}`}`;
}
