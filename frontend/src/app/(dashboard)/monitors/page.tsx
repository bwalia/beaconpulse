"use client";

import { useEffect, useMemo, useState } from "react";
import { brand } from "@/brand";
import { motion } from "framer-motion";
import { useTranslations } from "next-intl";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  useCreateMonitor,
  useDeleteMonitor,
  useMonitorMetrics,
  useMonitorsPage,
  useProjects,
  useSetMonitorEnabled,
  useUpdateMonitor,
  useUsage,
} from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import {
  Button,
  Card,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Select,
  Skeleton,
  StatusBadge,
  Textarea,
} from "@/components/ui";
import { ActivityIcon, CheckCircleIcon, PlusIcon, SearchIcon, WrenchIcon, XIcon } from "@/components/icons";
import { useConfirm } from "@/components/confirm";
import { isFailing, useDiagnoseControl } from "@/components/diagnose-panel";
import { Pagination, SearchInput } from "@/components/table-controls";
import { useRevealVariants, useStaggerVariants } from "@/lib/motion";
import type { MetricPoint, Monitor } from "@/lib/types";

const PAGE_SIZE = 20;

// Built via a factory so validation messages come from next-intl (a module-scope
// schema would freeze the messages in English before the translator exists).
function makeSchema(t: (key: string) => string) {
  return z.object({
    project_id: z.string().uuid(t("selectProject")),
    name: z.string().min(1, t("nameRequired")),
    type: z.enum(["http", "https", "ssl", "tcp", "icmp", "dns", "heartbeat"]),
    // Optional at the schema level; required-unless-heartbeat is enforced by the
    // refine() at the bottom of the object, because a heartbeat has no target.
    target: z.string().optional(),
    interval_seconds: z.coerce.number().int().min(10).max(86400),
    grace_seconds: z.coerce.number().int().min(0).max(86400).optional(),
    // Advanced (all optional)
    valid_status_codes: z.string().optional(),
    body_keyword: z.string().optional(),
    follow_redirects: z.boolean().optional(),
    skip_tls_verify: z.boolean().optional(),
    response_time_warning_ms: z.coerce.number().int().min(0).optional(),
    ssl_expiry_warning_days: z.coerce.number().int().min(0).max(825).optional(),
    headers: z.string().optional(),
    dns_query_type: z.enum(["A", "AAAA", "CNAME", "MX", "TXT", "NS", "SOA", "CAA"]).optional(),
    alert_sensitivity: z.enum(["immediate", "balanced", "relaxed"]).optional(),
  }).refine((v) => v.type === "heartbeat" || (v.target ?? "").trim().length > 0, {
    message: t("targetRequired"),
    path: ["target"],
  });
}
type Values = z.infer<ReturnType<typeof makeSchema>>;

// AdvancedFields is the shared shape of the advanced settings inputs, used by
// both the create and edit forms.
type AdvancedFields = {
  valid_status_codes?: string;
  body_keyword?: string;
  follow_redirects?: boolean;
  skip_tls_verify?: boolean;
  response_time_warning_ms?: number;
  ssl_expiry_warning_days?: number;
  headers?: string;
  dns_query_type?: string;
  alert_sensitivity?: string;
};

// SENSITIVITY_OPTIONS controls how long a monitor must be down before it alerts.
// Labels are resolved via t() at the render site (see the `key`).
const SENSITIVITY_OPTIONS = [
  { v: "immediate", key: "sensitivityImmediate" },
  { v: "balanced", key: "sensitivityBalanced" },
  { v: "relaxed", key: "sensitivityRelaxed" },
];

// buildSettings turns the advanced form fields into the API's settings object,
// omitting anything the user left blank.
function buildSettings(v: AdvancedFields): Record<string, unknown> {
  const s: Record<string, unknown> = {};
  if (v.body_keyword) s.body_keyword = v.body_keyword;
  if (v.follow_redirects) s.follow_redirects = true;
  if (v.skip_tls_verify) s.skip_tls_verify = true;
  if (v.response_time_warning_ms) s.response_time_warning_ms = v.response_time_warning_ms;
  if (v.ssl_expiry_warning_days) s.ssl_expiry_warning_days = v.ssl_expiry_warning_days;
  if (v.dns_query_type) s.dns_query_type = v.dns_query_type;
  if (v.alert_sensitivity) s.alert_sensitivity = v.alert_sensitivity;
  if (v.valid_status_codes) {
    const codes = v.valid_status_codes
      .split(",")
      .map((c) => parseInt(c.trim(), 10))
      .filter((n) => !Number.isNaN(n));
    if (codes.length) s.valid_status_codes = codes;
  }
  if (v.headers) {
    const h: Record<string, string> = {};
    for (const line of v.headers.split("\n")) {
      const idx = line.indexOf(":");
      if (idx > 0) {
        const key = line.slice(0, idx).trim();
        const val = line.slice(idx + 1).trim();
        if (key) h[key] = val;
      }
    }
    if (Object.keys(h).length) s.headers = h;
  }
  return s;
}

