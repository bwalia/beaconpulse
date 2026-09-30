import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import type { ReactNode } from "react";

import { ArrowRightIcon, ClockIcon } from "@/components/icons";
import { MarketingNav } from "@/components/marketing/nav";
import { Footer } from "@/components/marketing/sections";
import { CMS_BASE_PATH, type CmsKind, type CmsPost, type CmsPostList, type CmsPostSummary } from "@/lib/cms";

// Server components for the CMS-backed Blog and Articles sections. Both sections share
// every view; `kind` only picks the base path and the copy.

/** The landing page's chrome (skip link, fixed nav, footer) around a CMS page. */
export async function MarketingPage({ children }: { children: ReactNode }) {
  const t = await getTranslations("marketing");
  return (
    <div className="min-h-dvh bg-white text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[60] focus:rounded-lg focus:bg-slate-900 focus:px-4 focus:py-2 focus:text-white dark:focus:bg-white dark:focus:text-slate-900"
      >
        {t("skipToContent")}
      </a>
      <MarketingNav />
      <main id="main" className="pb-24 pt-36 sm:pt-40">
        {children}
      </main>
      <Footer />
    </div>
  );
}

async function PostMeta({ post }: { post: CmsPostSummary }) {
  const t = await getTranslations("cms");
  const format = await getFormatter();
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-slate-500 dark:text-slate-400">
      {post.publishedAt && (
        <time dateTime={post.publishedAt}>
          {format.dateTime(new Date(post.publishedAt), { year: "numeric", month: "long", day: "numeric" })}
        </time>
      )}
      {post.readingMinutes > 0 && (
        <span className="inline-flex items-center gap-1.5">
          <ClockIcon className="h-4 w-4" />
          {t("minRead", { minutes: post.readingMinutes })}
        </span>
      )}
      {post.author && <span>{post.author}</span>}
    </div>
  );
}

