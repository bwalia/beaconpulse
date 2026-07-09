"use client";

import { useAuth } from "@/lib/auth";
import { Card, PageHeader } from "@/components/ui";
import { ChartLineIcon, LockIcon, MegaphoneIcon } from "@/components/icons";

// Prometheus and Alertmanager are served through prom-label-proxy at the gateway,
// which enforces {org_id="<you>"} on every query — so these UIs show ONLY your
// organization's data. (Blackbox has no query API and cannot be tenant-filtered,
// so it is not exposed here — it's an operator-only tool on the internal network.)
const tools = [
  { href: "/prometheus/", label: "Prometheus", Icon: ChartLineIcon, desc: "PromQL, graphs & alerts — scoped to your org", scoped: true },
  { href: "/alertmanager/", label: "Alertmanager", Icon: MegaphoneIcon, desc: "Your org's alerts & silences", scoped: true },
];

export default function SystemPage() {
  const { user } = useAuth();
  const isOperator = user?.role === "owner" || user?.role === "admin";

  if (!isOperator) {
    return (
      <div className="mx-auto max-w-3xl">
        <Card>
          <p className="text-slate-500 dark:text-slate-400">This page is restricted to organization owners and admins.</p>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader title="System" subtitle="Operator access to the underlying monitoring engines." />

      <div className="flex items-start gap-2 rounded-lg border border-emerald-300 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300">
        <LockIcon className="mt-0.5 h-4 w-4 shrink-0" />
        <p>
          <span className="font-medium">Prometheus &amp; Alertmanager are filtered to your organization</span> — the
          gateway enforces <span className="font-mono">org_id</span> on every query, so you only see your own data.
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {tools.map((t) => (
          <a
            key={t.href}
            href={t.href}
            target="_blank"
            rel="noreferrer"
            className="flex items-start gap-3 rounded-lg border border-slate-200 p-3 transition hover:border-brand-400 hover:bg-brand-50/40 dark:border-slate-800 dark:hover:border-brand-700 dark:hover:bg-brand-900/20"
          >
            <t.Icon className="mt-0.5 h-5 w-5 shrink-0 text-slate-500 dark:text-slate-400" />
            <span>
              <span className="flex items-center gap-1.5 text-sm font-medium">
                {t.label}
                {t.scoped ? (
                  <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300">
                    your org
                  </span>
                ) : (
                  <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-300">
                    global
                  </span>
                )}
              </span>
              <span className="block text-xs text-slate-500 dark:text-slate-400">{t.desc}</span>
            </span>
          </a>
        ))}
      </div>
    </div>
  );
}
