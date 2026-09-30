import type { Metadata } from "next";
import { notFound, permanentRedirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

import { brand } from "@/brand";
import { CMS_BASE_PATH, getPost, getPostList, type CmsKind } from "@/lib/cms";
import { MarketingPage, PostArticle, PostListing } from "./views";

// Route bodies shared by /blog and /articles, so the two sections can't drift. Each
// app/<section>/... page file is a one-line binding of these to its `kind`.

type SlugParams = { params: Promise<{ slug: string }> };
type PageParams = { params: Promise<{ page: string }> };

export async function listingMetadata(kind: CmsKind, page = 1): Promise<Metadata> {
  const t = await getTranslations("cms");
  const title = `${t(`${kind}.title`)} — ${brand.name}`;
  const path = page > 1 ? `${CMS_BASE_PATH[kind]}/page/${page}` : CMS_BASE_PATH[kind];
  return {
    title,
    description: t(`${kind}.subtitle`),
    alternates: { canonical: path },
    openGraph: { type: "website", siteName: brand.name, url: path, title, description: t(`${kind}.subtitle`) },
  };
}

export async function ListingRoute({ kind, page = 1 }: { kind: CmsKind; page?: number }) {
  const list = await getPostList(kind, page);
  // Past the last page is a 404, not an empty grid (page 1 stays: it's the empty state).
  if (page > 1 && page > list.totalPages) notFound();
  return (
    <MarketingPage>
      <PostListing kind={kind} list={list} />
    </MarketingPage>
  );
}

/** Parse the /page/[page] segment. Page 1's canonical home is the section root. */
export async function pageNumber(kind: CmsKind, { params }: PageParams): Promise<number> {
  const { page } = await params;
  const n = Number(page);
  if (n === 1) permanentRedirect(CMS_BASE_PATH[kind]);
  if (!Number.isInteger(n) || n < 1) notFound();
  return n;
}

export async function postMetadata(kind: CmsKind, { params }: SlugParams): Promise<Metadata> {
  const { slug } = await params;
  const post = await getPost(kind, slug);
  if (!post) return { title: brand.name };
  const path = `${CMS_BASE_PATH[kind]}/${post.slug}`;
  return {
    title: `${post.seoTitle} — ${brand.name}`,
    description: post.seoDescription,
    ...(post.seoKeywords && { keywords: post.seoKeywords }),
    alternates: { canonical: path },
    openGraph: {
      type: "article",
      siteName: brand.name,
      url: path,
      title: post.seoTitle,
      description: post.seoDescription,
      ...(post.publishedAt && { publishedTime: post.publishedAt }),
      ...(post.author && { authors: [post.author] }),
      ...(post.image && { images: [{ url: post.image }] }),
    },
    twitter: {
      card: post.image ? "summary_large_image" : "summary",
      title: post.seoTitle,
      description: post.seoDescription,
      ...(post.image && { images: [post.image] }),
    },
  };
}

export async function PostRoute({ kind, params }: { kind: CmsKind } & SlugParams) {
  const { slug } = await params;
  const post = await getPost(kind, slug);
  if (!post) notFound();
  return (
    <MarketingPage>
      <PostArticle kind={kind} post={post} />
    </MarketingPage>
  );
}
