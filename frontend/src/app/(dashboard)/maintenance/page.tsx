"use client";

import { useMemo, useState } from "react";

import {
  useCreateMaintenanceWindow,
  useDeleteMaintenanceWindow,
  useMaintenanceWindows,
  useMonitors,
  useProjects,
  type MaintenanceWindowInput,
} from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Field, Input, PageHeader, Skeleton, Textarea } from "@/components/ui";
import { useConfirm } from "@/components/confirm";
import { Pagination } from "@/components/table-controls";
import { PlusIcon, WrenchIcon, XIcon } from "@/components/icons";
import type { MaintenanceScope, MaintenanceWindow } from "@/lib/types";

type Notice = { kind: "ok" | "err"; text: string } | null;

const MAINTENANCE_PAGE_SIZE = 20;

// The status page and alerting both key off the server clock, so windows are
// entered in the operator's local time and converted to an absolute instant here.
function toISO(local: string): string {
  return new Date(local).toISOString();
}

// toLocalInput renders a Date as the "YYYY-MM-DDTHH:mm" a datetime-local input
// expects, in the browser's local time (so "Start now" reads as the wall clock).
function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatRange(startsAt: string, endsAt: string): string {
  const fmt = (iso: string) =>
    new Date(iso).toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  return `${fmt(startsAt)} → ${fmt(endsAt)}`;
}

// The viewer's timezone, shown so nobody wonders which clock the times use. Guarded
// because a rare environment can throw resolving it.
function localTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "your local time";
  } catch {
    return "your local time";
  }
}

// HowItWorks is the plain-language explainer at the top of the page. Maintenance is
// a passive feature — the most common confusion is expecting it to "do" something —
// so we spell out what it changes and that it runs on its own.
function HowItWorks() {
  const steps = [
    {
      n: "1",
      title: "Pick a time and what it covers",
      body: "Your whole organisation, some projects, or specific monitors.",
    },
    {
      n: "2",
      title: "While it’s active",
      body: "Beacon Pulse pauses alerts for those monitors and shows “Under maintenance” on your public status page — instead of a red outage.",
    },
    {
      n: "3",
      title: "It’s automatic",
      body: "Nothing to switch on or off. It starts and ends on its own; when it ends, alerts resume.",
    },
  ];
  return (
    <div className="rounded-xl border border-blue-200 bg-blue-50/60 p-4 dark:border-blue-900/40 dark:bg-blue-950/30">
      <div className="flex items-center gap-2">
        <WrenchIcon className="h-4 w-4 shrink-0 text-blue-700 dark:text-blue-300" />
        <p className="text-sm font-semibold text-blue-900 dark:text-blue-100">How maintenance windows work</p>
      </div>
      <ol className="mt-3 grid gap-3 sm:grid-cols-3">
        {steps.map((s) => (
          <li key={s.n} className="flex gap-2.5">
            <span
              aria-hidden
              className="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-blue-600 text-xs font-semibold text-white"
            >
              {s.n}
            </span>
            <p className="text-sm text-blue-900/80 dark:text-blue-100/80">
              <span className="font-semibold text-blue-900 dark:text-blue-100">{s.title}.</span> {s.body}
            </p>
          </li>
        ))}
      </ol>
    </div>
  );
}

