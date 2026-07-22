"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { BeaconMark } from "@/components/icons";
import { ThemeToggle } from "@/lib/theme";
import { LanguageSwitcher } from "@/components/language-switcher";
import { brand } from "@/brand";

/**
 * The documentation frame: a persistent table of contents, and the page beside it.
 *
 * Docs are read by someone stuck partway through a task, so the whole structure is
 * visible at all times rather than behind a menu — being able to see that the answer
 * exists two sections down is most of what a sidebar is for.
 */

interface Item {
  href: string;
  label: string;
}
interface Group {
  title: string;
  items: Item[];
}

const NAV: Group[] = [
  {
    title: "Getting started",
    items: [
      { href: "/docs", label: "Introduction" },
      { href: "/docs/quickstart", label: "Quickstart" },
      { href: "/docs/monitors", label: "Monitor types" },
    ],
  },
  {
    title: "API",
    items: [
      { href: "/docs/authentication", label: "Authentication" },
      { href: "/docs/api", label: "API reference" },
      { href: "/docs/console", label: "API console" },
      { href: "/docs/automation", label: "CI & automation" },
    ],
  },
  {
    title: "Platform",
    items: [
      { href: "/docs/alerts", label: "Alerts & maintenance" },
      { href: "/docs/status-pages", label: "Status pages" },
      { href: "/docs/plans", label: "Plans & billing" },
    ],
  },
];

function NavList({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();
  return (
    <nav aria-label="Documentation">
      {NAV.map((group) => (
        <div key={group.title} className="mb-7">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
            {group.title}
          </p>
          <ul className="space-y-0.5">
            {group.items.map((item) => {
              // Exact match, not prefix: /docs is a prefix of every page here, so a
              // prefix test would light up "Introduction" on all of them.
              const active = pathname === item.href;
              return (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    onClick={onNavigate}
                    aria-current={active ? "page" : undefined}
                    className={`block rounded-lg px-3 py-1.5 text-sm transition-colors motion-reduce:transition-none ${
                      active
                        ? "bg-brand-50 font-medium text-brand-700 dark:bg-brand-950/50 dark:text-brand-300"
                        : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800/60 dark:hover:text-slate-100"
                    }`}
                  >
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
    </nav>
  );
}

export function DocsShell({ children }: { children: React.ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="min-h-dvh bg-white text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <a
        href="#doc"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[60] focus:rounded-lg focus:bg-slate-900 focus:px-4 focus:py-2 focus:text-white dark:focus:bg-white dark:focus:text-slate-900"
      >
        Skip to content
      </a>

      <header className="sticky top-0 z-40 border-b border-slate-200 bg-white/85 backdrop-blur dark:border-slate-800 dark:bg-slate-950/85">
        <div className="mx-auto flex w-full max-w-[1400px] items-center gap-4 px-4 py-3.5 sm:px-6 lg:px-8">
          <Link
            href="/"
            className="flex items-center gap-2.5 rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600"
          >
            <BeaconMark className="h-7 w-7 text-brand-600 dark:text-brand-400" />
            <span className="whitespace-nowrap font-semibold tracking-tight">{brand.name}</span>
          </Link>
          <span className="hidden text-sm text-slate-400 dark:text-slate-600 sm:inline">/</span>
          <span className="hidden text-sm text-slate-600 dark:text-slate-300 sm:inline">Docs</span>

          <div className="ml-auto flex items-center gap-2">
            <Link
              href="/dashboard"
              className="hidden rounded-lg px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-100 hover:text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white sm:block"
            >
              Dashboard
            </Link>
            <LanguageSwitcher className="hidden sm:inline-flex" />
            <ThemeToggle />
            <button
              type="button"
              onClick={() => setMenuOpen((v) => !v)}
              aria-expanded={menuOpen}
              aria-label="Toggle documentation menu"
              className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm text-slate-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 dark:border-slate-700 dark:text-slate-200 lg:hidden"
            >
              {menuOpen ? "Close" : "Contents"}
            </button>
          </div>
        </div>

        {menuOpen && (
          <div className="border-t border-slate-200 px-4 py-4 dark:border-slate-800 lg:hidden">
            <NavList onNavigate={() => setMenuOpen(false)} />
          </div>
        )}
      </header>

      <div className="mx-auto flex w-full max-w-[1400px] gap-10 px-4 sm:px-6 lg:px-8">
        <aside className="sticky top-[4.25rem] hidden h-[calc(100dvh-4.25rem)] w-56 shrink-0 overflow-y-auto py-10 lg:block">
          <NavList />
        </aside>

        {/* Prose is capped near 70ch: past that the eye loses the start of the next
            line, which is exactly the wrong failure mode in a reference someone is
            scanning rather than reading. */}
        <main id="doc" className="min-w-0 flex-1 py-10 lg:max-w-[46rem]">
          {children}
        </main>
      </div>

      <footer className="mt-12 border-t border-slate-200 dark:border-slate-800">
        <div className="mx-auto flex w-full max-w-[1400px] flex-wrap items-center gap-x-6 gap-y-2 px-4 py-6 text-sm text-slate-500 sm:px-6 lg:px-8 dark:text-slate-400">
          <Link href="/" className="hover:text-slate-900 dark:hover:text-white">
            Home
          </Link>
          <Link href="/docs" className="hover:text-slate-900 dark:hover:text-white">
            Docs
          </Link>
          <Link href="/register" className="hover:text-slate-900 dark:hover:text-white">
            Create an account
          </Link>
        </div>
      </footer>
    </div>
  );
}
