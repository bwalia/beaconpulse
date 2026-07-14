"use client";

import { useActiveAlerts } from "@/lib/hooks";
import { Card, EmptyState, PageHeader, Skeleton } from "@/components/ui";
import { AlertTriangleIcon, CheckCircleIcon, ClockIcon, WrenchIcon } from "@/components/icons";

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
  const critical = alerts.filter((a) => a.severity === "critical").length;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Alerts"
        subtitle="Currently-firing alerts for your organization only."
        actions={
          alerts.length > 0 ? (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-red-50 px-3 py-1 text-xs font-semibold text-red-800 dark:bg-red-950/60 dark:text-red-300">
              <AlertTriangleIcon className="h-3.5 w-3.5" />
              {alerts.length} firing{critical ? ` · ${critical} critical` : ""}
            </span>
          ) : null
        }
      />

      {isLoading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 w-full rounded-xl" />
          ))}
        </div>
      ) : alerts.length === 0 ? (
        <EmptyState
          icon={<CheckCircleIcon className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />}
          title="All clear"
        >
          Nothing is firing right now. Alerts appear here the moment a monitor breaches its rule.
        </EmptyState>
      ) : (
        <ul className="space-y-3">
          {alerts.map((a, i) => {
            const isCritical = a.severity === "critical";
            return (
              <li key={i}>
                <Card
                  className={`border-l-4 transition-shadow hover:shadow-md motion-reduce:transition-none ${
                    isCritical ? "border-l-red-600" : "border-l-amber-500"
                  }`}
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold">{a.name}</span>
                        <span
                          className={`rounded-full px-2 py-0.5 text-xs font-semibold uppercase tracking-wide ${
                            isCritical
                              ? "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-200"
                              : "bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200"
                          }`}
                        >
                          {a.severity}
                        </span>
                        {a.in_maintenance && (
                          <span
                            title="This monitor is under an active maintenance window — its notification was suppressed"
                            className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/40 dark:text-blue-200"
                          >
                            <WrenchIcon className="h-3 w-3" />
                            Suppressed
                          </span>
                        )}
                      </div>
                      <p className="mt-1 truncate text-sm text-slate-600 dark:text-slate-300">
                        {a.monitor_name}{" "}
                        <span className="text-slate-500 dark:text-slate-400">({a.monitor_type})</span>
                      </p>
                      <p className="truncate font-mono text-xs text-slate-500 dark:text-slate-400">{a.target}</p>
                    </div>
                    {a.since && (
                      <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap text-xs tabular-nums text-slate-500 dark:text-slate-400">
                        <ClockIcon className="h-3.5 w-3.5" />
                        firing {sinceLabel(a.since)}
                      </span>
                    )}
                  </div>
                </Card>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
