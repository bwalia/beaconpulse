/**
 * Server-only data layer for the Blog and Articles sections, sourced from the OpsAPI
 * CMS public API (the same pattern as workstation-website's src/lib/cms.js):
 *
 *   GET {BEACON_OPSAPI_URL}/api/v2/public/cms/{namespace}/posts[?category=&page=&perPage=]
 *   GET {BEACON_OPSAPI_URL}/api/v2/public/cms/{namespace}/posts/{slug}
 *
 * Public endpoints — no token. Every fetch goes through Next's data cache with a
 * `revalidate` window and the `cms-posts` tag, so pages are served from cache, refreshed
 * in the background every BEACON_CMS_REVALIDATE_SECONDS, and busted on publish by the
 * OpsAPI webhook at /cms/revalidate. The cache is shared across replicas by the
 * Redis-backed handler in cache-handler.mjs.
 *
 * Blog and Articles are both CMS posts in the same namespace, told apart by category
 * slug (BEACON_CMS_BLOG_CATEGORY / BEACON_CMS_ARTICLES_CATEGORY). An empty category
 * means "no filter".
 *
 * Graceful degradation: with BEACON_OPSAPI_URL unset, or the API down/slow, every call
 * returns an empty result and the pages render their empty state — never an error.
 *
 * Server-only: import it from server components and route handlers, never a client one.
 */

/** OpsAPI CMS namespace the content is read from. Per deployment, so a brand can have its own. */
export const namespace = process.env.BEACON_CMS_NAMESPACE || "beaconpulse";

const OPSAPI_URL = (process.env.BEACON_OPSAPI_URL || "").replace(/\/+$/, "");
const BASE = OPSAPI_URL ? `${OPSAPI_URL}/api/v2/public/cms/${encodeURIComponent(namespace)}` : "";
export const CMS_ENABLED = !!BASE;

export const CMS_CACHE_TAG = "cms-posts";
// `||`, not `??`: an empty env var must fall through to the default too.
const REVALIDATE = Number(process.env.BEACON_CMS_REVALIDATE_SECONDS || 3600);
// Bound every call: a slow (not down) OpsAPI would otherwise hang the render for
// undici's ~300s default before the empty-state fallback could run.
const TIMEOUT_MS = Number(process.env.BEACON_CMS_FETCH_TIMEOUT_MS || 5000);
export const PER_PAGE = 12;

export type CmsKind = "blog" | "articles";

/** Where each section is mounted on the site. */
export const CMS_BASE_PATH: Record<CmsKind, string> = { blog: "/blog", articles: "/articles" };

// `??` here: an explicitly empty category is meaningful (no filter).
const CATEGORY: Record<CmsKind, string> = {
  blog: process.env.BEACON_CMS_BLOG_CATEGORY ?? "blog",
  articles: process.env.BEACON_CMS_ARTICLES_CATEGORY ?? "articles",
};

export interface CmsTerm {
  name: string;
  slug: string;
}

export interface CmsPostSummary {
  uuid: string;
  slug: string;
  title: string;
  excerpt: string;
  image: string | null;
  author: string;
  publishedAt: string | null;
  readingMinutes: number;
  featured: boolean;
  tags: CmsTerm[];
}

export interface CmsPost extends CmsPostSummary {
  html: string;
  seoTitle: string;
  seoDescription: string;
  seoKeywords: string;
}

export interface CmsPostList {
  posts: CmsPostSummary[];
  page: number;
  totalPages: number;
}

type Raw = Record<string, unknown>;

// OpsAPI is Lua; lua-cjson can encode an empty list as `{}` rather than `[]`, so guard
// every list before mapping — otherwise a tag-less post would throw.
const asArray = (v: unknown): Raw[] => (Array.isArray(v) ? (v as Raw[]) : []);
const str = (v: unknown): string => (typeof v === "string" ? v : "");

// OpsAPI emits Postgres `timestamp` text ("2026-09-29 17:31:26.966869", UTC, no zone).
// `new Date()` on that is implementation-defined, so normalise to ISO 8601 UTC.
function isoDate(v: unknown): string | null {
  const s = str(v).trim();
  if (!s) return null;
  const iso = s.replace(" ", "T");
  const zoned = /(Z|[+-]\d{2}:?\d{2})$/.test(iso) ? iso : `${iso}Z`;
  return Number.isNaN(Date.parse(zoned)) ? null : zoned;
}

