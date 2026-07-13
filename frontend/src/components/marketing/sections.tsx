"use client";

import { motion } from "framer-motion";
import Link from "next/link";
import type { ReactNode } from "react";

import {
  ActivityIcon,
  AlertTriangleIcon,
  ArrowRightIcon,
  BeaconMark,
  BellIcon,
  ChartLineIcon,
  CheckCircleIcon,
  ClockIcon,
  FolderIcon,
  GaugeIcon,
  LockIcon,
} from "@/components/icons";
import { IN_VIEW, useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { GlowCard } from "./pointer";

/** Section heading with a reveal. Shared so every section has one rhythm. */
function SectionHead({
  eyebrow,
  title,
  blurb,
}: {
  eyebrow: string;
  title: ReactNode;
  blurb: string;
}) {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants();
  return (
    <motion.div
      initial="hidden"
      whileInView="show"
      viewport={IN_VIEW}
      variants={stagger}
      className="mx-auto max-w-2xl text-center"
    >
      <motion.p
        variants={reveal}
        className="text-sm font-semibold uppercase tracking-widest text-blue-700 dark:text-blue-400"
      >
        {eyebrow}
      </motion.p>
      <motion.h2
        variants={reveal}
        className="mt-3 text-balance text-4xl font-semibold tracking-tight text-slate-900 dark:text-white"
      >
        {title}
      </motion.h2>
      <motion.p
        variants={reveal}
        className="mt-4 text-lg leading-relaxed text-slate-600 dark:text-slate-300"
      >
        {blurb}
      </motion.p>
    </motion.div>
  );
}

const FEATURES = [
  {
    icon: ActivityIcon,
    title: "Twelve monitor types",
    body: "HTTP, TCP, ICMP, DNS, SSL expiry, Kubernetes, Prometheus and more — one system instead of five.",
    wide: true,
  },
  {
    icon: BellIcon,
    title: "Alerts that reach a human",
    body: "Route to Slack, email or webhooks with de-duplication, so one outage isn't fifty pings.",
  },
  {
    icon: LockIcon,
    title: "Multi-tenant by design",
    body: "Every query is scoped to an org. Teams share an instance without sharing data.",
  },
  {
    icon: GaugeIcon,
    title: "Latency you can act on",
    body: "p50/p95 response times per endpoint, not just a binary up/down flag.",
  },
  {
    icon: AlertTriangleIcon,
    title: "AI incident summaries",
    body: "Every alert arrives with a plain-English explanation of what likely broke, and where to look first.",
    wide: true,
  },
];

function Features() {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.06);

  return (
    <section id="features" className="scroll-mt-24 py-28">
      <div className="mx-auto w-full max-w-[1200px] px-6">
        <SectionHead
          eyebrow="Everything, one place"
          title="Replace the five tools you're duct-taping together"
          blurb="Beacon is one control plane for uptime, latency, certificates and alerting — self-hosted, so the data never leaves your infrastructure."
        />

        <motion.ul
          initial="hidden"
          whileInView="show"
          viewport={IN_VIEW}
          variants={stagger}
          className="mt-16 grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
        >
          {FEATURES.map((f) => {
            const Icon = f.icon;
            return (
              <motion.li
                key={f.title}
                variants={reveal}
                className={f.wide ? "sm:col-span-2 lg:col-span-2" : ""}
              >
                <GlowCard className="h-full rounded-2xl border border-slate-900/10 bg-white/60 p-6 backdrop-blur-xl transition-transform duration-200 hover:-translate-y-1 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:border-white/10 dark:bg-white/[0.04]">
                  <div className="relative">
                    <span className="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-blue-600/10 text-blue-700 dark:bg-blue-400/10 dark:text-blue-400">
                      <Icon className="h-6 w-6" />
                    </span>
                    <h3 className="mt-4 text-lg font-semibold text-slate-900 dark:text-white">
                      {f.title}
                    </h3>
                    <p className="mt-2 leading-relaxed text-slate-600 dark:text-slate-300">
                      {f.body}
                    </p>
                  </div>
                </GlowCard>
              </motion.li>
            );
          })}
        </motion.ul>
      </div>
    </section>
  );
}

