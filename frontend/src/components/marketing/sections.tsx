"use client";

import { motion } from "framer-motion";
import { useTranslations } from "next-intl";
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
import { BRAND_NAME } from "@/lib/brand";
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
      className="mx-auto max-w-3xl text-center"
    >
      <motion.p
        variants={reveal}
        className="text-base font-semibold uppercase tracking-widest text-blue-700 dark:text-blue-400"
      >
        {eyebrow}
      </motion.p>
      <motion.h2
        variants={reveal}
        className="mt-4 text-balance text-5xl font-semibold tracking-tight text-slate-900 xl:text-6xl dark:text-white"
      >
        {title}
      </motion.h2>
      <motion.p
        variants={reveal}
        className="mt-5 text-xl leading-relaxed text-slate-600 dark:text-slate-300"
      >
        {blurb}
      </motion.p>
    </motion.div>
  );
}

// The visual metadata (icon, wide) lives in code; the words come from the catalog,
// keyed by name, so the whole grid speaks the reader's language.
const FEATURES = [
  { icon: ActivityIcon, key: "feature1", wide: true },
  { icon: BellIcon, key: "feature2", wide: false },
  { icon: LockIcon, key: "feature3", wide: false },
  { icon: GaugeIcon, key: "feature4", wide: false },
  { icon: AlertTriangleIcon, key: "feature5", wide: true },
] as const;

function Features() {
  const t = useTranslations("marketing");
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.06);

  return (
    <section id="features" className="scroll-mt-24 py-28">
      <div className="mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16">
        <SectionHead
          eyebrow={t("featuresEyebrow")}
          title={t("featuresTitle")}
          blurb={t("featuresBlurb", { brand: BRAND_NAME })}
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
                key={f.key}
                variants={reveal}
                className={f.wide ? "sm:col-span-2 lg:col-span-2" : ""}
              >
                <GlowCard className="h-full rounded-2xl border border-slate-900/10 bg-white/60 p-6 backdrop-blur-xl transition-transform duration-200 hover:-translate-y-1 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:border-white/10 dark:bg-white/[0.04]">
                  <div className="relative">
                    <span className="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-blue-600/10 text-blue-700 dark:bg-blue-400/10 dark:text-blue-400">
                      <Icon className="h-6 w-6" />
                    </span>
                    <h3 className="mt-5 text-xl font-semibold text-slate-900 dark:text-white">
                      {t(`${f.key}Title`)}
                    </h3>
                    <p className="mt-2.5 text-lg leading-relaxed text-slate-600 dark:text-slate-300">
                      {t(`${f.key}Body`)}
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
  { icon: FolderIcon, key: "step1" },
  { icon: ActivityIcon, key: "step2" },
  { icon: ChartLineIcon, key: "step3" },
] as const;

