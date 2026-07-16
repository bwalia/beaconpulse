"use client";

import { motion, useReducedMotion } from "framer-motion";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { BeaconMark } from "@/components/icons";
import { useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { useHydrated, useNow } from "@/lib/time";
import type {
  PublicStatusGroup,
  PublicStatusMaintenance,
  PublicStatusMonitor,
  PublicStatusPage,
  StatusOverall,
} from "@/lib/types";

// The whole console renders in Departure Mono (loaded on the status route), with a
// monospace fallback so it degrades gracefully before the font paints.
const TERMINAL_FONT = "var(--font-departure), ui-monospace, SFMono-Regular, Menlo, monospace";

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

/**
 * "2 minutes ago". Derived from the shared clock rather than read during render, so
 * it ticks on its own and the server (which has no matching clock) renders nothing
 * instead of a number the browser would immediately contradict.
 */
function Ago({ iso }: { iso: string | null }) {
  const now = useNow(30_000);
  return (
    <span className="tabular-nums text-slate-500">{now === null ? "" : agoText(iso, now)}</span>
  );
}

function agoText(iso: string | null, now: number): string {
  if (!iso) return "NEVER";
  const secs = Math.max(0, (now - new Date(iso).getTime()) / 1000);
  if (secs < 60) return `${Math.floor(secs)}S AGO`;
  if (secs < 3600) return `${Math.floor(secs / 60)}M AGO`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}H AGO`;
  return `${Math.floor(secs / 86400)}D AGO`;
}

/** Local-time window bounds — gated on hydration because the server cannot know the
 *  reader's timezone or locale, and would render a string the browser replaces. */
function WindowWhen({ startsAt, endsAt }: { startsAt: string; endsAt: string }) {
  const hydrated = useHydrated();
  const fmt = (iso: string) =>
    new Date(iso)
      .toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
      .toUpperCase();
  return (
    <span className="tabular-nums text-orange-300/80">
      {hydrated ? `${fmt(startsAt)} → ${fmt(endsAt)}` : ""}
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

/**
 * Re-asks the server for this page on a timer, so an OPEN tab keeps up.
 *
 * The route's `revalidate` only makes the SERVER rebuild the page; it says nothing
 * about a browser that already has it. Without this the markup a visitor loaded is
 * the markup they keep — during an outage they would sit on "ALL SYSTEMS
 * OPERATIONAL" for as long as the tab stayed open, while the ticking "Ago" labels
 * made it look live. Stale is bad; stale and convincingly alive is worse.
 *
 * router.refresh() re-fetches this route rather than the API, which is the point:
 * the answer comes from the route cache, so ten thousand people watching an incident
 * cost one backend read per revalidate window instead of ten thousand. A status page
 * is read exactly when the infrastructure behind it is least able to take the load.
 */
function useLiveRefresh(intervalMs: number) {
  const router = useRouter();
  useEffect(() => {
    const refresh = () => router.refresh();
    // Hidden tabs don't poll — nobody is reading, and a status page can be left open
    // for days. Visibility change covers the return.
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") refresh();
    }, intervalMs);
    const onVisible = () => {
      if (document.visibilityState === "visible") refresh();
    };
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("online", refresh);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("online", refresh);
    };
  }, [router, intervalMs]);
}

export function StatusView({ page }: { page: PublicStatusPage }) {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.06);
  const reduceMotion = useReducedMotion();
  const o = OVERALL[page.overall] ?? OVERALL.unknown;

  // Half the route's revalidate window, deliberately. Revalidation is
  // stale-while-revalidate: the first request after the window expires still gets the
  // OLD render and merely triggers the rebuild, and only a later request sees the new
  // data. Polling at the window length would wait a full extra window for that second
  // request; polling at half lands it sooner. Measured end-to-end, a change published
  // mid-window surfaces in a tab within roughly 30-60s. The extra polls are answered
  // from the route cache and never reach the API.
  useLiveRefresh(15_000);

  return (
    <div
      className="relative min-h-dvh overflow-hidden bg-[#080a0f] text-slate-300 selection:bg-orange-500/30"
      style={{ fontFamily: TERMINAL_FONT }}
    >
      {/* Static CRT texture: scanlines + faint RGB grid + vignette. */}
      <div aria-hidden className="pointer-events-none fixed inset-0 z-0" style={CRT} />

      {/* The CRT refresh sweep — a soft bright band travelling top → bottom, on a
          loop. This is the "moving white line". Disabled under reduced-motion. */}
      {!reduceMotion && (
        <motion.div
          aria-hidden
          className="pointer-events-none fixed inset-x-0 z-20 h-40 bg-gradient-to-b from-transparent via-white/[0.07] to-transparent blur-[1px]"
          initial={{ y: "-30vh" }}
          animate={{ y: "130vh" }}
          transition={{ duration: 7, repeat: Infinity, ease: "linear", repeatDelay: 1.5 }}
        />
      )}

      <div className="relative z-10 mx-auto w-full max-w-3xl px-4 py-8 sm:py-12">
        <motion.div initial="hidden" animate="show" variants={stagger}>
          {/* Beacon header — the brand doubles as the way back to the main site. */}
          <motion.header
            variants={reveal}
            className="mb-5 flex items-center justify-between border-b border-slate-800 pb-4"
          >
            <Link
              href="/"
              className="group flex items-center gap-2.5 text-orange-400 focus:outline-none focus-visible:ring-1 focus-visible:ring-orange-400"
            >
              <BeaconMark className="h-5 w-5" />
              <span className="text-xs uppercase tracking-[0.2em] group-hover:text-orange-300 sm:text-sm">
                Beacon&nbsp;Pulse
              </span>
            </Link>
            <Link
              href="/"
              className="text-[11px] uppercase tracking-[0.25em] text-slate-500 underline-offset-4 hover:text-orange-400 hover:underline focus:outline-none focus-visible:ring-1 focus-visible:ring-orange-400"
            >
              ← Home
            </Link>
          </motion.header>

          {/* Terminal window */}
          <motion.div
            variants={reveal}
            className="border border-slate-700/70 bg-[#0b0d13] shadow-[0_0_40px_-12px_rgba(255,90,30,0.25)]"
          >
            {/* Title bar */}
            <div className="flex items-center justify-between border-b border-slate-700/70 bg-white/[0.02] px-4 py-2.5 text-[11px] uppercase tracking-widest">
              <span className="truncate text-slate-400">
                BEACONPULSE<span className="text-slate-600">{" // "}</span>
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
                <span className="text-slate-500">$</span> beaconpulse status --now
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
              <Link
                href="/"
                className="text-slate-500 underline-offset-4 hover:text-orange-400 hover:underline focus:outline-none focus-visible:ring-1 focus-visible:ring-orange-400"
              >
                POWERED BY BEACON PULSE
              </Link>
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