export default function MaintenancePage() {
  const [page, setPage] = useState(0);
  const { data, isLoading, isPlaceholderData } = useMaintenanceWindows({
    page,
    pageSize: MAINTENANCE_PAGE_SIZE,
  });
  const [showForm, setShowForm] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);

  const windows = data?.data ?? [];
  const total = data?.pagination.total ?? 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Maintenance windows"
        subtitle="Planning a deploy or known downtime? Tell Beacon Pulse in advance so it doesn’t cry wolf."
        actions={
          <Button onClick={() => setShowForm((v) => !v)}>
            {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
            {showForm ? "Close" : "Schedule window"}
          </Button>
        }
      />

      <HowItWorks />

      {notice && (
        <div
          role={notice.kind === "ok" ? "status" : "alert"}
          className={`rounded-lg px-4 py-2 text-sm font-medium ${
            notice.kind === "ok"
              ? "bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200"
              : "bg-red-50 text-red-800 dark:bg-red-900/30 dark:text-red-200"
          }`}
        >
          {notice.text}
        </div>
      )}

      {showForm && <CreateWindowForm onDone={() => setShowForm(false)} setNotice={setNotice} />}

      {isLoading ? (
        <div className="space-y-3">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      ) : !windows.length ? (
        <EmptyState
          icon={<WrenchIcon className="h-5 w-5" />}
          title="No maintenance windows"
          action={
            <Button onClick={() => setShowForm(true)}>
              <PlusIcon className="h-4 w-4" />
              Schedule window
            </Button>
          }
        >
          Before a deploy or planned outage, schedule a window so Beacon Pulse holds back the alerts and your
          public status page shows planned work rather than a red “major outage”.
        </EmptyState>
      ) : (
        <>
          <div className={`space-y-3 ${isPlaceholderData ? "opacity-60 transition-opacity" : "transition-opacity"}`}>
            {windows.map((w) => (
              <WindowRow key={w.id} window={w} setNotice={setNotice} />
            ))}
          </div>
          <Pagination
            page={page}
            pageSize={MAINTENANCE_PAGE_SIZE}
            total={total}
            unit="windows"
            busy={isPlaceholderData}
            onPageChange={setPage}
          />
        </>
      )}
    </div>
  );
}

function WindowRow({ window: w, setNotice }: { window: MaintenanceWindow; setNotice: (n: Notice) => void }) {
  const del = useDeleteMaintenanceWindow();
  const { data: projects } = useProjects();
  const { data: monitors } = useMonitors();
  const confirm = useConfirm();

  const scopeLabel = useMemo(() => {
    if (w.scope === "org") return "All monitors";
    if (w.scope === "project") {
      const names = w.scope_ids
        .map((id) => projects?.data.find((p) => p.id === id)?.name)
        .filter(Boolean);
      return names.length ? `Projects: ${names.join(", ")}` : `${w.scope_ids.length} project(s)`;
    }
    const names = w.scope_ids
      .map((id) => monitors?.data.find((m) => m.id === id)?.name)
      .filter(Boolean);
    return names.length ? `Monitors: ${names.join(", ")}` : `${w.scope_ids.length} monitor(s)`;
  }, [w, projects, monitors]);

  const ended = new Date(w.ends_at).getTime() < Date.now();

  return (
    <Card className="flex flex-wrap items-center justify-between gap-3">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium">{w.title}</span>
          {w.active ? (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/40 dark:text-blue-200">
              <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden />
              Active now
            </span>
          ) : ended ? (
            <span className="rounded-full bg-slate-200 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400">
              Ended
            </span>
          ) : (
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              Scheduled
            </span>
          )}
        </div>
        <p className="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">
          <span className="font-mono">{formatRange(w.starts_at, w.ends_at)}</span> · {scopeLabel}
        </p>
        {w.description && (
          <p className="mt-1 max-w-2xl truncate text-xs text-slate-500 dark:text-slate-400">{w.description}</p>
        )}
      </div>
      <div className="flex shrink-0 gap-2">
        <Button
          variant="danger"
          disabled={del.isPending}
          onClick={async () => {
            if (
              await confirm({
                title: `Delete “${w.title}”?`,
                body: w.active
                  ? "This window is active now — deleting it resumes alerts for its monitors immediately."
                  : "This removes the scheduled maintenance window.",
                confirmLabel: "Delete window",
                danger: true,
              })
            ) {
              del.mutate(w.id, {
                onError: (e) =>
                  setNotice({ kind: "err", text: e instanceof ApiRequestError ? e.message : "Delete failed" }),
              });
            }
          }}
        >
          Delete
        </Button>
      </div>
    </Card>
  );
}

