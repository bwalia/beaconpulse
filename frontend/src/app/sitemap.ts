import type { MetadataRoute } from "next";

import { siteUrl } from "@/lib/site";

// Served at /sitemap.xml. Only public, indexable pages — the dashboard and auth routes
// need a session and carry no crawlable content, so they are intentionally absent (and
// disallowed in robots.ts). Add an entry here when a new public page ships.
const DOC_PAGES = [
  "quickstart",
  "monitors",
  "alerts",
  "status-pages",
  "automation",
  "api",
  "authentication",
  "console",
  "plans",
];

export default function sitemap(): MetadataRoute.Sitemap {
  const lastModified = new Date();
  return [
    { url: `${siteUrl}/`, lastModified, changeFrequency: "weekly", priority: 1 },
    { url: `${siteUrl}/docs`, lastModified, changeFrequency: "weekly", priority: 0.8 },
    ...DOC_PAGES.map((slug) => ({
      url: `${siteUrl}/docs/${slug}`,
      lastModified,
      changeFrequency: "monthly" as const,
      priority: 0.6,
    })),
  ];
}
