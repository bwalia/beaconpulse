import type { MetadataRoute } from "next";

import { noindex, siteUrl } from "@/lib/site";

// Served at /robots.txt. Non-prod deployments (NEXT_PUBLIC_NOINDEX=true) tell crawlers to
// stay out entirely; production allows the public marketing/docs/status pages and keeps
// the authenticated app surface out of the index.
export default function robots(): MetadataRoute.Robots {
  if (noindex) {
    return { rules: { userAgent: "*", disallow: "/" } };
  }
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: [
        "/api/",
        "/login",
        "/register",
        "/dashboard",
        "/monitors",
        "/projects",
        "/alerts",
        "/notifications",
        "/maintenance",
        "/billing",
        "/api-keys",
        "/status-page",
        "/platform",
        "/explore",
        "/system",
      ],
    },
    sitemap: `${siteUrl}/sitemap.xml`,
    host: siteUrl,
  };
}
