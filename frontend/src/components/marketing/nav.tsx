"use client";

import { motion, useScroll, useTransform } from "framer-motion";
import Link from "next/link";

import { BeaconMark } from "@/components/icons";
import { ThemeToggle } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { DUR } from "@/lib/motion";

const LINKS = [
  { href: "#features", label: "Features" },
  { href: "#how", label: "How it works" },
  { href: "#status", label: "Status pages" },
];

/**
 * Marketing header. The bar is transparent over the hero and gains a frosted
 * background once you scroll — the glass appears only when there is content
 * behind it to blur, which is the point of the material.
 */
export function MarketingNav() {
  const { user, loading } = useAuth();
  const { scrollY } = useScroll();

  // Drive the chrome from scroll position rather than a state + listener, so no
  // re-render happens on scroll.
  const bg = useTransform(scrollY, [0, 80], ["rgba(255,255,255,0)", "rgba(255,255,255,0.72)"]);
  const bgDark = useTransform(scrollY, [0, 80], ["rgba(2,6,23,0)", "rgba(2,6,23,0.72)"]);
  const border = useTransform(scrollY, [0, 80], ["rgba(148,163,184,0)", "rgba(148,163,184,0.25)"]);
  const blur = useTransform(scrollY, [0, 80], ["blur(0px)", "blur(12px)"]);

  return (
    <motion.header
      style={{ borderColor: border, backdropFilter: blur }}
      className="fixed inset-x-0 top-0 z-50 border-b"
    >
      {/* Two stacked layers so light/dark each get their own tint without JS. */}
      <motion.div aria-hidden style={{ background: bg }} className="absolute inset-0 dark:hidden" />
      <motion.div aria-hidden style={{ background: bgDark }} className="absolute inset-0 hidden dark:block" />

      <nav
        aria-label="Primary"
        className="relative mx-auto w-full max-w-[1800px] px-6 sm:px-10 lg:px-16 flex items-center gap-8 py-5"
      >
        <Link
          href="/"
          className="flex items-center gap-2.5 rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
        >
          <BeaconMark className="h-8 w-8 text-blue-600 dark:text-blue-400" />
          <span className="text-xl font-semibold tracking-tight text-slate-900 dark:text-white">
            Beacon
          </span>
        </Link>

        <ul className="ml-4 hidden items-center gap-1 md:flex">
          {LINKS.map((l) => (
            <li key={l.href}>
              <a
                href={l.href}
                className="rounded-lg px-3.5 py-2 text-lg text-slate-600 transition-colors hover:bg-slate-900/5 hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:text-slate-300 dark:hover:bg-white/10 dark:hover:text-white"
              >
                {l.label}
              </a>
            </li>
          ))}
        </ul>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          {/* Until auth resolves, render nothing rather than flashing "Sign in"
              at a user who is already logged in. */}
          {!loading &&
            (user ? (
              <Link
                href="/dashboard"
                className="inline-flex items-center gap-2 rounded-lg bg-slate-900 px-5 py-2.5 text-lg font-medium text-white transition-transform hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:bg-white dark:text-slate-900 dark:focus-visible:ring-offset-slate-950"
              >
                Go to dashboard
              </Link>
            ) : (
              <>
                <Link
                  href="/login"
                  className="hidden rounded-lg px-4 py-2.5 text-lg font-medium text-slate-700 transition-colors hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none sm:inline-flex dark:text-slate-300 dark:hover:text-white"
                >
                  Sign in
                </Link>
                <Link
                  href="/register"
                  className="inline-flex items-center gap-2 rounded-lg bg-slate-900 px-5 py-2.5 text-lg font-medium text-white transition-transform hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 motion-reduce:transition-none motion-reduce:hover:translate-y-0 dark:bg-white dark:text-slate-900 dark:focus-visible:ring-offset-slate-950"
                  style={{ transitionDuration: `${DUR.micro}s` }}
                >
                  Start free
                </Link>
              </>
            ))}
        </div>
      </nav>
    </motion.header>
  );
}
