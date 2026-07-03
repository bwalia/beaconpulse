"use client";

import { useActiveAlerts } from "@/lib/hooks";
import { Card } from "@/components/ui";

function sinceLabel(since?: string): string {
  if (!since) return "";
  const ms = Date.now() - new Date(since).getTime();
  if (ms < 0) return "just now";
  const mins = Math.floor(ms / 60000);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  const rem = mins % 60;
  return rem ? `${hrs}h${rem}m` : `${hrs}h`;
}

export default function AlertsPage() {
  const { data, isLoading } = useActiveAlerts();
  const alerts = data?.data ?? [];

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Alerts</h1>
        <p className="text-sm text-slate-500">
          Currently-firing alerts for <span className="font-medium">your organization</span> only.
        </p>
      </div>

      {isLoading ? (
        <p className="text-slate-500">Loading…</p>
      ) : alerts.length === 0 ? (
        <Card>
          <p className="text-center text-emerald-600">✅ All clear — nothing is firing right now.</p>
        </Card>
      ) : (
        <div className="space-y-3">
          {alerts.map((a, i) => (
            <Card
              key={i}
              className={`border-l-4 ${
                a.severity === "critical" ? "border-l-red-500" : "border-l-amber-500"
              }`}
            >
              <div className="flex items-start justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-semibold">{a.name}</span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs font-medium uppercase ${
                        a.severity === "critical"
                          ? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300"
                          : "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
                      }`}
                    >
                      {a.severity}
                    </span>
                  </div>
                  <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
                    {a.monitor_name} <span className="text-slate-400">({a.monitor_type})</span>
                  </p>
                  <p className="font-mono text-xs text-slate-400">{a.target}</p>
                </div>
                {a.since && (
                  <span className="whitespace-nowrap text-xs text-slate-400">firing {sinceLabel(a.since)}</span>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