const targetHints: Record<string, string> = {
  http: "http://example.com/health",
  https: "https://example.com",
  ssl: "https://example.com",
  tcp: "db.example.com:5432",
  icmp: "example.com",
  dns: "example.com",
};

// HeartbeatCreated shows the ping URL right after a heartbeat is created. The URL
// carries the token (the credential), so we surface it prominently once and give a
// ready-to-paste cron example, rather than burying it in the monitor row.
function HeartbeatCreated({ monitor, onDone }: { monitor: Monitor; onDone: () => void }) {
  const t = useTranslations("pages.monitors");
  const c = useTranslations("common");
  const [copied, setCopied] = useState(false);
  // ping_url is returned relative; compose the absolute URL from the current origin
  // so it never hard-codes a gateway host.
  const url =
    typeof window !== "undefined" && monitor.ping_url
      ? new URL(monitor.ping_url, window.location.origin).toString()
      : (monitor.ping_url ?? "");

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard blocked — the URL is selectable in the field regardless */
    }
  }

  return (
    <Card>
      <div className="flex items-start gap-3">
        <CheckCircleIcon className="mt-0.5 h-7 w-7 shrink-0 text-emerald-600 dark:text-emerald-400" />
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold text-slate-900 dark:text-white">
            {t("heartbeatReadyTitle", { name: monitor.name })}
          </h3>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
            {t("heartbeatReadyBody", { brand: brand.name })}
          </p>

          <div className="mt-3 flex items-center gap-2">
            <input
              readOnly
              value={url}
              onFocus={(e) => e.currentTarget.select()}
              className="w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
            />
            <Button variant="secondary" onClick={copy}>
              {copied ? c("copied") : c("copy")}
            </Button>
          </div>

          <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">{t("heartbeatExampleCrontab")}</p>
          <pre className="mt-1 overflow-x-auto rounded-lg bg-slate-900 p-3 text-xs text-slate-100">
            {`0 2 * * *  /path/to/backup.sh && curl -fsS ${url}`}
          </pre>

          <div className="mt-4">
            <Button onClick={onDone}>{c("done")}</Button>
          </div>
        </div>
      </div>
    </Card>
  );
}

