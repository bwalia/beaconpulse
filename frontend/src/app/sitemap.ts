import type { MetadataRoute } from "next";

import { CMS_BASE_PATH, getAllSlugs, type CmsKind } from "@/lib/cms";
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

// Regenerate hourly so newly published Blog/Articles posts appear without a redeploy.
export const revalidate = 3600;

async function cmsEntries(kind: CmsKind): Promise<MetadataRoute.Sitemap> {
  const posts = await getAllSlugs(kind);
  return posts.map((p) => ({
    url: `${siteUrl}${CMS_BASE_PATH[kind]}/${p.slug}`,
    ...(p.publishedAt && { lastModified: new Date(p.publishedAt) }),
    changeFrequency: "monthly" as const,
    priority: 0.6,
  }));
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const lastModified = new Date();
  const [blog, articles] = await Promise.all([cmsEntries("blog"), cmsEntries("articles")]);
  return [
    { url: `${siteUrl}/`, lastModified, changeFrequency: "weekly", priority: 1 },
    { url: `${siteUrl}/docs`, lastModified, changeFrequency: "weekly", priority: 0.8 },
    ...DOC_PAGES.map((slug) => ({
      url: `${siteUrl}/docs/${slug}`,
      lastModified,
      changeFrequency: "monthly" as const,
      priority: 0.6,
    })),
    { url: `${siteUrl}${CMS_BASE_PATH.blog}`, lastModified, changeFrequency: "daily", priority: 0.7 },
    { url: `${siteUrl}${CMS_BASE_PATH.articles}`, lastModified, changeFrequency: "daily", priority: 0.7 },
    { url: `${siteUrl}/terms`, changeFrequency: "yearly", priority: 0.3 },
    { url: `${siteUrl}/privacy`, changeFrequency: "yearly", priority: 0.3 },
    ...blog,
    ...articles,
  ];
}
