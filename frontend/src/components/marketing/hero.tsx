"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { ArrowRightIcon, CheckCircleIcon } from "@/components/icons";
import { DUR, EASE_OUT, IN_VIEW, useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { Spotlight, TiltCard } from "./pointer";

/**
 * Counts up to `to` when scrolled into view.
 *
 * Uses rAF rather than animating a React state on a timer: one state write per
 * frame, and it stops exactly on the target instead of drifting. Reduced-motion
 * users get the final value immediately — the number is the information, the
 * count is only flourish.
 */
function CountUp({ to, decimals = 0, suffix = "" }: { to: number; decimals?: number; suffix?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-40px" });
  const reduce = useReducedMotion();
  const [value, setValue] = useState(reduce ? to : 0);

  useEffect(() => {
    if (!inView || reduce) return;
    let raf = 0;
    const start = performance.now();
    const dur = 1100;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / dur);
      // ease-out cubic: fast, then settles — matches the site's entrance easing.
      setValue(to * (1 - Math.pow(1 - t, 3)));
      if (t < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [inView, reduce, to]);

  return (
    <span ref={ref} className="tabular-nums">
      {value.toFixed(decimals)}
      {suffix}
    </span>
  );
}

/** The mock status card. Deliberately the product's real visual language. */
function LiveCard() {
  const reduce = useReducedMotion();
  const rows = [
    { name: "api.acme.com", ms: 142, up: true },
    { name: "app.acme.com", ms: 88, up: true },
    { name: "checkout.acme.com", ms: 310, up: true },
    { name: "legacy.acme.com", ms: 0, up: false },
  ];

  return (
    <TiltCard className="w-full">
      <div className="rounded-2xl border border-slate-900/10 bg-white/70 p-7 shadow-2xl shadow-slate-900/10 backdrop-blur-xl dark:border-white/10 dark:bg-slate-900/60 dark:shadow-black/40">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2.5 w-2.5">
              {/* The ping is the only looping animation on the page — it earns its
                  place by signalling "this is live", not decoration. */}
              {!reduce && (
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-75" />
              )}
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-600" />
            </span>
            <span className="text-base font-medium text-slate-700 dark:text-slate-200">Live</span>
          </div>
          <span className="font-mono text-sm text-slate-500 dark:text-slate-400">30s interval</span>
        </div>

        <ul className="space-y-2">
          {rows.map((r, i) => (
            <motion.li
              key={r.name}
              initial={{ opacity: 0, x: reduce ? 0 : -8 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: 0.5 + i * 0.08, duration: DUR.base, ease: EASE_OUT }}
              className="flex items-center justify-between rounded-lg bg-slate-900/[0.03] px-4 py-3 dark:bg-white/[0.04]"
            >
              <span className="flex items-center gap-2.5 truncate">
                <span
                  aria-hidden
                  className={`h-2 w-2 shrink-0 rounded-full ${r.up ? "bg-emerald-600" : "bg-red-600"}`}
                />
                <span className="truncate font-mono text-base text-slate-700 dark:text-slate-200">
                  {r.name}
                </span>
              </span>
              {/* Status is never conveyed by colour alone — the label carries it. */}
              <span
                className={`ml-3 shrink-0 font-mono text-sm font-medium ${
                  r.up ? "text-emerald-700 dark:text-emerald-400" : "text-red-700 dark:text-red-400"
                }`}
              >
                {r.up ? `${r.ms}ms` : "down"}
              </span>
            </motion.li>
          ))}
        </ul>

        <div className="mt-4 flex items-center gap-2 border-t border-slate-900/10 pt-5 text-base text-slate-600 dark:border-white/10 dark:text-slate-300">
          <CheckCircleIcon className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
          <span>3 of 4 operational — alert sent 12s ago</span>
        </div>
      </div>
    </TiltCard>
  );
}

export function Hero() {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.07);

  return (
    <section className="relative overflow-hidden pb-24 pt-36 sm:pt-44">
      <Spotlight />

      {/* Grid + orbs. aria-hidden: pure texture, nothing to announce. */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10 bg-[linear-gradient(to_right,rgba(100,116,139,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(100,116,139,0.07)_1px,transparent_1px)] bg-[size:56px_56px] [mask-image:radial-gradient(ellipse_at_center,black_35%,transparent_75%)]"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute -top-40 left-1/2 -z-10 h-[520px] w-[820px] -translate-x-1/2 rounded-full bg-blue-500/10 blur-3xl dark:bg-blue-500/15"
      />

      <div className="relative mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16 grid items-center gap-16 lg:grid-cols-2 xl:gap-24">
        <motion.div initial="hidden" animate="show" variants={stagger}>
          <motion.div variants={reveal}>
            <span className="inline-flex items-center gap-2 rounded-full border border-slate-900/10 bg-white/60 px-3 py-1.5 text-base font-medium text-slate-700 backdrop-blur dark:border-white/15 dark:bg-white/5 dark:text-slate-200">
              <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-emerald-600" />
              Self-hosted &amp; multi-tenant
            </span>
          </motion.div>

          <motion.h1
            variants={reveal}
            className="mt-7 text-balance text-6xl font-semibold leading-[1.03] tracking-tight text-slate-900 sm:text-7xl xl:text-[5.25rem] dark:text-white"
          >
            Know it&apos;s down
            <br />
            <span className="bg-gradient-to-r from-blue-600 to-emerald-600 bg-clip-text text-transparent dark:from-blue-400 dark:to-emerald-400">
              before they do.
            </span>
          </motion.h1>

          <motion.p
            variants={reveal}
            className="mt-7 max-w-2xl text-xl leading-relaxed text-slate-600 xl:text-2xl dark:text-slate-300"
          >
            Beacon watches your endpoints, certificates and DNS every 30 seconds — then
            tells the right person the moment something breaks. Own your data, run it
            anywhere, and give customers a status page they actually trust.
          </motion.p>

          <motion.div variants={reveal} className="mt-9 flex flex-wrap items-center gap-3">
            <Link
              href="/register"
              className="group inline-flex items-center gap-2 rounded-xl bg-slate-900 px-7 py-4 text-lg font-medium text-white shadow-lg shadow-slate-900/20 transition-transform hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:bg-white dark:text-slate-900 dark:focus-visible:ring-offset-slate-950"
            >
              Start monitoring free
              <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
            </Link>
            <a
              href="#status"
              className="inline-flex items-center gap-2 rounded-xl border border-slate-900/15 bg-white/60 px-7 py-4 text-lg font-medium text-slate-800 backdrop-blur transition-colors hover:bg-white focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:border-white/15 dark:bg-white/5 dark:text-slate-100 dark:hover:bg-white/10"
            >
              See a status page
            </a>
          </motion.div>

          <motion.dl
            variants={reveal}
            className="mt-14 grid max-w-xl grid-cols-3 gap-8 border-t border-slate-900/10 pt-9 dark:border-white/10"
          >
            {[
              { v: <CountUp to={99.99} decimals={2} suffix="%" />, l: "Uptime tracked" },
              { v: <CountUp to={30} suffix="s" />, l: "Check interval" },
              { v: <CountUp to={6} />, l: "Monitor types" },
            ].map((s, i) => (
              <div key={i}>
                <dt className="sr-only">{s.l}</dt>
                <dd className="text-3xl font-semibold text-slate-900 xl:text-4xl dark:text-white">{s.v}</dd>
                <dd className="mt-1.5 text-base text-slate-500 dark:text-slate-400">{s.l}</dd>
              </div>
            ))}
          </motion.dl>
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 24 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: DUR.slow, ease: EASE_OUT, delay: 0.15 }}
          viewport={IN_VIEW}
        >
          <LiveCard />
        </motion.div>
      </div>
    </section>
  );
}
