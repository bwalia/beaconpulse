"use client";

import { ReactNode, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { Button } from "@/components/ui";

const baseNav = [
  { href: "/dashboard", label: "Dashboard", icon: "📊" },
  { href: "/monitors", label: "Monitors", icon: "🛰️" },
  { href: "/alerts", label: "Alerts", icon: "🚨" },
  { href: "/projects", label: "Projects", icon: "📁" },
  { href: "/notifications", label: "Notifications", icon: "🔔" },
  { href: "/billing", label: "Billing", icon: "💳" },
];
// The System page exposes the raw (global) Prometheus/Alertmanager tools and is
// therefore restricted to operators.
const adminNav = [{ href: "/system", label: "System", icon: "⚙️" }];

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { user, loading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!loading && !user) router.replace("/login");
  }, [loading, user, router]);

  if (loading || !user) {
    return (
      <div className="flex h-screen items-center justify-center text-slate-500">
        <span className="animate-pulse">Loading…</span>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-60 flex-shrink-0 border-r border-slate-200 bg-white p-4 dark:border-slate-800 dark:bg-slate-900 md:block">
        <div className="mb-6 flex items-center gap-2 px-2">
          <span className="text-xl">🛰️</span>
          <span className="text-lg font-bold">Beacon</span>
        </div>
        <nav className="space-y-1">
          {[...baseNav, ...(user.role === "owner" || user.role === "admin" ? adminNav : [])].map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition ${
                  active
                    ? "bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300"
                    : "text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
                }`}
              >
                <span>{item.icon}</span>
                {item.label}
              </Link>
            );
          })}
        </nav>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-slate-200 bg-white px-6 py-3 dark:border-slate-800 dark:bg-slate-900">
          <div className="text-sm text-slate-500">
            {user.name} · <span className="capitalize">{user.role}</span>
          </div>
          <Button variant="ghost" onClick={logout}>
            Sign out
          </Button>
        </header>
        <main className="flex-1 p-6">{children}</main>
      </div>
    </div>
  );
}
