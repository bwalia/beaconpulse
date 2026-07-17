"use client";

import { motion } from "framer-motion";
import Link from "next/link";
import { useEffect, useState } from "react";

import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CheckCircleIcon,
  GlobeIcon,
  LockIcon,
} from "@/components/icons";
import { Button, Card, Label, PageHeader, Skeleton } from "@/components/ui";
import { Pagination, SearchInput } from "@/components/table-controls";
import { ApiRequestError } from "@/lib/api";
import {
  useMonitorsPage,
  useSetMonitorPublic,
  useStatusPageSettings,
  useUpdateStatusPageSettings,
} from "@/lib/hooks";
import { DUR, useRevealVariants, useStaggerVariants } from "@/lib/motion";

const DOMAINS_PAGE_SIZE = 10;

/**
 * Owner controls for the public status page.
 *
 * The page is built around one idea: publishing is a security decision, so the
 * UI must never let it happen by accident, and must always show exactly what is
 * currently exposed. Hence the explicit per-monitor list rather than a single
 * "publish everything" switch.
 */
export default function StatusPageSettings() {
  const { data: settings, isLoading } = useStatusPageSettings();
  const update = useUpdateStatusPageSettings();
  const setPublic = useSetMonitorPublic();

  const [domainPage, setDomainPage] = useState(0);
  const [domainSearchInput, setDomainSearchInput] = useState("");
  const [domainSearch, setDomainSearch] = useState("");
  // The debounced search and the page reset land together: a new query has a
  // different first page, so they are one change, not an effect reconciling the
  // second after the first.
  useEffect(() => {
    const t = setTimeout(() => {
      setDomainSearch(domainSearchInput.trim());
      setDomainPage(0);
    }, 300);
    return () => clearTimeout(t);
  }, [domainSearchInput]);
  const { data: monitors, isPlaceholderData: monitorsBusy } = useMonitorsPage({
    page: domainPage,
    pageSize: DOMAINS_PAGE_SIZE,
    search: domainSearch || undefined,
  });

  // Both editors follow the same rule: null means "untouched", so the field shows
  // whatever the server has until the user actually types. That makes the saved
  // value the single source of truth — no effect copying it into state once it
  // arrives, and no chance a refetch overwrites an edit in progress. It also fixes
  // the older `title === ""` seeding, which treated a deliberately-cleared field as
  // untouched and silently refilled it on the next refetch.
  const [titleEdit, setTitleEdit] = useState<string | null>(null);
  const [slug, setSlug] = useState<string | null>(null);
  const [slugError, setSlugError] = useState<string | null>(null);
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants();

  if (isLoading || !settings) {
    return (
      <div className="space-y-4">
        <PageHeader title="Status page" subtitle="Publish a public page your customers can trust." />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  // Derived, not copied: the saved title shows through until the user edits.
  const title = titleEdit ?? settings.title;

  // The authoritative published count comes from the settings API (org-wide), not
  // the current monitor page — so the "enabled but empty" warning is correct even
  // when the list below is paginated or filtered.
  const monitorRows = monitors?.data ?? [];
  const monitorTotal = monitors?.pagination.total ?? 0;
  const domainSearching = domainSearch !== "";
  const enabledButEmpty = settings.enabled && settings.published_count === 0;

  return (
    <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-6">
      <PageHeader
        title="Status page"
        subtitle="Publish a public page your customers can trust. Nothing is exposed until you say so."
      />

      {/* ---- Publish switch ---- */}
      <motion.div variants={reveal}>
        <Card className="p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 gap-4">
              <span
                className={`inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ${
                  settings.enabled ? "bg-emerald-600/10" : "bg-slate-500/10"
                }`}
              >
                {settings.enabled ? (
                  <GlobeIcon className="h-6 w-6 text-emerald-700 dark:text-emerald-400" />
                ) : (
                  <LockIcon className="h-6 w-6 text-slate-600 dark:text-slate-400" />
                )}
              </span>
              <div>
                <h2 className="text-lg font-semibold text-slate-900 dark:text-white">
                  {settings.enabled ? "Your status page is live" : "Your status page is private"}
                </h2>
                <p className="mt-1 text-slate-600 dark:text-slate-300">
                  {settings.enabled
                    ? "Anyone with the link can see the domains you have published — and nothing else."
                    : "Turn this on to publish a page at the address below."}
                </p>

                {settings.enabled && (
                  <Link
                    href={settings.url}
                    target="_blank"
                    className="group mt-3 inline-flex max-w-full items-center gap-2 rounded-lg font-mono text-sm text-blue-700 underline-offset-4 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:text-blue-400"
                  >
                    <span className="break-all">{settings.url}</span>
                    <ArrowRightIcon className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
                  </Link>
                )}
              </div>
            </div>

            <Button
              variant={settings.enabled ? "secondary" : "primary"}
              disabled={update.isPending}
              onClick={() => update.mutate({ enabled: !settings.enabled })}
              style={{ transitionDuration: `${DUR.micro}s` }}
              className="w-full shrink-0 sm:w-auto"
            >
              {update.isPending
                ? "Saving…"
                : settings.enabled
                  ? "Unpublish"
                  : "Publish status page"}
            </Button>
          </div>

          {/* The "enabled but empty" trap: a live page showing nothing is worse
              than no page at all, so we say so plainly instead of letting them
              discover it from a customer. */}
          {enabledButEmpty && (
            <p
              role="alert"
              className="mt-5 flex items-start gap-2 rounded-lg bg-amber-500/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-300"
            >
              <AlertTriangleIcon className="mt-0.5 h-4 w-4 shrink-0" />
              <span>
                This page is live but has no domains on it. Publish at least one below, or
                visitors will see an empty page.
              </span>
            </p>
          )}
        </Card>
      </motion.div>

      {/* ---- Page address (custom slug) ---- */}
      <motion.div variants={reveal}>
        <Card className="p-6">
          <h2 className="font-semibold text-slate-900 dark:text-white">Page address</h2>
          <p className="mt-0.5 text-sm text-slate-600 dark:text-slate-300">
            The web address for your status page. Letters, numbers and hyphens — spaces and capitals
            are tidied automatically.
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setSlugError(null);
              update.mutate(
                { slug: slug ?? settings.custom_slug },
                {
                  onError: (err) =>
                    setSlugError(err instanceof ApiRequestError ? err.message : "Could not save the address"),
                },
              );
            }}
            className="mt-4"
          >
            <Label htmlFor="status-slug">Custom URL</Label>
            {/* Label on top, [input + buttons] in one stretch row (so they share a
                height and align), hint below. Stacks full-width on mobile. */}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-stretch">
              <div
                className={`flex min-w-0 flex-1 items-stretch overflow-hidden rounded-lg border bg-white focus-within:ring-2 focus-within:ring-blue-600 dark:bg-slate-900 ${
                  slugError ? "border-red-500" : "border-slate-300 dark:border-slate-700"
                }`}
              >
                <span className="flex items-center whitespace-nowrap bg-slate-50 px-3 font-mono text-sm text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                  /status/
                </span>
                <input
                  id="status-slug"
                  value={slug ?? settings.custom_slug}
                  maxLength={63}
                  onChange={(e) => {
                    setSlug(e.target.value);
                    setSlugError(null);
                  }}
                  placeholder={settings.org_slug}
                  aria-invalid={slugError ? true : undefined}
                  className="w-full min-w-0 bg-transparent px-3 py-2.5 font-mono text-base text-slate-900 focus:outline-none dark:text-white"
                />
              </div>
              <div className="flex gap-3">
                <Button
                  type="submit"
                  variant="secondary"
                  disabled={update.isPending}
                  className="flex-1 sm:flex-none"
                >
                  Save address
                </Button>
                {settings.custom_slug && (
                  <Button
                    type="button"
                    variant="ghost"
                    disabled={update.isPending}
                    className="flex-1 sm:flex-none"
                    onClick={() => {
                      setSlug("");
                      setSlugError(null);
                      update.mutate({ slug: "" });
                    }}
                  >
                    Reset to default
                  </Button>
                )}
              </div>
            </div>
            {slugError ? (
              <p role="alert" className="mt-1.5 text-xs font-medium text-red-700 dark:text-red-400">
                {slugError}
              </p>
            ) : (
              <p className="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
                Leave blank to use the default: <span className="font-mono">/status/{settings.org_slug}</span>
              </p>
            )}
          </form>
        </Card>
      </motion.div>

      {/* ---- Title ---- */}
      <motion.div variants={reveal}>
        <Card className="p-6">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              update.mutate({ title });
            }}
          >
            <Label htmlFor="status-heading">Public heading</Label>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-stretch">
              <input
                id="status-heading"
                value={title}
                maxLength={120}
                onChange={(e) => setTitleEdit(e.target.value)}
                placeholder={settings.org_name}
                className="w-full min-w-0 flex-1 rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-base text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
              />
              <Button type="submit" variant="secondary" disabled={update.isPending} className="sm:flex-none">
                Save heading
              </Button>
            </div>
            <p className="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
              Shown at the top of the page. Leave blank to use “{settings.org_name}”.
            </p>
          </form>
        </Card>
      </motion.div>

      {/* ---- Which domains are public ---- */}
      <motion.div variants={reveal}>
        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-6 py-4 dark:border-slate-800">
            <div>
              <h2 className="font-semibold text-slate-900 dark:text-white">Published domains</h2>
              <p className="mt-0.5 text-sm text-slate-600 dark:text-slate-300">
                Only the name and status are ever exposed — never the target, IP or check
                configuration.
              </p>
            </div>
            <span className="shrink-0 font-mono text-sm tabular-nums text-slate-600 dark:text-slate-300">
              {settings.published_count} of {monitorTotal} public
            </span>
          </div>

          {(monitorTotal > 0 || domainSearching) && (
            <div className="border-b border-slate-200 px-6 py-3 dark:border-slate-800">
              <SearchInput
                value={domainSearchInput}
                onChange={setDomainSearchInput}
                placeholder="Search monitors…"
                label="Search monitors to publish"
              />
            </div>
          )}

          <ul
            className={`divide-y divide-slate-200 dark:divide-slate-800 ${monitorsBusy ? "opacity-60 transition-opacity" : "transition-opacity"}`}
          >
            {monitorRows.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-4 px-6 py-4">
                <div className="min-w-0">
                  <p className="truncate font-medium text-slate-900 dark:text-white">{m.name}</p>
                  {/* The target is shown HERE (authenticated) precisely because it
                      is what will NOT appear on the public page. */}
                  <p className="mt-0.5 truncate font-mono text-xs text-slate-500 dark:text-slate-400">
                    {m.target} · stays private
                  </p>
                </div>

                <label className="flex shrink-0 cursor-pointer items-center gap-2.5">
                  <span className="text-sm text-slate-600 dark:text-slate-300">
                    {m.public ? "Public" : "Private"}
                  </span>
                  <input
                    type="checkbox"
                    checked={m.public}
                    disabled={setPublic.isPending}
                    onChange={(e) => setPublic.mutate({ id: m.id, isPublic: e.target.checked })}
                    className="h-5 w-5 cursor-pointer rounded border-slate-300 text-blue-600 focus:ring-2 focus:ring-blue-600 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:border-slate-700 dark:bg-slate-900 dark:focus:ring-offset-slate-950"
                    aria-label={`Publish ${m.name} on the public status page`}
                  />
                </label>
              </li>
            ))}
          </ul>

          {monitorTotal === 0 && (
            <p className="px-6 py-10 text-center text-slate-500 dark:text-slate-400">
              {domainSearching
                ? "No monitors match your search."
                : "No monitors yet. Add one and it will appear here, private by default."}
            </p>
          )}

          {monitorTotal > 0 && (
            <div className="border-t border-slate-200 px-6 py-3 dark:border-slate-800">
              <Pagination
                page={domainPage}
                pageSize={DOMAINS_PAGE_SIZE}
                total={monitorTotal}
                unit="monitors"
                busy={monitorsBusy}
                onPageChange={setDomainPage}
              />
            </div>
          )}
        </Card>
      </motion.div>

      <motion.p
        variants={reveal}
        className="flex items-center justify-center gap-2 pb-4 text-sm text-slate-500 dark:text-slate-400"
      >
        <CheckCircleIcon className="h-4 w-4" />
        Domains are grouped on the public page by their project.
      </motion.p>
    </motion.div>
  );
}