const STEPS = [
  {
    icon: FolderIcon,
    title: "Group by project",
    body: "Create a project per app or team. Every monitor lives in one, so dashboards and status pages stay organised as you grow.",
  },
  {
    icon: ActivityIcon,
    title: "Add your endpoints",
    body: "Point Beacon at a URL, host or cluster. It starts probing on your interval within seconds — no agent to install.",
  },
  {
    icon: ChartLineIcon,
    title: "Publish a status page",
    body: "Flip one switch to share a public page grouped by project. Customers see what's up; they never see your internals.",
  },
];

function HowItWorks() {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.08);

  return (
    <section id="how" className="scroll-mt-24 border-y border-slate-900/5 bg-slate-50/60 py-28 dark:border-white/5 dark:bg-white/[0.02]">
      <div className="mx-auto w-full max-w-[1200px] px-6">
        <SectionHead
          eyebrow="How it works"
          title="Live in about three minutes"
          blurb="No agents, no sidecars, no YAML archaeology."
        />

        <motion.ol
          initial="hidden"
          whileInView="show"
          viewport={IN_VIEW}
          variants={stagger}
          className="mt-16 grid gap-8 md:grid-cols-3"
        >
          {STEPS.map((s, i) => {
            const Icon = s.icon;
            return (
              <motion.li key={s.title} variants={reveal} className="relative">
                <div className="flex items-center gap-3">
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-900/10 bg-white text-base font-semibold tabular-nums text-slate-900 dark:border-white/15 dark:bg-slate-900 dark:text-white">
                    {i + 1}
                  </span>
                  <Icon className="h-5 w-5 text-blue-700 dark:text-blue-400" />
                </div>
                <h3 className="mt-4 text-lg font-semibold text-slate-900 dark:text-white">
                  {s.title}
                </h3>
                <p className="mt-2 leading-relaxed text-slate-600 dark:text-slate-300">{s.body}</p>
              </motion.li>
            );
          })}
        </motion.ol>
      </div>
    </section>
  );
}

/** A miniature of the real public status page, grouped by project. */
function StatusPreview() {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.05);

  const groups = [
    {
      name: "Production",
      up: 3,
      total: 3,
      hosts: [
        { n: "api.acme.com", up: true },
        { n: "app.acme.com", up: true },
        { n: "cdn.acme.com", up: true },
      ],
    },
    {
      name: "Staging",
      up: 1,
      total: 2,
      hosts: [
        { n: "stg-api.acme.com", up: true },
        { n: "stg-app.acme.com", up: false },
      ],
    },
  ];

  return (
    <section id="status" className="scroll-mt-24 py-28">
      <div className="mx-auto w-full max-w-[1200px] px-6">
        <SectionHead
          eyebrow="Status pages"
          title="A page your customers can trust"
          blurb="Group domains by project, publish on your own domain, and keep targets, IPs and internals private — only names and status are ever exposed."
        />

        <motion.div
          initial="hidden"
          whileInView="show"
          viewport={IN_VIEW}
          variants={stagger}
          className="mx-auto mt-14 max-w-3xl"
        >
          <motion.div
            variants={reveal}
            className="overflow-hidden rounded-2xl border border-slate-900/10 bg-white/70 shadow-xl shadow-slate-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-slate-900/50"
          >
            <div className="flex items-center gap-3 border-b border-slate-900/10 px-6 py-5 dark:border-white/10">
              <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg bg-emerald-600/10">
                <CheckCircleIcon className="h-5 w-5 text-emerald-700 dark:text-emerald-400" />
              </span>
              <div>
                <p className="font-semibold text-slate-900 dark:text-white">
                  All systems operational
                </p>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  Updated moments ago
                </p>
              </div>
            </div>

            <div className="divide-y divide-slate-900/5 dark:divide-white/5">
              {groups.map((g) => (
                <motion.div key={g.name} variants={reveal} className="px-6 py-5">
                  <div className="flex items-center justify-between">
                    <h3 className="font-medium text-slate-900 dark:text-white">{g.name}</h3>
                    <span className="font-mono text-sm text-slate-500 dark:text-slate-400">
                      {g.up}/{g.total} up
                    </span>
                  </div>
                  <ul className="mt-3 space-y-2">
                    {g.hosts.map((h) => (
                      <li
                        key={h.n}
                        className="flex items-center justify-between rounded-lg bg-slate-900/[0.03] px-3 py-2 dark:bg-white/[0.04]"
                      >
                        <span className="truncate font-mono text-sm text-slate-700 dark:text-slate-200">
                          {h.n}
                        </span>
                        <span
                          className={`ml-3 flex shrink-0 items-center gap-1.5 text-xs font-medium ${
                            h.up
                              ? "text-emerald-700 dark:text-emerald-400"
                              : "text-red-700 dark:text-red-400"
                          }`}
                        >
                          <span
                            aria-hidden
                            className={`h-1.5 w-1.5 rounded-full ${h.up ? "bg-emerald-600" : "bg-red-600"}`}
                          />
                          {h.up ? "Operational" : "Down"}
                        </span>
                      </li>
                    ))}
                  </ul>
                </motion.div>
              ))}
            </div>
          </motion.div>

          <motion.p
            variants={reveal}
            className="mt-6 flex items-center justify-center gap-2 text-sm text-slate-500 dark:text-slate-400"
          >
            <LockIcon className="h-4 w-4" />
            Targets, IPs and check config are never exposed publicly.
          </motion.p>
        </motion.div>
      </div>
    </section>
  );
}

