"use client";

import { ReactNode, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui";
import { BuildFooter } from "@/components/build-footer";
import { ConfirmProvider } from "@/components/confirm";
import { ThemeToggle } from "@/lib/theme";
import { LanguageSwitcher } from "@/components/language-switcher";
import { useTranslations } from "next-intl";
import { brand } from "@/brand";
import {
  ActivityIcon,
  AlertTriangleIcon,
  BeaconMark,
  BellIcon,
  CreditCardIcon,
  DashboardIcon,
  FolderIcon,
  GlobeIcon,
  LockIcon,
  SettingsIcon,
  SearchIcon,
  WrenchIcon,
} from "@/components/icons";

type NavItem = { href: string; key: string; Icon: (p: { className?: string }) => React.ReactElement };

const baseNav: NavItem[] = [
  { href: "/dashboard", key: "dashboard", Icon: DashboardIcon },
  { href: "/monitors", key: "monitors", Icon: ActivityIcon },
  { href: "/alerts", key: "alerts", Icon: AlertTriangleIcon },
  { href: "/explore", key: "explore", Icon: SearchIcon },
  { href: "/projects", key: "projects", Icon: FolderIcon },
  { href: "/status-page", key: "statusPage", Icon: GlobeIcon },
  { href: "/notifications", key: "notifications", Icon: BellIcon },
  { href: "/maintenance", key: "maintenance", Icon: WrenchIcon },
  { href: "/billing", key: "billing", Icon: CreditCardIcon },
];
// The System page exposes the raw (global) Prometheus/Alertmanager tools and is
// therefore restricted to operators.
const adminNav: NavItem[] = [
  { href: "/api-keys", key: "apiKeys", Icon: LockIcon },
  { href: "/system", key: "system", Icon: SettingsIcon },
];

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { user, loading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const t = useTranslations("sidebar");

  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);

  if (loading || !user) {
    return (
      <div className="flex h-screen items-center justify-center text-slate-500 dark:text-slate-400">
        <span className="motion-safe:animate-pulse">Loading…</span>
      </div>
    );
  }

  return (
    <ConfirmProvider>
    <div className="flex min-h-screen">
      <aside className="hidden w-64 flex-shrink-0 border-r border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900 md:block">
        <div className="mb-6 flex items-center gap-2.5 px-2">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-brand-600 text-white">
            <BeaconMark className="h-5 w-5" />
          </span>
          <span className="text-xl font-bold tracking-tight">{brand.name}</span>
        </div>
        <nav className="space-y-1">
          {[...baseNav, ...(user.role === "owner" || user.role === "admin" ? adminNav : [])].map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`relative flex items-center gap-3 rounded-lg px-3 py-2.5 text-base font-medium transition-colors motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 ${
                  active
                    ? "bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300"
                    : "text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
                }`}
              >
                {active && (
                  <span className="absolute inset-y-1.5 left-0 w-0.5 rounded-full bg-brand-600 dark:bg-brand-400" aria-hidden />
                )}
                <item.Icon className="h-5 w-5 shrink-0" />
                {t(item.key)}
              </Link>
            );
          })}
        </nav>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="sticky top-0 z-20 flex items-center justify-between gap-4 border-b border-slate-200 bg-white/90 px-6 py-3 backdrop-blur supports-[backdrop-filter]:bg-white/75 dark:border-slate-800 dark:bg-slate-900/90 dark:supports-[backdrop-filter]:bg-slate-900/75">
          <div className="min-w-0 truncate text-sm text-slate-600 dark:text-slate-300">
            <span className="font-medium text-slate-900 dark:text-slate-100">{user.name}</span> ·{" "}
            <span className="capitalize">{user.role}</span>
          </div>
          <div className="flex shrink-0 items-center gap-3">
            <LanguageSwitcher />
            <ThemeToggle />
            <Button variant="ghost" size="sm" onClick={logout}>
              {t("signOut")}
            </Button>
          </div>
        </header>
        <main className="flex-1 p-4 sm:p-6">
          {/* Single content container for every route, so page widths never drift
              and wide screens aren't wasted on empty gutters. */}
          <div className="mx-auto w-full max-w-[1600px]">
            {children}
            {/* In the layout, so it is on every dashboard page: "which environment am
                I on, and is this the build I just shipped?" is asked from wherever you
                happen to be standing, not from one page you have to navigate to. */}
            <BuildFooter />
          </div>
        </main>
      </div>
    </div>
    </ConfirmProvider>
  );
}
