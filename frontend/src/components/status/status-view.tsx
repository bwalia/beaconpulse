"use client";

import { motion } from "framer-motion";
import { useEffect, useState } from "react";

import { useRevealVariants, useStaggerVariants } from "@/lib/motion";
import type {
  PublicStatusGroup,
  PublicStatusMaintenance,
  PublicStatusMonitor,
  PublicStatusPage,
  StatusOverall,
} from "@/lib/types";

/**
 * The PUBLIC status page, styled as an old-school terminal / CRT console.
 *
 * The look is deliberately monospace-and-scanlines, but two rules from before
 * still hold and matter most here:
 *   1. Status is never colour alone — every state is a coloured dot PLUS an
 *      uppercase word, because ~1 in 12 men can't reliably tell the green from the
 *      red, and this is the page where that matters most.
 *   2. The headline states the answer first, in a live region, so a screen reader
 *      announces a change without the visitor scanning every row.
 *
 * It commits to a dark palette on purpose: a terminal is dark, and this is a
 * branded standalone page, not a themed app surface.
 */

// `bar` is the cursor-block background, kept as a literal class (not derived from
// `color`) so Tailwind's scanner actually generates it.
const OVERALL: Record<StatusOverall, { label: string; color: string; bar: string }> = {
  operational: { label: "ALL SYSTEMS OPERATIONAL", color: "text-emerald-400", bar: "bg-emerald-400" },
  degraded: { label: "PARTIAL DEGRADATION", color: "text-amber-400", bar: "bg-amber-400" },
  outage: { label: "MAJOR OUTAGE", color: "text-red-400", bar: "bg-red-400" },
  under_maintenance: { label: "UNDER MAINTENANCE", color: "text-orange-400", bar: "bg-orange-400" },
  unknown: { label: "AWAITING FIRST CHECKS", color: "text-slate-400", bar: "bg-slate-400" },
};

const MONITOR: Record<PublicStatusMonitor["status"], { label: string; dot: string; text: string }> = {
  up: { label: "OPERATIONAL", dot: "bg-emerald-400", text: "text-emerald-400" },
  down: { label: "DOWN", dot: "bg-red-500", text: "text-red-400" },
  degraded: { label: "DEGRADED", dot: "bg-amber-400", text: "text-amber-400" },
  paused: { label: "PAUSED", dot: "bg-slate-500", text: "text-slate-400" },
  unknown: { label: "NO DATA", dot: "bg-slate-600", text: "text-slate-500" },
};

const MAINT = { label: "UNDER MAINTENANCE", dot: "bg-orange-400", text: "text-orange-400" } as const;

// The CRT layer: horizontal scanlines, a faint RGB vertical grid, and a vignette.
// Purely decorative, so pointer-events-none and aria-hidden.
const CRT: React.CSSProperties = {
  backgroundImage: [
    "repeating-linear-gradient(0deg, rgba(255,255,255,0.035) 0px, rgba(255,255,255,0.035) 1px, transparent 1px, transparent 3px)",
    "repeating-linear-gradient(90deg, rgba(255,80,30,0.020) 0px, rgba(80,255,140,0.018) 1px, rgba(60,120,255,0.020) 2px, transparent 3px)",
    "radial-gradient(120% 90% at 50% 0%, transparent 55%, rgba(0,0,0,0.55) 100%)",
  ].join(","),
};

/** "2 minutes ago", rendered client-side to avoid an SSR/CSR clock mismatch. */
function Ago({ iso }: { iso: string | null }) {
  const [text, setText] = useState<string>("");
  useEffect(() => {
    if (!iso) {
      setText("NEVER");
      return;
    }
    const compute = () => {
      const secs = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
      if (secs < 60) return `${Math.floor(secs)}S AGO`;
      if (secs < 3600) return `${Math.floor(secs / 60)}M AGO`;
      if (secs < 86400) return `${Math.floor(secs / 3600)}H AGO`;
      return `${Math.floor(secs / 86400)}D AGO`;
    };
    setText(compute());
    const t = setInterval(() => setText(compute()), 30_000);
    return () => clearInterval(t);
  }, [iso]);
  return (
    <span suppressHydrationWarning className="tabular-nums text-slate-500">
      {text}
    </span>
  );
}

function WindowWhen({ startsAt, endsAt }: { startsAt: string; endsAt: string }) {
  const [text, setText] = useState<string>("");
  useEffect(() => {
    const fmt = (iso: string) =>
      new Date(iso)
        .toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
        .toUpperCase();
    setText(`${fmt(startsAt)} → ${fmt(endsAt)}`);
  }, [startsAt, endsAt]);
  return (
    <span suppressHydrationWarning className="tabular-nums text-orange-300/80">
      {text}
    </span>
  );
}

function Row({ m }: { m: PublicStatusMonitor }) {
  const s = MONITOR[m.status] ?? MONITOR.unknown;
  const shown = m.in_maintenance ? MAINT : s;
  return (
    <li className="flex items-center gap-3 py-1.5 text-sm">
      <span className="truncate text-slate-300">{m.name}</span>
      {/* dotted leader — classic terminal fill between label and value */}
      <span aria-hidden className="mt-2 min-w-6 flex-1 border-b border-dotted border-slate-700/70" />
      <Ago iso={m.last_checked_at} />
      {/* If under maintenance, show the true probe state muted so a coincident
          failure isn't hidden, then the maintenance state as the headline. */}
      {m.in_maintenance && (
        <span className={`hidden items-center gap-1.5 opacity-50 sm:flex ${s.text}`}>
          <span aria-hidden className={`h-1.5 w-1.5 ${s.dot}`} />
          {s.label}
        </span>
      )}
      <span className={`flex shrink-0 items-center gap-1.5 font-medium ${shown.text}`}>
        <span aria-hidden className={`h-1.5 w-1.5 ${shown.dot}`} />
        {shown.label}
      </span>
    </li>
  );
}