function mapSummary(p: Raw): CmsPostSummary {
  return {
    uuid: str(p.uuid),
    slug: str(p.slug),
    title: str(p.title),
    excerpt: str(p.excerpt),
    image: str(p.featured_image_url) || null,
    author: str(p.author_name).trim(),
    publishedAt: isoDate(p.published_at),
    readingMinutes: Number(p.reading_minutes) || 0,
    featured: p.is_featured === true,
    tags: asArray(p.tags)
      .map((t) => ({ name: str(t.name) || str(t.slug), slug: str(t.slug) }))
      .filter((t) => t.slug),
  };
}

function mapPost(p: Raw): CmsPost {
  const summary = mapSummary(p);
  return {
    ...summary,
    html: str(p.content_html),
    seoTitle: str(p.seo_title) || summary.title,
    seoDescription: str(p.seo_description) || summary.excerpt,
    seoKeywords: str(p.seo_keywords),
  };
}

function inCategory(p: Raw, category: string): boolean {
  if (!category) return true;
  const primary = p.category as Raw | null | undefined;
  return (
    str(primary?.slug) === category || asArray(p.categories).some((c) => str(c.slug) === category)
  );
}

async function cmsGet(pathAndQuery: string): Promise<Raw | null> {
  const res = await fetch(`${BASE}${pathAndQuery}`, {
    next: { revalidate: REVALIDATE, tags: [CMS_CACHE_TAG] },
    signal: AbortSignal.timeout(TIMEOUT_MS),
  });
  // A missing post/namespace is a normal outcome (404 page / empty list), not an error.
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`CMS ${res.status}`);
  return (await res.json()) as Raw;
}

/** One page of published posts for a section, newest first. */
export async function getPostList(kind: CmsKind, page = 1): Promise<CmsPostList> {
  const empty = { posts: [], page, totalPages: 1 };
  if (!CMS_ENABLED) return empty;
  try {
    const qs = new URLSearchParams({ page: String(page), perPage: String(PER_PAGE) });
    if (CATEGORY[kind]) qs.set("category", CATEGORY[kind]);
    const body = await cmsGet(`/posts?${qs}`);
    const meta = (body?.meta ?? {}) as Raw;
    return {
      posts: asArray(body?.data).map(mapSummary).filter((p) => p.slug),
      page,
      totalPages: Math.max(1, Number(meta.totalPages) || 1),
    };
  } catch (e) {
    console.warn(`cms.getPostList(${kind}) fallback: ${(e as Error).message}`);
    return empty;
  }
}

/**
 * A single published post by slug — or null (→ 404) when it doesn't exist, the CMS is
 * unreachable, or it belongs to the other section (so an article isn't also served,
 * duplicated, under /blog).
 */
export async function getPost(kind: CmsKind, slug: string): Promise<CmsPost | null> {
  if (!CMS_ENABLED) return null;
  try {
    const body = await cmsGet(`/posts/${encodeURIComponent(slug)}`);
    const data = body?.data as Raw | undefined;
    // Guard the lua-cjson "200 with data:{}" case: an empty object is truthy.
    if (!data || !str(data.slug) || !inCategory(data, CATEGORY[kind])) return null;
    return mapPost(data);
  } catch (e) {
    console.warn(`cms.getPost(${kind}, ${slug}) fallback: ${(e as Error).message}`);
    return null;
  }
}

/** Every published slug in a section, for the sitemap. */
export async function getAllSlugs(kind: CmsKind): Promise<{ slug: string; publishedAt: string | null }[]> {
  if (!CMS_ENABLED) return [];
  const out: { slug: string; publishedAt: string | null }[] = [];
  try {
    let page = 1;
    let totalPages = 1;
    do {
      const qs = new URLSearchParams({ page: String(page), perPage: "50" });
      if (CATEGORY[kind]) qs.set("category", CATEGORY[kind]);
      const body = await cmsGet(`/posts?${qs}`);
      for (const p of asArray(body?.data)) {
        if (str(p.slug)) out.push({ slug: str(p.slug), publishedAt: isoDate(p.published_at) });
      }
      totalPages = Number((body?.meta as Raw | undefined)?.totalPages) || 1;
      page += 1;
    } while (page <= totalPages && page <= 20);
  } catch (e) {
    console.warn(`cms.getAllSlugs(${kind}) fallback: ${(e as Error).message}`);
  }
  return out;
}