const SCOPES: { value: MaintenanceScope; label: string; blurb: string }[] = [
  { value: "org", label: "Whole org", blurb: "Every monitor in the organization." },
  { value: "project", label: "By project", blurb: "Every monitor in the chosen projects." },
  { value: "monitor", label: "Specific monitors", blurb: "Only the monitors you pick." },
];

// A window longer than this gets a soft warning — a permanent window silently
// blinds alerting, so it should be a deliberate choice, not an accident.
const LONG_WINDOW_HOURS = 24;

function CreateWindowForm({ onDone, setNotice }: { onDone: () => void; setNotice: (n: Notice) => void }) {
  const create = useCreateMaintenanceWindow();
  const { data: projects } = useProjects();
  const { data: monitors } = useMonitors();

  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [startsAt, setStartsAt] = useState("");
  const [endsAt, setEndsAt] = useState("");
  const [scope, setScope] = useState<MaintenanceScope>("org");
  const [ids, setIds] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  const tz = localTimezone();

  function toggleId(id: string) {
    setIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  }

  function selectScope(next: MaintenanceScope) {
    setScope(next);
    setIds([]); // ids only meaningful within one scope
    setErrors({});
  }

  // "Start now" one-click fill — the fix for the commonest mistake, scheduling a
  // window that isn't actually active yet. Defaults the end to one hour out.
  function fillStartNow() {
    const now = new Date();
    setStartsAt(toLocalInput(now));
    if (!endsAt) setEndsAt(toLocalInput(new Date(now.getTime() + 60 * 60 * 1000)));
  }

  // Plain-language summary of what this window will cover, shown live in the form.
  const coverage =
    scope === "org"
      ? "every monitor in your organisation"
      : scope === "project"
        ? `monitors in ${ids.length || "the selected"} project${ids.length === 1 ? "" : "s"}`
        : `${ids.length || "the selected"} monitor${ids.length === 1 ? "" : "s"}`;

  const longWarning =
    startsAt && endsAt && new Date(endsAt).getTime() - new Date(startsAt).getTime() > LONG_WINDOW_HOURS * 3600_000;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (!title.trim()) errs.title = "Title is required";
    if (!startsAt) errs.startsAt = "Start is required";
    if (!endsAt) errs.endsAt = "End is required";
    if (startsAt && endsAt && new Date(endsAt) <= new Date(startsAt)) errs.endsAt = "End must be after start";
    if (scope !== "org" && ids.length === 0) errs.ids = "Pick at least one";
    setErrors(errs);
    if (Object.keys(errs).length) return;

    const payload: MaintenanceWindowInput = {
      title: title.trim(),
      description: description.trim() || undefined,
      starts_at: toISO(startsAt),
      ends_at: toISO(endsAt),
      scope,
      scope_ids: scope === "org" ? [] : ids,
    };

    setSubmitting(true);
    try {
      await create.mutateAsync(payload);
      setNotice({ kind: "ok", text: `Maintenance window “${payload.title}” scheduled.` });
      onDone();
    } catch (err) {
      setNotice({ kind: "err", text: err instanceof ApiRequestError ? err.message : "Failed to schedule window" });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card>
      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Title" hint="Shown on the public status page banner." error={errors.title}>
          <Input placeholder="Database upgrade" value={title} onChange={(e) => setTitle(e.target.value)} />
        </Field>

        <Field label="Description" hint="Optional context for your team.">
          <Textarea
            rows={2}
            placeholder="Rolling Postgres 15 → 16; brief write pauses expected."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </Field>

        <div>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Starts" error={errors.startsAt}>
              <Input type="datetime-local" value={startsAt} onChange={(e) => setStartsAt(e.target.value)} />
            </Field>
            <Field label="Ends" error={errors.endsAt}>
              <Input type="datetime-local" value={endsAt} onChange={(e) => setEndsAt(e.target.value)} />
            </Field>
          </div>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1">
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Times are in your timezone ({tz}). To take effect right away, start it now.
            </p>
            <button
              type="button"
              onClick={fillStartNow}
              className="rounded text-xs font-medium text-brand-700 underline-offset-2 hover:underline focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:text-brand-400"
            >
              Start now →
            </button>
          </div>
        </div>

        {longWarning && (
          <p className="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-900/30 dark:text-amber-200">
            This window is longer than {LONG_WINDOW_HOURS} hours. Alerts stay suppressed the whole time —
            make sure that is intended.
          </p>
        )}

        <div>
          <span className="mb-1 block text-sm font-medium text-slate-700 dark:text-slate-300">Applies to</span>
          <div role="group" aria-label="Scope" className="flex flex-wrap gap-2">
            {SCOPES.map((s) => (
              <button
                key={s.value}
                type="button"
                onClick={() => selectScope(s.value)}
                aria-pressed={scope === s.value}
                className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors motion-reduce:transition-none ${
                  scope === s.value
                    ? "border-brand-600 bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300"
                    : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300"
                }`}
              >
                {s.label}
              </button>
            ))}
          </div>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            {SCOPES.find((s) => s.value === scope)?.blurb}
          </p>
        </div>

        {scope === "project" && (
          <SelectList
            label="Projects"
            error={errors.ids}
            items={(projects?.data ?? []).map((p) => ({ id: p.id, label: p.name, sub: p.environment }))}
            selected={ids}
            onToggle={toggleId}
            empty="No projects yet."
          />
        )}
        {scope === "monitor" && (
          <SelectList
            label="Monitors"
            error={errors.ids}
            items={(monitors?.data ?? []).map((m) => ({ id: m.id, label: m.name, sub: m.type }))}
            selected={ids}
            onToggle={toggleId}
            empty="No monitors yet."
          />
        )}

        {/* Live plain-language summary, so the person scheduling sees exactly what
            it will do before they commit. */}
        <p className="rounded-lg bg-slate-50 px-3 py-2.5 text-sm text-slate-600 dark:bg-slate-800/50 dark:text-slate-300">
          While active, Beacon Pulse will <span className="font-medium text-slate-900 dark:text-slate-100">pause alerts</span>{" "}
          for {coverage} and mark them{" "}
          <span className="font-medium text-slate-900 dark:text-slate-100">“Under maintenance”</span> on your public
          status page.
        </p>

        <Button type="submit" disabled={submitting}>
          {submitting ? "Scheduling…" : "Schedule window"}
        </Button>
      </form>
    </Card>
  );
}

function SelectList({
  label,
  error,
  items,
  selected,
  onToggle,
  empty,
}: {
  label: string;
  error?: string;
  items: { id: string; label: string; sub?: string }[];
  selected: string[];
  onToggle: (id: string) => void;
  empty: string;
}) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <span className="text-sm font-medium text-slate-700 dark:text-slate-300">{label}</span>
        <span className="text-xs text-slate-500 dark:text-slate-400">{selected.length} selected</span>
      </div>
      {items.length === 0 ? (
        <p className="rounded-lg border border-dashed border-slate-300 px-3 py-4 text-center text-xs text-slate-500 dark:border-slate-700 dark:text-slate-400">
          {empty}
        </p>
      ) : (
        <div className="max-h-52 space-y-1 overflow-y-auto rounded-lg border border-slate-200 p-2 dark:border-slate-700">
          {items.map((it) => (
            <label
              key={it.id}
              className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 text-sm hover:bg-slate-50 dark:hover:bg-slate-800"
            >
              <input
                type="checkbox"
                checked={selected.includes(it.id)}
                onChange={() => onToggle(it.id)}
                className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500"
              />
              <span className="truncate text-slate-800 dark:text-slate-100">{it.label}</span>
              {it.sub && <span className="ml-auto shrink-0 text-xs capitalize text-slate-500 dark:text-slate-400">{it.sub}</span>}
            </label>
          ))}
        </div>
      )}
      {error && (
        <p role="alert" className="mt-1 text-xs font-medium text-red-700 dark:text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