function Group({ group }: { group: PublicStatusGroup }) {
  const reveal = useRevealVariants();
  const up = group.monitors.filter((m) => m.status === "up").length;
  return (
    <motion.section variants={reveal} className="px-4 py-4 sm:px-5">
      <div className="flex items-baseline gap-3 text-xs uppercase tracking-widest text-slate-500">
        <span className="text-slate-400">{group.name}</span>
        <span className="text-slate-600">[{group.environment}]</span>
        <span aria-hidden className="min-w-6 flex-1 border-b border-slate-800" />
        <span className="tabular-nums text-slate-400">
          {up}/{group.monitors.length} UP
        </span>
      </div>
      <ul className="mt-1">
        {group.monitors.map((m) => (
          <Row key={m.name} m={m} />
        ))}
      </ul>
    </motion.section>
  );
}

function MaintenanceBlock({ windows }: { windows: PublicStatusMaintenance[] }) {
  const reveal = useRevealVariants();
  return (
    <motion.div
      variants={reveal}
      aria-live="polite"
      className="mx-4 mb-4 border border-orange-500/40 bg-orange-500/[0.06] p-3.5 sm:mx-5"
    >
      <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-orange-400">
        <span aria-hidden>▲</span> Scheduled maintenance
      </p>
      <ul className="mt-2 space-y-1 text-sm">
        {windows.map((mw, i) => (
          <li key={`${mw.title}-${i}`} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-0.5">
            <span className="text-orange-100/90">{mw.title}</span>
            <WindowWhen startsAt={mw.starts_at} endsAt={mw.ends_at} />
          </li>
        ))}
      </ul>
    </motion.div>
  );
}

export function StatusView({ page }: { page: PublicStatusPage }) {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.06);
  const o = OVERALL[page.overall] ?? OVERALL.unknown;

  return (
    <div className="relative min-h-dvh overflow-hidden bg-[#080a0f] font-mono text-slate-300 selection:bg-orange-500/30">
      <div aria-hidden className="pointer-events-none fixed inset-0 z-0" style={CRT} />

      <div className="relative z-10 mx-auto w-full max-w-3xl px-4 py-8 sm:py-12">
        <motion.div initial="hidden" animate="show" variants={stagger}>
          {/* Brand line */}
          <motion.div
            variants={reveal}
            className="mb-4 flex items-center justify-between text-xs uppercase tracking-[0.25em] text-slate-500"
          >
            <span className="flex items-center gap-2 text-orange-400">
              <span aria-hidden className="inline-block h-3 w-3 rotate-45 border border-orange-400" />
              BEACON
            </span>
            <span>STATUS CONSOLE</span>
          </motion.div>

          {/* Terminal window */}
          <motion.div
            variants={reveal}
            className="border border-slate-700/70 bg-[#0b0d13] shadow-[0_0_40px_-12px_rgba(255,90,30,0.25)]"
          >
            {/* Title bar */}
            <div className="flex items-center justify-between border-b border-slate-700/70 bg-white/[0.02] px-4 py-2.5 text-[11px] uppercase tracking-widest">
              <span className="truncate text-slate-400">
                BEACON<span className="text-slate-600"> // </span>
                <span className="text-slate-300">{page.title}</span>
              </span>
              <span className="flex shrink-0 items-center gap-2 text-emerald-400">
                <span aria-hidden className="h-2 w-2 rounded-full bg-emerald-400 motion-safe:animate-pulse" />
                LIVE
              </span>
            </div>

            {/* Headline: the answer, first, in a live region */}
            <motion.div variants={reveal} aria-live="polite" className="border-b border-slate-800 px-4 py-5 sm:px-5">
              <p className="text-[11px] uppercase tracking-[0.25em] text-slate-600">
                <span className="text-slate-500">$</span> beacon status --now
              </p>
              <p className={`mt-2 flex items-center gap-2 text-xl font-semibold tracking-wide sm:text-2xl ${o.color}`}>
                <span className="text-slate-600">&gt;</span>
                {o.label}
                <span aria-hidden className={`inline-block h-5 w-2.5 ${o.bar} motion-safe:animate-pulse`} />
              </p>
              <p className="mt-1.5 text-xs uppercase tracking-widest text-slate-500">
                LAST SYNC <Ago iso={page.updated_at} /> <span className="text-slate-700">·</span> REFRESH 30S
              </p>
            </motion.div>

            {page.maintenances.length > 0 && (
              <div className="pt-4">
                <MaintenanceBlock windows={page.maintenances} />
              </div>
            )}

            {/* Groups */}
            <motion.div variants={stagger} className="divide-y divide-slate-800/70">
              {page.groups.map((g) => (
                <Group key={g.name} group={g} />
              ))}
            </motion.div>

            {/* Footer strip */}
            <div className="flex items-center justify-between border-t border-slate-700/70 bg-white/[0.02] px-4 py-2.5 text-[11px] uppercase tracking-widest text-slate-600">
              <span>{page.org_name}</span>
              <a
                href="/"
                className="text-slate-500 underline-offset-4 hover:text-orange-400 hover:underline focus:outline-none focus-visible:ring-1 focus-visible:ring-orange-400"
              >
                POWERED BY BEACON
              </a>
            </div>
          </motion.div>

          <motion.p variants={reveal} className="mt-4 text-center text-[11px] uppercase tracking-[0.25em] text-slate-700">
            ▮ end of transmission
          </motion.p>
        </motion.div>
      </div>
    </div>
  );
}