function FinalCTA() {
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants();

  return (
    <section className="relative overflow-hidden py-28">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-1/2 -z-10 h-[420px] -translate-y-1/2 bg-gradient-to-r from-blue-500/10 via-emerald-500/10 to-blue-500/10 blur-3xl"
      />
      <motion.div
        initial="hidden"
        whileInView="show"
        viewport={IN_VIEW}
        variants={stagger}
        className="mx-auto max-w-2xl px-6 text-center"
      >
        <motion.h2
          variants={reveal}
          className="text-balance text-4xl font-semibold tracking-tight text-slate-900 dark:text-white"
        >
          Stop finding out from your customers.
        </motion.h2>
        <motion.p variants={reveal} className="mt-4 text-lg text-slate-600 dark:text-slate-300">
          Spin up Beacon, add your first domain, and get alerted before anyone opens a ticket.
        </motion.p>
        <motion.div variants={reveal} className="mt-9 flex flex-wrap justify-center gap-3">
          <Link
            href="/login?mode=register"
            className="group inline-flex items-center gap-2 rounded-xl bg-slate-900 px-7 py-4 text-base font-medium text-white shadow-lg shadow-slate-900/20 transition-transform hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:bg-white dark:text-slate-900 dark:focus-visible:ring-offset-slate-950"
          >
            Create your free account
            <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
          </Link>
        </motion.div>
        <motion.p
          variants={reveal}
          className="mt-5 flex items-center justify-center gap-2 text-sm text-slate-500 dark:text-slate-400"
        >
          <ClockIcon className="h-4 w-4" />
          Free to start — no credit card.
        </motion.p>
      </motion.div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="border-t border-slate-900/10 py-12 dark:border-white/10">
      <div className="mx-auto flex w-full max-w-[1200px] flex-col items-center gap-4 px-6 sm:flex-row sm:justify-between">
        <div className="flex items-center gap-2.5">
          <BeaconMark className="h-6 w-6 text-blue-600 dark:text-blue-400" />
          <span className="font-semibold text-slate-900 dark:text-white">Beacon</span>
          <span className="text-sm text-slate-500 dark:text-slate-400">
            — infrastructure monitoring
          </span>
        </div>
        <nav aria-label="Footer" className="flex items-center gap-5 text-sm">
          <a
            href="#features"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            Features
          </a>
          <a
            href="#status"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            Status pages
          </a>
          <Link
            href="/login"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            Sign in
          </Link>
        </nav>
      </div>
    </footer>
  );
}

export { Features, HowItWorks, StatusPreview, FinalCTA, Footer };
