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
import { PlusIcon, WrenchIcon, XIcon } from "@/components/icons";
import type { MaintenanceScope, MaintenanceWindow } from "@/lib/types";

type Notice = { kind: "ok" | "err"; text: string } | null;

// The status page and alerting both key off the server clock, so windows are
// entered in the operator's local time and converted to an absolute instant here.
function toISO(local: string): string {
  return new Date(local).toISOString();
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

export default function MaintenancePage() {
  const { data, isLoading } = useMaintenanceWindows();
  const [showForm, setShowForm] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);

  const windows = data?.data ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Maintenance windows"
        subtitle="Schedule planned downtime: alerts are suppressed and the status page shows “under maintenance” instead of an outage."
        actions={
          <Button onClick={() => setShowForm((v) => !v)}>
            {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
            {showForm ? "Close" : "Schedule window"}
          </Button>
        }
      />

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
          Before a deploy or planned outage, schedule a window so Beacon holds back the alerts and your
          public status page shows planned work rather than a red “major outage”.
        </EmptyState>
      ) : (
        <div className="space-y-3">
          {windows.map((w) => (
            <WindowRow key={w.id} window={w} setNotice={setNotice} />
          ))}
        </div>
      )}
    </div>
  );
}

function WindowRow({ window: w, setNotice }: { window: MaintenanceWindow; setNotice: (n: Notice) => void }) {
  const del = useDeleteMaintenanceWindow();
  const { data: projects } = useProjects();
  const { data: monitors } = useMonitors();

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
          onClick={() => {
            if (confirm(`Delete maintenance window "${w.title}"?`)) {
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

  function toggleId(id: string) {
    setIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  }

  function selectScope(next: MaintenanceScope) {
    setScope(next);
    setIds([]); // ids only meaningful within one scope
    setErrors({});
  }

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

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Starts" error={errors.startsAt}>
            <Input type="datetime-local" value={startsAt} onChange={(e) => setStartsAt(e.target.value)} />
          </Field>
          <Field label="Ends" error={errors.endsAt}>
            <Input type="datetime-local" value={endsAt} onChange={(e) => setEndsAt(e.target.value)} />
          </Field>
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
