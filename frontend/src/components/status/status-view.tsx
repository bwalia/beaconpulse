"use client";

import { motion } from "framer-motion";
import { useEffect, useState } from "react";

import {
  AlertTriangleIcon,
  BeaconMark,
  CheckCircleIcon,
  ClockIcon,
  WrenchIcon,
  XIcon,
} from "@/components/icons";
import { IN_VIEW, useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { ThemeToggle } from "@/lib/theme";
import type {
  PublicStatusGroup,
  PublicStatusMaintenance,
  PublicStatusMonitor,
  PublicStatusPage,
  StatusOverall,
} from "@/lib/types";

/**
 * Presentation for the public status page.
 *
 * Two rules run through everything below:
 *
 *   1. Status is never carried by colour alone. Every dot is paired with a word
 *      ("Operational", "Down"), because ~1 in 12 men cannot reliably separate the
 *      green from the red — and this is the one page where that matters most.
 *   2. The headline states the answer. A visitor should not have to scan rows to
 *      learn whether things are broken.
 */

const OVERALL: Record<
  StatusOverall,
  { label: string; tone: string; ring: string; Icon: typeof CheckCircleIcon }
> = {
  operational: {
    label: "All systems operational",
    // 700 on light / 400 on dark: both clear 4.5:1 against their surface.
    tone: "text-emerald-700 dark:text-emerald-400",
    ring: "bg-emerald-600/10",
    Icon: CheckCircleIcon,
  },
  degraded: {
    label: "Partial outage",
    tone: "text-amber-700 dark:text-amber-400",
    ring: "bg-amber-600/10",
    Icon: AlertTriangleIcon,
  },
  outage: {
    label: "Major outage",
    tone: "text-red-700 dark:text-red-400",
    ring: "bg-red-600/10",
    Icon: XIcon,
  },
  under_maintenance: {
    label: "Under maintenance",
    tone: "text-blue-700 dark:text-blue-300",
    ring: "bg-blue-600/10",
    Icon: WrenchIcon,
  },
  unknown: {
    label: "Awaiting first checks",
    tone: "text-slate-600 dark:text-slate-300",
    ring: "bg-slate-500/10",
    Icon: ClockIcon,
  },
};

const MONITOR: Record<
  PublicStatusMonitor["status"],
  { label: string; dot: string; tone: string }
> = {
  up: { label: "Operational", dot: "bg-emerald-600", tone: "text-emerald-700 dark:text-emerald-400" },
  down: { label: "Down", dot: "bg-red-600", tone: "text-red-700 dark:text-red-400" },
  degraded: { label: "Degraded", dot: "bg-amber-600", tone: "text-amber-700 dark:text-amber-400" },
  paused: { label: "Paused", dot: "bg-slate-400", tone: "text-slate-600 dark:text-slate-400" },
  unknown: { label: "No data", dot: "bg-slate-400", tone: "text-slate-600 dark:text-slate-400" },
};

// A monitor under an active window shows this neutral state as its headline pill.
// Its true probe state is still rendered beside it (muted) so a real failure that
// coincides with planned work is never hidden.
const MAINT = {
  label: "Under maintenance",
  dot: "bg-blue-500",
  tone: "text-blue-700 dark:text-blue-300",
} as const;

/** "2 minutes ago", rendered client-side to avoid an SSR/CSR clock mismatch. */
function Ago({ iso }: { iso: string | null }) {
  const [text, setText] = useState<string>("");

  useEffect(() => {
    if (!iso) {
      setText("never checked");
      return;
    }
    const compute = () => {
      const secs = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
      if (secs < 60) return `${Math.floor(secs)}s ago`;
      if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
      if (secs < 86400) return `${Math.floor(secs / 3600)}h ago`;
      return `${Math.floor(secs / 86400)}d ago`;
    };
    setText(compute());
    const t = setInterval(() => setText(compute()), 30_000);
    return () => clearInterval(t);
  }, [iso]);

  // suppressHydrationWarning: the server has no "now", so this text is
  // intentionally client-only. Rendering nothing on the server keeps the markup
  // identical in both passes.
  return (
    <span suppressHydrationWarning className="font-mono text-xs text-slate-500 dark:text-slate-400">
      {text}
    </span>
  );
}

/** A calendar window rendered as "1 Jan, 14:00 → 16:00", client-side to dodge an
 *  SSR/CSR locale mismatch. */
function WindowWhen({ startsAt, endsAt }: { startsAt: string; endsAt: string }) {
  const [text, setText] = useState<string>("");
  useEffect(() => {
    const fmt = (iso: string) =>
      new Date(iso).toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    setText(`${fmt(startsAt)} → ${fmt(endsAt)}`);
  }, [startsAt, endsAt]);
  return (
    <span suppressHydrationWarning className="font-mono text-xs text-blue-700/80 dark:text-blue-300/80">
      {text}
    </span>
  );
}

function MaintenanceBanner({ windows }: { windows: PublicStatusMaintenance[] }) {
  const reveal = useRevealVariants();
  return (
    <motion.div
      variants={reveal}
      aria-live="polite"
      className="mt-6 rounded-2xl border border-blue-600/20 bg-blue-50 p-5 dark:border-blue-400/20 dark:bg-blue-950/40"
    >
      <div className="flex items-center gap-2.5">
        <WrenchIcon className="h-5 w-5 shrink-0 text-blue-700 dark:text-blue-300" />
        <p className="font-semibold text-blue-800 dark:text-blue-200">Scheduled maintenance</p>
      </div>
      <ul className="mt-3 space-y-2">
        {windows.map((mw, i) => (
          <li key={`${mw.title}-${i}`} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <span className="text-sm text-blue-900 dark:text-blue-100">{mw.title}</span>
            <WindowWhen startsAt={mw.starts_at} endsAt={mw.ends_at} />
          </li>
        ))}
      </ul>
    </motion.div>
  );
}

function Group({ group }: { group: PublicStatusGroup }) {
  const reveal = useRevealVariants();
  const up = group.monitors.filter((m) => m.status === "up").length;

  return (
    <motion.section variants={reveal} className="px-6 py-5">
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="font-medium text-slate-900 dark:text-white">
          {group.name}
          <span className="ml-2 text-sm font-normal capitalize text-slate-500 dark:text-slate-400">
            {group.environment}
          </span>
        </h2>
        <span className="shrink-0 font-mono text-sm tabular-nums text-slate-500 dark:text-slate-400">
          {up}/{group.monitors.length} up
        </span>
      </div>

      <ul className="mt-3 space-y-2">
        {group.monitors.map((m) => {
          const s = MONITOR[m.status] ?? MONITOR.unknown;
          return (
            <li
              key={m.name}
              className="flex items-center justify-between gap-4 rounded-lg bg-slate-900/[0.03] px-3 py-2.5 dark:bg-white/[0.04]"
            >
              <span className="min-w-0 truncate font-mono text-sm text-slate-700 dark:text-slate-200">
                {m.name}
              </span>
              <span className="flex shrink-0 items-center gap-3">
                <Ago iso={m.last_checked_at} />
                {m.in_maintenance ? (
                  <>
                    {/* True probe state, muted, so a real failure during planned
                        work is still visible — just not the headline. */}
                    <span className={`hidden items-center gap-1.5 text-xs font-medium opacity-60 sm:flex ${s.tone}`}>
                      <span aria-hidden className={`h-1.5 w-1.5 rounded-full ${s.dot}`} />
                      {s.label}
                    </span>
                    <span className={`flex items-center gap-1.5 text-xs font-medium ${MAINT.tone}`}>
                      <span aria-hidden className={`h-1.5 w-1.5 rounded-full ${MAINT.dot}`} />
                      {MAINT.label}
                    </span>
                  </>
                ) : (
                  /* dot + word: never colour alone */
                  <span className={`flex items-center gap-1.5 text-xs font-medium ${s.tone}`}>
                    <span aria-hidden className={`h-1.5 w-1.5 rounded-full ${s.dot}`} />
                    {s.label}
                  </span>
                )}
              </span>
            </li>
          );
        })}
      </ul>
    </motion.section>
  );
}

export function StatusView({ page }: { page: PublicStatusPage }) {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.06);
  const o = OVERALL[page.overall] ?? OVERALL.unknown;
  const Icon = o.Icon;

  return (
    <div className="min-h-dvh bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <header className="border-b border-slate-900/10 bg-white/70 backdrop-blur dark:border-white/10 dark:bg-slate-900/50">
        <div className="mx-auto flex w-full max-w-3xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-2.5">
            <BeaconMark className="h-6 w-6 text-blue-600 dark:text-blue-400" />
            <span className="font-semibold tracking-tight">{page.title}</span>
          </div>
          <ThemeToggle />
        </div>
      </header>

      <main className="mx-auto w-full max-w-3xl px-6 py-10">
        <motion.div initial="hidden" animate="show" variants={stagger}>
          {/* The answer, first. aria-live so a screen reader announces a change
              if the page is refreshed in place. */}
          <motion.div
            variants={reveal}
            aria-live="polite"
            className="flex items-center gap-4 rounded-2xl border border-slate-900/10 bg-white p-6 shadow-sm dark:border-white/10 dark:bg-slate-900"
          >
            <span className={`inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-xl ${o.ring}`}>
              <Icon className={`h-6 w-6 ${o.tone}`} />
            </span>
            <div className="min-w-0">
              <p className={`text-xl font-semibold ${o.tone}`}>{o.label}</p>
              <p className="mt-0.5 text-sm text-slate-500 dark:text-slate-400">
                <Ago iso={page.updated_at} /> · refreshes every 30s
              </p>
            </div>
          </motion.div>

          {page.maintenances.length > 0 && <MaintenanceBanner windows={page.maintenances} />}

          <motion.div
            variants={reveal}
            viewport={IN_VIEW}
            className="mt-6 overflow-hidden rounded-2xl border border-slate-900/10 bg-white shadow-sm dark:border-white/10 dark:bg-slate-900"
          >
            <motion.div variants={stagger} className="divide-y divide-slate-900/5 dark:divide-white/5">
              {page.groups.map((g) => (
                <Group key={g.name} group={g} />
              ))}
            </motion.div>
          </motion.div>

          <motion.p
            variants={reveal}
            className="mt-8 text-center text-sm text-slate-500 dark:text-slate-400"
          >
            Status for {page.org_name} · powered by{" "}
            <a
              href="/"
              className="rounded font-medium text-blue-700 underline-offset-4 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:text-blue-400"
            >
              Beacon
            </a>
          </motion.p>
        </motion.div>
      </main>
    </div>
  );
}