function HowItWorks() {
  const t = useTranslations("marketing");
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.08);

  return (
    <section id="how" className="scroll-mt-24 border-y border-slate-900/5 bg-slate-50/60 py-28 dark:border-white/5 dark:bg-white/[0.02]">
      <div className="mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16">
        <SectionHead
          eyebrow={t("howEyebrow")}
          title={t("howTitle")}
          blurb={t("howBlurb")}
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
              <motion.li key={s.key} variants={reveal} className="relative">
                <div className="flex items-center gap-3">
                  <span className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-slate-900/10 bg-white text-base font-semibold tabular-nums text-slate-900 dark:border-white/15 dark:bg-slate-900 dark:text-white">
                    {i + 1}
                  </span>
                  <Icon className="h-5 w-5 text-blue-700 dark:text-blue-400" />
                </div>
                <h3 className="mt-5 text-xl font-semibold text-slate-900 dark:text-white">
                  {t(`${s.key}Title`)}
                </h3>
                <p className="mt-2.5 text-lg leading-relaxed text-slate-600 dark:text-slate-300">
                  {s.key === "step2" ? t(`${s.key}Body`, { brand: BRAND_NAME }) : t(`${s.key}Body`)}
                </p>
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
  const t = useTranslations("marketing");
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
      <div className="mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16">
        <SectionHead
          eyebrow={t("statusEyebrow")}
          title={t("statusTitle")}
          blurb={t("statusBlurb")}
        />

        <motion.div
          initial="hidden"
          whileInView="show"
          viewport={IN_VIEW}
          variants={stagger}
          className="mx-auto mt-16 max-w-4xl"
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
                  {t("statusAllOperational")}
                </p>
                <p className="text-sm text-slate-500 dark:text-slate-400">
                  {t("statusUpdated")}
                </p>
              </div>
            </div>

            <div className="divide-y divide-slate-900/5 dark:divide-white/5">
              {groups.map((g) => (
                <motion.div key={g.name} variants={reveal} className="px-6 py-5">
                  <div className="flex items-center justify-between">
                    <h3 className="font-medium text-slate-900 dark:text-white">{g.name}</h3>
                    <span className="font-mono text-sm text-slate-500 dark:text-slate-400">
                      {t("statusUp", { up: g.up, total: g.total })}
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
                          {h.up ? t("statusOperational") : t("statusDown")}
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
            {t("statusPrivacyNote")}
          </motion.p>
        </motion.div>
      </div>
    </section>
  );
}

function FinalCTA() {
  const t = useTranslations("marketing");
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
        className="mx-auto max-w-3xl px-6 text-center"
      >
        <motion.h2
          variants={reveal}
          className="text-balance text-5xl font-semibold tracking-tight text-slate-900 xl:text-6xl dark:text-white"
        >
          {t("ctaTitle")}
        </motion.h2>
        <motion.p variants={reveal} className="mt-5 text-xl text-slate-600 dark:text-slate-300">
          {t("ctaBlurb", { brand: BRAND_NAME })}
        </motion.p>
        <motion.div variants={reveal} className="mt-9 flex flex-wrap justify-center gap-3">
          <Link
            href="/register"
            className="group inline-flex items-center gap-2 rounded-xl bg-slate-900 px-8 py-4 text-lg font-medium text-white shadow-lg shadow-slate-900/20 transition-transform hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:bg-white dark:text-slate-900 dark:focus-visible:ring-offset-slate-950"
          >
            {t("ctaButton")}
            <ArrowRightIcon className="h-4 w-4 transition-transform group-hover:translate-x-0.5 motion-reduce:transition-none" />
          </Link>
        </motion.div>
        <motion.p
          variants={reveal}
          className="mt-5 flex items-center justify-center gap-2 text-sm text-slate-500 dark:text-slate-400"
        >
          <ClockIcon className="h-4 w-4" />
          {t("ctaFinePrint")}
        </motion.p>
      </motion.div>
    </section>
  );
}

function Footer() {
  const t = useTranslations("nav");
  const tm = useTranslations("marketing");
  return (
    <footer className="border-t border-slate-900/10 py-12 dark:border-white/10">
      <div className="mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16 flex flex-col items-center gap-4 sm:flex-row sm:justify-between">
        <div className="flex items-center gap-2.5">
          <BeaconMark className="h-6 w-6 text-blue-600 dark:text-blue-400" />
          <span className="font-semibold text-slate-900 dark:text-white">{BRAND_NAME}</span>
          <span className="text-sm text-slate-500 dark:text-slate-400">
            — {tm("footerTagline")}
          </span>
        </div>
        <nav aria-label="Footer" className="flex items-center gap-5 text-sm">
          <a
            href="#features"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            {t("features")}
          </a>
          <a
            href="#status"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            {t("statusPages")}
          </a>
          <Link
            href="/docs"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            {t("docs")}
          </Link>
          <Link
            href="/login"
            className="rounded text-slate-600 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:text-white"
          >
            {t("signIn")}
          </Link>
        </nav>
      </div>
    </footer>
  );
}

export { Features, HowItWorks, StatusPreview, FinalCTA, Footer };
