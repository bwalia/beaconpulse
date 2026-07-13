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
import { Button, Card, Field, PageHeader, Skeleton } from "@/components/ui";
import {
  useMonitors,
  useSetMonitorPublic,
  useStatusPageSettings,
  useUpdateStatusPageSettings,
} from "@/lib/hooks";
import { DUR, useRevealVariants, useStaggerVariants } from "@/lib/motion";

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
  const { data: monitors } = useMonitors();
  const update = useUpdateStatusPageSettings();
  const setPublic = useSetMonitorPublic();

  const [title, setTitle] = useState("");
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants();

  // Seed the input once settings arrive, without clobbering what the user is
  // mid-way through typing.
  useEffect(() => {
    if (settings && title === "") setTitle(settings.title);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settings?.title]);

  if (isLoading || !settings) {
    return (
      <div className="space-y-4">
        <PageHeader title="Status page" subtitle="Publish a public page your customers can trust." />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  const published = monitors?.data?.filter((m) => m.public) ?? [];
  const enabledButEmpty = settings.enabled && published.length === 0;

  return (
    <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-6">
      <PageHeader
        title="Status page"
        subtitle="Publish a public page your customers can trust. Nothing is exposed until you say so."
      />

      {/* ---- Publish switch ---- */}
      <motion.div variants={reveal}>
        <Card className="p-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex gap-4">
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
                    className="group mt-3 inline-flex items-center gap-2 rounded-lg font-mono text-sm text-blue-700 underline-offset-4 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:text-blue-400"
                  >
                    {settings.url}
                    <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
                  </Link>
                )}
              </div>
            </div>

            <Button
              variant={settings.enabled ? "secondary" : "primary"}
              disabled={update.isPending}
              onClick={() => update.mutate({ enabled: !settings.enabled })}
              style={{ transitionDuration: `${DUR.micro}s` }}
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

      {/* ---- Title ---- */}
      <motion.div variants={reveal}>
        <Card className="p-6">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              update.mutate({ title });
            }}
            className="flex flex-wrap items-end gap-4"
          >
            <div className="min-w-[16rem] flex-1">
              <Field
                label="Public heading"
                hint={`Shown at the top of the page. Leave blank to use “${settings.org_name}”.`}
              >
                <input
                  value={title}
                  maxLength={120}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder={settings.org_name}
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-base text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
                />
              </Field>
            </div>
            <Button type="submit" variant="secondary" disabled={update.isPending}>
              Save heading
            </Button>
          </form>
        </Card>
      </motion.div>

      {/* ---- Which domains are public ---- */}
      <motion.div variants={reveal}>
        <Card className="overflow-hidden">
          <div className="flex items-center justify-between border-b border-slate-200 px-6 py-4 dark:border-slate-800">
            <div>
              <h2 className="font-semibold text-slate-900 dark:text-white">Published domains</h2>
              <p className="mt-0.5 text-sm text-slate-600 dark:text-slate-300">
                Only the name and status are ever exposed — never the target, IP or check
                configuration.
              </p>
            </div>
            <span className="shrink-0 font-mono text-sm tabular-nums text-slate-500 dark:text-slate-400">
              {published.length}/{monitors?.data?.length ?? 0}
            </span>
          </div>

          <ul className="divide-y divide-slate-200 dark:divide-slate-800">
            {(monitors?.data ?? []).map((m) => (
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

          {(monitors?.data?.length ?? 0) === 0 && (
            <p className="px-6 py-10 text-center text-slate-500 dark:text-slate-400">
              No monitors yet. Add one and it will appear here, private by default.
            </p>
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