function PostCard({ kind, post, readMore }: { kind: CmsKind; post: CmsPostSummary; readMore: string }) {
  return (
    <li className="group relative flex flex-col overflow-hidden rounded-2xl border border-slate-900/10 bg-white transition-shadow hover:shadow-lg motion-reduce:transition-none dark:border-white/10 dark:bg-slate-900">
      {post.image && (
        <div className="aspect-[16/9] overflow-hidden bg-slate-100 dark:bg-slate-800">
          {/* CMS images live on arbitrary hosts; next/image would need each one allow-listed. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={post.image} alt="" loading="lazy" className="h-full w-full object-cover" />
        </div>
      )}
      <div className="flex flex-1 flex-col gap-3 p-6">
        <h2 className="text-xl font-semibold tracking-tight text-slate-900 dark:text-white">
          {/* The stretched link makes the whole card clickable with one tab stop. */}
          <Link
            href={`${CMS_BASE_PATH[kind]}/${post.slug}`}
            className="after:absolute after:inset-0 focus:outline-none focus-visible:underline"
          >
            {post.title}
          </Link>
        </h2>
        {post.excerpt && (
          <p className="line-clamp-3 leading-relaxed text-slate-600 dark:text-slate-300">{post.excerpt}</p>
        )}
        <div className="mt-auto flex items-center justify-between gap-4 pt-2">
          <PostMeta post={post} />
          <span
            aria-hidden
            className="inline-flex shrink-0 items-center gap-1 text-sm font-medium text-brand-700 dark:text-brand-400"
          >
            {readMore}
            <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
          </span>
        </div>
      </div>
    </li>
  );
}

function pageHref(kind: CmsKind, page: number) {
  return page <= 1 ? CMS_BASE_PATH[kind] : `${CMS_BASE_PATH[kind]}/page/${page}`;
}

/** A section's listing: heading, card grid (or empty state), pagination. */
export async function PostListing({ kind, list }: { kind: CmsKind; list: CmsPostList }) {
  const t = await getTranslations("cms");
  const { posts, page, totalPages } = list;
  const pagerLink =
    "inline-flex items-center rounded-lg border border-slate-900/10 px-4 py-2 font-medium text-slate-700 transition-colors hover:bg-slate-900/5 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 motion-reduce:transition-none dark:border-white/10 dark:text-slate-200 dark:hover:bg-white/10";

  return (
    <div className="mx-auto w-full max-w-[1400px] px-6 sm:px-10 lg:px-16">
      <header className="mx-auto max-w-3xl text-center">
        <h1 className="text-balance text-5xl font-semibold tracking-tight text-slate-900 xl:text-6xl dark:text-white">
          {t(`${kind}.title`)}
        </h1>
        <p className="mt-5 text-xl leading-relaxed text-slate-600 dark:text-slate-300">
          {t(`${kind}.subtitle`)}
        </p>
      </header>

      {posts.length === 0 ? (
        <p className="mx-auto mt-16 max-w-xl rounded-2xl border border-dashed border-slate-900/15 p-10 text-center text-slate-500 dark:border-white/15 dark:text-slate-400">
          {t("empty")}
        </p>
      ) : (
        <ul className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {posts.map((p) => (
            <PostCard key={p.uuid || p.slug} kind={kind} post={p} readMore={t("readMore")} />
          ))}
        </ul>
      )}

      {totalPages > 1 && (
        <nav aria-label={t("pagination")} className="mt-14 flex items-center justify-center gap-4 text-sm">
          {page > 1 && (
            <Link href={pageHref(kind, page - 1)} rel="prev" className={pagerLink}>
              {t("newer")}
            </Link>
          )}
          <span className="text-slate-500 dark:text-slate-400">
            {t("pageOf", { page, total: totalPages })}
          </span>
          {page < totalPages && (
            <Link href={pageHref(kind, page + 1)} rel="next" className={pagerLink}>
              {t("older")}
            </Link>
          )}
        </nav>
      )}
    </div>
  );
}

/** A single post. The body is CMS-authored HTML from our own OpsAPI namespace. */
export async function PostArticle({ kind, post }: { kind: CmsKind; post: CmsPost }) {
  const t = await getTranslations("cms");
  return (
    <article className="mx-auto w-full max-w-3xl px-6 sm:px-10">
      <nav aria-label={t("breadcrumb")} className="text-sm text-slate-500 dark:text-slate-400">
        <Link href="/" className="hover:text-slate-900 dark:hover:text-white">
          {t("home")}
        </Link>
        <span aria-hidden className="mx-2">/</span>
        <Link href={CMS_BASE_PATH[kind]} className="hover:text-slate-900 dark:hover:text-white">
          {t(`${kind}.title`)}
        </Link>
      </nav>

      <header className="mt-6">
        {post.tags.length > 0 && (
          <ul className="mb-4 flex flex-wrap gap-2">
            {post.tags.map((tag) => (
              <li
                key={tag.slug}
                className="rounded-full bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700 dark:bg-brand-900/40 dark:text-brand-300"
              >
                {tag.name}
              </li>
            ))}
          </ul>
        )}
        <h1 className="text-balance text-4xl font-semibold tracking-tight text-slate-900 sm:text-5xl dark:text-white">
          {post.title}
        </h1>
        {post.excerpt && (
          <p className="mt-4 text-xl leading-relaxed text-slate-600 dark:text-slate-300">{post.excerpt}</p>
        )}
        <div className="mt-6">
          <PostMeta post={post} />
        </div>
      </header>

      {post.image && (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={post.image}
          alt=""
          className="mt-10 aspect-[16/9] w-full rounded-2xl object-cover"
        />
      )}

      <div
        className="prose-docs prose-cms mt-10 text-lg"
        dangerouslySetInnerHTML={{ __html: post.html }}
      />

      <div className="mt-16 border-t border-slate-900/10 pt-8 dark:border-white/10">
        <Link
          href={CMS_BASE_PATH[kind]}
          className="inline-flex items-center gap-2 font-medium text-brand-700 hover:underline dark:text-brand-400"
        >
          {t("backTo", { section: t(`${kind}.title`) })}
        </Link>
      </div>
    </article>
  );
}