export default function MonitorsPage() {
  const t = useTranslations("pages.monitors");
  const c = useTranslations("common");
  const [page, setPage] = useState(0);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");

  // Debounce the search box so we don't refetch on every keystroke. The page reset
  // rides along with the debounced value: a new query has a different first page, so
  // the two belong to one change rather than an effect reconciling them afterwards.
  useEffect(() => {
    const t = setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(0);
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  // Same for the status filter — otherwise you could sit on an out-of-range page
  // showing nothing.
  const changeStatus = (next: string) => {
    setStatus(next);
    setPage(0);
  };

  const { data, isLoading, isPlaceholderData } = useMonitorsPage({
    page,
    pageSize: PAGE_SIZE,
    search: search || undefined,
    status: status || undefined,
  });
  const { data: usage } = useUsage();
  const [showForm, setShowForm] = useState(false);
  const [metricsFor, setMetricsFor] = useState<Monitor | null>(null);
  const [editing, setEditing] = useState<Monitor | null>(null);

  const rows = data?.data ?? [];
  const total = data?.pagination.total ?? 0;
  const filtering = search !== "" || status !== "";
  const stagger = useStaggerVariants(0.03);

  const atLimit = usage ? usage.monitors_used >= usage.monitors_limit : false;
  const pct = usage ? Math.min(100, Math.round((usage.monitors_used / usage.monitors_limit) * 100)) : 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("title")}
        subtitle={t("subtitle")}
        actions={
          <>
            {usage && (
              <div className="mr-2 text-right">
                <div className="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                  <span className="rounded bg-slate-100 px-1.5 py-0.5 font-medium uppercase text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                    {usage.plan}
                  </span>
                  <span className="tabular-nums">
                    {t("usageCount", { used: usage.monitors_used, limit: usage.monitors_limit })}
                  </span>
                </div>
                <div
                  className="mt-1 h-1.5 w-32 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800"
                  role="progressbar"
                  aria-label={t("quotaAriaLabel")}
                  aria-valuenow={usage.monitors_used}
                  aria-valuemin={0}
                  aria-valuemax={usage.monitors_limit}
                >
                  <div
                    className={`h-full rounded-full ${atLimit ? "bg-red-600" : "bg-brand-600"}`}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            )}
            <Button onClick={() => setShowForm((v) => !v)}>
              {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
              {showForm ? c("close") : t("addMonitor")}
            </Button>
          </>
        }
      />

      {atLimit && !showForm && (
        <div className="rounded-lg border border-amber-300 bg-amber-50 px-4 py-2 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
          {t.rich("atLimitBanner", {
            plan: usage?.plan ?? "",
            limit: usage?.monitors_limit ?? 0,
            b: (chunks) => <span className="font-medium uppercase">{chunks}</span>,
          })}
        </div>
      )}

      {showForm && <CreateMonitorForm onDone={() => setShowForm(false)} />}

      {/* Toolbar: server-side search + status filter. */}
      {(total > 0 || filtering) && !isLoading && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <SearchInput
            value={searchInput}
            onChange={setSearchInput}
            placeholder={t("searchPlaceholder")}
            label={t("searchLabel")}
          />
          <select
            value={status}
            onChange={(e) => changeStatus(e.target.value)}
            aria-label={t("filterStatusLabel")}
            className="rounded-lg border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 sm:w-44"
          >
            <option value="">{t("statusAll")}</option>
            <option value="up">{t("statusUp")}</option>
            <option value="down">{t("statusDown")}</option>
            <option value="degraded">{t("statusDegraded")}</option>
            <option value="paused">{t("statusPaused")}</option>
            <option value="unknown">{t("statusUnknown")}</option>
          </select>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-11 w-full rounded-t-xl" />
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : total === 0 ? (
        <EmptyState
          icon={filtering ? <SearchIcon className="h-5 w-5" /> : <ActivityIcon className="h-5 w-5" />}
          title={filtering ? t("emptyFiltered") : t("empty")}
          action={
            filtering ? (
              <Button
                variant="secondary"
                onClick={() => {
                  setSearchInput("");
                  changeStatus("");
                }}
              >
                {t("clearFilters")}
              </Button>
            ) : (
              <Button onClick={() => setShowForm(true)}>
                <PlusIcon className="h-4 w-4" />
                {t("addMonitor")}
              </Button>
            )
          }
        >
          {filtering
            ? t("emptyFilteredBody")
            : t("emptyBody", { brand: brand.name })}
        </EmptyState>
      ) : (
        <>
          <Card className="overflow-x-auto p-0">
            <table className="w-full text-[15px]">
              <thead className="border-b border-slate-200 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:bg-slate-800/40 dark:text-slate-400">
                <tr>
                  <th scope="col" className="px-4 py-3 font-semibold">{t("colStatus")}</th>
                  <th scope="col" className="px-4 py-3 font-semibold">{t("colName")}</th>
                  <th scope="col" className="px-4 py-3 font-semibold">{t("colType")}</th>
                  <th scope="col" className="px-4 py-3 font-semibold">{t("colTarget")}</th>
                  <th scope="col" className="px-4 py-3 font-semibold">{t("colInterval")}</th>
                  <th scope="col" className="px-4 py-3 text-right font-semibold">{t("colActions")}</th>
                </tr>
              </thead>
              <motion.tbody
                key={page}
                initial="hidden"
                animate="show"
                variants={stagger}
                className={isPlaceholderData ? "opacity-60 transition-opacity duration-200" : "transition-opacity duration-200"}
              >
                {rows.map((m) => (
                  <MonitorRow
                    key={m.id}
                    monitor={m}
                    onMetrics={() => setMetricsFor(m)}
                    onEdit={() => setEditing(m)}
                  />
                ))}
              </motion.tbody>
            </table>
          </Card>

          <Pagination
            page={page}
            pageSize={PAGE_SIZE}
            total={total}
            unit="monitors"
            busy={isPlaceholderData}
            onPageChange={setPage}
          />
        </>
      )}

      {metricsFor && <MonitorMetricsModal monitor={metricsFor} onClose={() => setMetricsFor(null)} />}
      {editing && <EditMonitorModal monitor={editing} onClose={() => setEditing(null)} />}
    </div>
  );
}

function MonitorRow({
  monitor,
  onMetrics,
  onEdit,
}: {
  monitor: Monitor;
  onMetrics: () => void;
  onEdit: () => void;
}) {
  const t = useTranslations("pages.monitors");
  const c = useTranslations("common");
  const setEnabled = useSetMonitorEnabled();
  const deleteMonitor = useDeleteMonitor();
  const confirm = useConfirm();
  const reveal = useRevealVariants();
  const diagnose = useDiagnoseControl(monitor.id);

  const target =
    monitor.type === "heartbeat" && monitor.ping_url ? monitor.ping_url : monitor.target;

  return (
    <>
    <motion.tr
      variants={reveal}
      className="border-b border-slate-100 transition-colors last:border-0 hover:bg-slate-50 motion-reduce:transition-none dark:border-slate-800/60 dark:hover:bg-slate-800/40"
    >
      <td className="px-4 py-3.5 align-top">
        <div className="flex flex-col items-start gap-1">
          <StatusBadge status={monitor.enabled ? monitor.last_status : "paused"} />
          {monitor.in_maintenance && (
            <span
              title={t("maintenanceTooltip")}
              className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/40 dark:text-blue-200"
            >
              <WrenchIcon className="h-3 w-3" />
              {t("maintenanceBadge")}
            </span>
          )}
        </div>
      </td>
      <td className="px-4 py-3.5">
        <button
          onClick={onMetrics}
          className="rounded text-left font-medium text-slate-900 hover:text-brand-700 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:text-white dark:hover:text-brand-400"
        >
          {monitor.name}
        </button>
      </td>
      <td className="px-4 py-3.5">
        <span className="inline-block rounded border border-slate-200 px-2 py-0.5 text-xs font-medium uppercase tracking-wide text-slate-600 dark:border-slate-700 dark:text-slate-300">
          {monitor.type}
        </span>
      </td>
      <td className="px-4 py-3.5">
        {/* A heartbeat has no probe target; show its ping URL instead, so the
            owner can retrieve it any time. Truncate long values with the full
            string on hover. */}
        <span
          title={target}
          className="block max-w-[22rem] truncate font-mono text-sm text-slate-700 dark:text-slate-300 lg:max-w-[32rem]"
        >
          {target}
        </span>
      </td>
      <td className="px-4 py-3.5 tabular-nums text-slate-700 dark:text-slate-300">{monitor.interval_seconds}s</td>
      <td className="px-4 py-3">
        {/* Safe actions recede; the destructive one keeps its danger colour but is
            separated and de-emphasised, so `Delete` isn't the loudest thing on the
            page six times over. */}
        <div className="flex items-center justify-end gap-1">
          {isFailing(monitor) && (
            <Button
              size="sm"
              variant="ghost"
              onClick={diagnose.run}
              disabled={diagnose.isPending}
              className="text-brand-700 hover:bg-brand-50 dark:text-brand-400 dark:hover:bg-brand-950/40"
            >
              {diagnose.label}
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={onMetrics}>
            {t("metricsAction")}
          </Button>
          <Button size="sm" variant="ghost" onClick={onEdit}>
            {c("edit")}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setEnabled.mutate({ id: monitor.id, enabled: !monitor.enabled })}
            disabled={setEnabled.isPending}
          >
            {monitor.enabled ? t("pause") : t("resume")}
          </Button>
          <span className="mx-1 h-5 w-px bg-slate-200 dark:bg-slate-700" aria-hidden />
          <Button
            size="sm"
            variant="ghost"
            className="text-red-700 hover:bg-red-50 hover:text-red-800 focus-visible:ring-red-500 dark:text-red-400 dark:hover:bg-red-950/50 dark:hover:text-red-300"
            onClick={async () => {
              if (
                await confirm({
                  title: t("deleteConfirmTitle", { name: monitor.name }),
                  body: t("deleteConfirmBody"),
                  confirmLabel: t("deleteConfirmLabel"),
                  danger: true,
                })
              ) {
                deleteMonitor.mutate(monitor.id);
              }
            }}
            disabled={deleteMonitor.isPending}
          >
            {c("delete")}
          </Button>
        </div>
      </td>
    </motion.tr>
      {diagnose.panel && (
        <tr>
          {/* Spans the table so the diagnosis reads as part of this monitor's row
              rather than as a new column of anything. */}
          <td colSpan={7} className="px-4 pb-4 pt-0">
            {diagnose.panel}
          </td>
        </tr>
      )}
    </>
  );
}

function Sparkline({ points, color = "#328cff" }: { points: MetricPoint[]; color?: string }) {
  const t = useTranslations("pages.monitors");
  if (points.length < 2) {
    return <p className="py-6 text-center text-xs text-slate-500 dark:text-slate-400">{t("notEnoughData")}</p>;
  }
  const w = 320;
  const h = 60;
  const vals = points.map((p) => p.v);
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const span = max - min || 1;
  const d = points
    .map((p, i) => {
      const x = (i / (points.length - 1)) * w;
      const y = h - ((p.v - min) / span) * h;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full" preserveAspectRatio="none" height={h}>
      <path d={d} fill="none" stroke={color} strokeWidth="2" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function MonitorMetricsModal({ monitor, onClose }: { monitor: Monitor; onClose: () => void }) {
  const t = useTranslations("pages.monitors");
  const c = useTranslations("common");
  const { data, isLoading } = useMonitorMetrics(monitor.id);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-xl border border-slate-200 bg-white p-5 shadow-xl dark:border-slate-800 dark:bg-slate-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="text-lg font-bold">{monitor.name}</h2>
            <p className="font-mono text-xs text-slate-500 dark:text-slate-400">{monitor.target}</p>
          </div>
          <button
            onClick={onClose}
            aria-label={c("close")}
            className="rounded p-1.5 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 motion-reduce:transition-none dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
          >
            <XIcon className="h-4 w-4" />
          </button>
        </div>

        {isLoading ? (
          <p className="text-slate-500 dark:text-slate-400">{t("loadingMetrics")}</p>
        ) : (
          <>
            <div className="mb-4 grid grid-cols-3 gap-3 text-center">
              <div className="rounded-lg bg-slate-50 p-3 dark:bg-slate-800/50">
                <p className="text-xs text-slate-500 dark:text-slate-400">{t("statUptime24h")}</p>
                <p className="text-xl font-bold text-emerald-600">
                  {data ? `${data.uptime_percent}%` : "—"}
                </p>
              </div>
              <div className="rounded-lg bg-slate-50 p-3 dark:bg-slate-800/50">
                <p className="text-xs text-slate-500 dark:text-slate-400">{t("statResponseNow")}</p>
                <p className="text-xl font-bold">{data ? `${Math.round(data.response_ms_current)}ms` : "—"}</p>
              </div>
              <div className="rounded-lg bg-slate-50 p-3 dark:bg-slate-800/50">
                <p className="text-xs text-slate-500 dark:text-slate-400">{t("statAvg24h")}</p>
                <p className="text-xl font-bold">{data ? `${Math.round(data.response_ms_avg)}ms` : "—"}</p>
              </div>
            </div>
            <p className="mb-1 text-xs font-medium text-slate-500 dark:text-slate-400">{t("responseTime24h")}</p>
            <Sparkline points={data?.response_ms ?? []} />
            <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">{t("metricsPrivacyNote")}</p>
          </>
        )}
      </div>
    </div>
  );
}

// Built via a factory so validation messages come from next-intl (see makeSchema above).
function makeEditSchema(t: (key: string) => string) {
  return z.object({
    name: z.string().min(1, t("nameRequired")),
    target: z.string().min(1, t("targetRequired")),
    interval_seconds: z.coerce.number().int().min(10).max(86400),
    valid_status_codes: z.string().optional(),
    body_keyword: z.string().optional(),
    follow_redirects: z.boolean().optional(),
    skip_tls_verify: z.boolean().optional(),
    response_time_warning_ms: z.coerce.number().int().min(0).optional(),
    ssl_expiry_warning_days: z.coerce.number().int().min(0).max(825).optional(),
    headers: z.string().optional(),
    dns_query_type: z.enum(["A", "AAAA", "CNAME", "MX", "TXT", "NS", "SOA", "CAA"]).optional(),
    alert_sensitivity: z.enum(["immediate", "balanced", "relaxed"]).optional(),
  });
}
type EditValues = z.infer<ReturnType<typeof makeEditSchema>>;

function EditMonitorModal({ monitor, onClose }: { monitor: Monitor; onClose: () => void }) {
  const t = useTranslations("pages.monitors");
  const c = useTranslations("common");
  const tv = useTranslations("validation");
  const editSchema = useMemo(() => makeEditSchema(tv), [tv]);
  const updateMonitor = useUpdateMonitor();
  const { data: usage } = useUsage();
  const [serverError, setServerError] = useState<string | null>(null);
  const isHTTP = monitor.type === "http" || monitor.type === "https" || monitor.type === "ssl";
  const s = monitor.settings ?? {};

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    defaultValues: {
      name: monitor.name,
      target: monitor.target,
      interval_seconds: monitor.interval_seconds,
      valid_status_codes: (s.valid_status_codes ?? []).join(", "),
      body_keyword: s.body_keyword ?? "",
      follow_redirects: s.follow_redirects ?? false,
      skip_tls_verify: s.skip_tls_verify ?? false,
      response_time_warning_ms: s.response_time_warning_ms,
      ssl_expiry_warning_days: s.ssl_expiry_warning_days,
      headers: Object.entries(s.headers ?? {})
        .map(([k, v]) => `${k}: ${v}`)
        .join("\n"),
      dns_query_type: (s.dns_query_type as EditValues["dns_query_type"]) ?? "A",
      alert_sensitivity: (s.alert_sensitivity as EditValues["alert_sensitivity"]) ?? "balanced",
    },
  });

  // Interval options honoring the plan floor, always including the current value.
  const minInterval = usage?.min_interval_seconds ?? 10;
  const opts = INTERVAL_OPTIONS.filter((o) => o.v >= minInterval);
  if (!opts.some((o) => o.v === monitor.interval_seconds)) {
    opts.unshift({ v: monitor.interval_seconds, key: "intervalCurrent" });
  }

  const onSubmit = async (values: EditValues) => {
    setServerError(null);
    try {
      await updateMonitor.mutateAsync({
        id: monitor.id,
        input: {
          name: values.name,
          target: values.target,
          interval_seconds: values.interval_seconds,
          settings: buildSettings(values),
        },
      });
      onClose();
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : t("updateError"));
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl border border-slate-200 bg-white p-5 shadow-xl dark:border-slate-800 dark:bg-slate-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="text-lg font-bold">{t("editTitle")}</h2>
            <p className="text-xs uppercase text-slate-500 dark:text-slate-400">{monitor.type}</p>
          </div>
          <button
            onClick={onClose}
            aria-label={c("close")}
            className="rounded p-1.5 text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 motion-reduce:transition-none dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
          >
            <XIcon className="h-4 w-4" />
          </button>
        </div>

        <form onSubmit={handleSubmit(onSubmit)} className="grid gap-4 sm:grid-cols-2">
          <Field label={t("fieldName")} error={errors.name?.message}>
            <Input {...register("name")} />
          </Field>
          <Field label={t("fieldCheckInterval")} error={errors.interval_seconds?.message}>
            <Select {...register("interval_seconds")}>
              {opts.map((o) => (
                <option key={o.v} value={o.v}>
                  {t(o.key, { seconds: o.v })}
                </option>
              ))}
            </Select>
          </Field>
          <div className="sm:col-span-2">
            <Field label={t("fieldTarget")} error={errors.target?.message}>
              <Input {...register("target")} />
            </Field>
          </div>
          <div className="sm:col-span-2">
            <Field label={t("fieldAlertSensitivity")} error={errors.alert_sensitivity?.message}>
              <Select {...register("alert_sensitivity")}>
                {SENSITIVITY_OPTIONS.map((o) => (
                  <option key={o.v} value={o.v}>
                    {t(o.key)}
                  </option>
                ))}
              </Select>
            </Field>
          </div>

          {isHTTP && (
            <>
              <Field label={t("fieldExpectedStatusCodes")} error={errors.valid_status_codes?.message}>
                <Input placeholder="200, 204, 301" {...register("valid_status_codes")} />
              </Field>
              <Field label={t("fieldBodyKeyword")} error={errors.body_keyword?.message}>
                <Input {...register("body_keyword")} />
              </Field>
              <Field label={t("fieldSlowResponse")} error={errors.response_time_warning_ms?.message}>
                <Input type="number" {...register("response_time_warning_ms")} />
              </Field>
              <Field label={t("fieldSslExpiry")} error={errors.ssl_expiry_warning_days?.message}>
                <Input type="number" {...register("ssl_expiry_warning_days")} />
              </Field>
              <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
                <input type="checkbox" className="h-4 w-4" {...register("follow_redirects")} /> {t("fieldFollowRedirects")}
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
                <input type="checkbox" className="h-4 w-4" {...register("skip_tls_verify")} /> {t("fieldSkipTls")}
              </label>
              <div className="sm:col-span-2">
                <Field label={t("fieldCustomHeaders")} error={errors.headers?.message}>
                  <Textarea rows={3} {...register("headers")} />
                </Field>
              </div>
            </>
          )}

          {monitor.type === "dns" && (
            <Field label={t("fieldDnsRecordType")} error={errors.dns_query_type?.message}>
              <Select {...register("dns_query_type")}>
                {["A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA"].map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </Select>
            </Field>
          )}

          {serverError && <p className="text-sm text-red-600 sm:col-span-2">{serverError}</p>}
          <div className="flex gap-2 sm:col-span-2">
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("savingLabel") : t("saveChanges")}
            </Button>
            <Button type="button" variant="secondary" onClick={onClose}>
              {c("cancel")}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

// Labels are resolved via t() at the render site (see the `key`).
const INTERVAL_OPTIONS = [
  { v: 30, key: "intervalEvery30Seconds" },
  { v: 60, key: "intervalEveryMinute" },
  { v: 300, key: "intervalEvery5Minutes" },
  { v: 900, key: "intervalEvery15Minutes" },
];

function CreateMonitorForm({ onDone }: { onDone: () => void }) {
  const t = useTranslations("pages.monitors");
  const tv = useTranslations("validation");
  const schema = useMemo(() => makeSchema(tv), [tv]);
  const { data: projects } = useProjects();
  const { data: usage } = useUsage();
  const minInterval = usage?.min_interval_seconds ?? 10;
  const createMonitor = useCreateMonitor();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    control,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { type: "https", interval_seconds: 60, alert_sensitivity: "balanced" },
  });

  const [showAdvanced, setShowAdvanced] = useState(false);
  const [created, setCreated] = useState<Monitor | null>(null);
  // useWatch subscribes this component to one field. watch() re-reads the whole form
  // through a closure the compiler cannot see through, so it is opaque to
  // memoization; useWatch is the subscription-shaped API built for that.
  const type = useWatch({ control, name: "type" });
  const isHTTP = type === "http" || type === "https" || type === "ssl";
  const isHeartbeat = type === "heartbeat";

  const onSubmit = async (values: Values) => {
    setServerError(null);
    try {
      const monitor = await createMonitor.mutateAsync({
        project_id: values.project_id,
        name: values.name,
        type: values.type,
        // A heartbeat has no probe target; the server assigns a placeholder.
        target: isHeartbeat ? undefined : values.target,
        interval_seconds: values.interval_seconds,
        grace_seconds: isHeartbeat ? values.grace_seconds : undefined,
        settings: buildSettings(values),
      });
      // A heartbeat's ping URL is only shown once, right after creation — so we
      // pause on a success panel instead of closing the form immediately.
      if (monitor.type === "heartbeat" && monitor.ping_url) {
        setCreated(monitor);
      } else {
        onDone();
      }
    } catch (err) {
      setServerError(err instanceof ApiRequestError ? err.message : t("createError"));
    }
  };

  if (created) {
    return <HeartbeatCreated monitor={created} onDone={onDone} />;
  }

  if (!projects?.data.length) {
    return (
      <Card>
        <p className="text-slate-500 dark:text-slate-400">
          {t.rich("noProjectsBody", {
            b: (chunks) => <span className="font-medium">{chunks}</span>,
          })}
        </p>
      </Card>
    );
  }

  return (
    <Card>
      <form onSubmit={handleSubmit(onSubmit)} className="grid gap-4 sm:grid-cols-2">
        <Field label={t("fieldProject")} error={errors.project_id?.message}>
          <Select {...register("project_id")}>
            <option value="">{t("selectProjectPlaceholder")}</option>
            {projects.data.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("fieldName")} error={errors.name?.message}>
          <Input placeholder={t("namePlaceholder")} {...register("name")} />
        </Field>
        <Field label={t("fieldType")} error={errors.type?.message}>
          <Select {...register("type")}>
            <option value="https">{t("typeHttps")}</option>
            <option value="http">{t("typeHttp")}</option>
            <option value="ssl">{t("typeSsl")}</option>
            <option value="tcp">{t("typeTcp")}</option>
            <option value="icmp">{t("typeIcmp")}</option>
            <option value="dns">{t("typeDns")}</option>
            <option value="heartbeat">{t("typeHeartbeat")}</option>
          </Select>
        </Field>
        <Field label={t("fieldCheckInterval")} error={errors.interval_seconds?.message}>
          <Select {...register("interval_seconds")}>
            {INTERVAL_OPTIONS.filter((o) => o.v >= minInterval).map((o) => (
              <option key={o.v} value={o.v}>
                {t(o.key, { seconds: o.v })}
              </option>
            ))}
          </Select>
          {usage && minInterval > 30 && (
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              {t("intervalFloorNote", { plan: usage.plan, seconds: minInterval })}
            </p>
          )}
        </Field>
        {isHeartbeat ? (
          <>
            <Field label={t("fieldGracePeriod")} error={errors.grace_seconds?.message}>
              <Select {...register("grace_seconds")}>
                <option value="">{t("graceOneInterval")}</option>
                <option value="60">{t("grace1Minute")}</option>
                <option value="300">{t("grace5Minutes")}</option>
                <option value="900">{t("grace15Minutes")}</option>
                <option value="3600">{t("grace1Hour")}</option>
              </Select>
            </Field>
            <div className="sm:col-span-2 rounded-lg bg-blue-50 p-3 text-xs text-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
              {t("heartbeatInfo", { brand: brand.name })}
            </div>
          </>
        ) : (
          <div className="sm:col-span-2">
            <Field label={t("fieldTarget")} error={errors.target?.message}>
              <Input placeholder={targetHints[type]} {...register("target")} />
            </Field>
          </div>
        )}

        {/* Sensitivity is a probe concept (how many failed checks before alerting);
            a heartbeat uses its grace period instead, so it is hidden here. */}
        {!isHeartbeat && (
          <div className="sm:col-span-2">
            <Field label={t("fieldAlertSensitivity")} error={errors.alert_sensitivity?.message}>
              <Select {...register("alert_sensitivity")}>
                {SENSITIVITY_OPTIONS.map((o) => (
                  <option key={o.v} value={o.v}>
                    {t(o.key)}
                  </option>
                ))}
              </Select>
            </Field>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              {t("sensitivityHelp")}
            </p>
          </div>
        )}

        {!isHeartbeat && (
          <div className="sm:col-span-2 border-t border-slate-200 pt-3 dark:border-slate-800">
            <button
              type="button"
              onClick={() => setShowAdvanced((v) => !v)}
              className="text-sm font-medium text-brand-600 hover:underline"
            >
              {showAdvanced ? t("hideAdvanced") : t("showAdvanced")}
            </button>
          </div>
        )}

        {showAdvanced && isHTTP && (
          <>
            <Field label={t("fieldExpectedStatusCodes")} error={errors.valid_status_codes?.message}>
              <Input placeholder={t("statusCodesPlaceholder")} {...register("valid_status_codes")} />
            </Field>
            <Field label={t("fieldBodyKeyword")} error={errors.body_keyword?.message}>
              <Input placeholder={t("bodyKeywordPlaceholder")} {...register("body_keyword")} />
            </Field>
            <Field label={t("fieldSlowResponse")} error={errors.response_time_warning_ms?.message}>
              <Input type="number" placeholder="2000" {...register("response_time_warning_ms")} />
            </Field>
            <Field label={t("fieldSslExpiry")} error={errors.ssl_expiry_warning_days?.message}>
              <Input type="number" placeholder="30" {...register("ssl_expiry_warning_days")} />
            </Field>
            <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
              <input type="checkbox" className="h-4 w-4" {...register("follow_redirects")} /> {t("fieldFollowRedirects")}
            </label>
            <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
              <input type="checkbox" className="h-4 w-4" {...register("skip_tls_verify")} /> {t("fieldSkipTls")}
            </label>
            <div className="sm:col-span-2">
              <Field label={t("fieldCustomHeadersAuth")} error={errors.headers?.message}>
                <Textarea rows={3} placeholder={"Authorization: Bearer xxx\nX-API-Key: abc123"} {...register("headers")} />
              </Field>
            </div>
          </>
        )}

        {showAdvanced && type === "dns" && (
          <Field label={t("fieldDnsRecordType")} error={errors.dns_query_type?.message}>
            <Select {...register("dns_query_type")}>
              {["A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA"].map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </Select>
          </Field>
        )}

        {serverError && <p className="text-sm text-red-600 sm:col-span-2">{serverError}</p>}
        <div className="sm:col-span-2 flex items-center gap-3">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("creatingLabel") : t("createMonitor")}
          </Button>
          <span className="text-xs text-slate-500 dark:text-slate-400">
            {t("autoConfigNote")}
          </span>
        </div>
      </form>
    </Card>
  );
}
