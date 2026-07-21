"use client";

import Link from "next/link";
import { useMemo, useState, useSyncExternalStore } from "react";

import { CheckIcon, LockIcon } from "@/components/icons";

/**
 * A Swagger-style console: paste a key, pick an endpoint, send a real request, read the
 * real response — without leaving the docs or reaching for curl.
 *
 * It calls the LIVE API with the reader's own key. That is the point and also the whole
 * risk, so the design leans on three things:
 *
 *   - The base URL is same-origin, so on int/test/prod the console talks to the API on
 *     the very host the docs are served from. No CORS, no configuration, and the request
 *     is provably going where the reader thinks it is.
 *   - The key lives in this browser's localStorage and is sent to nowhere but that API.
 *     Stated plainly, because "paste your credential here" has to earn trust.
 *   - Writes are marked, and a destructive call is confirmed. A console that quietly
 *     deletes a monitor because someone was exploring is a console people stop trusting.
 */

const STORAGE_KEY = "beacon.docs.apiKey";

// The key is a value the BROWSER owns, so it is subscribed to rather than copied into
// React state on mount — the same reason theme.tsx does this. Reading localStorage in
// an effect would trip the set-state-in-effect rule and flash an empty field on every
// load; a store settles both.
const keyListeners = new Set<() => void>();
function subscribeKey(cb: () => void) {
  keyListeners.add(cb);
  window.addEventListener("storage", cb); // another tab changed it
  return () => {
    keyListeners.delete(cb);
    window.removeEventListener("storage", cb);
  };
}
function readKey(): string {
  try {
    return localStorage.getItem(STORAGE_KEY) ?? "";
  } catch {
    return "";
  }
}
function writeKey(value: string) {
  try {
    if (value) localStorage.setItem(STORAGE_KEY, value);
    else localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* storage blocked (private mode): the console still works, just without persistence */
  }
  for (const cb of keyListeners) cb();
}

function useApiKey(): [string, (v: string) => void] {
  // "" on the server, matching a first client render with no key, so hydration agrees.
  const key = useSyncExternalStore(subscribeKey, readKey, () => "");
  // writeKey is a stable module-level function — no useCallback needed to keep its
  // identity across renders, because it already has one.
  return [key, writeKey];
}

type Method = "GET" | "POST" | "PATCH" | "DELETE";

interface Param {
  name: string;
  where: "path" | "query";
  required?: boolean;
  placeholder?: string;
  hint?: string;
}

interface Endpoint {
  id: string;
  method: Method;
  path: string;
  title: string;
  desc: string;
  params?: Param[];
  body?: string;
  writes?: boolean;
  noAuth?: boolean;
}

// Curated rather than generated: the endpoints someone actually reaches for, safe ones
// first, each with a body a reader can send as-is and then edit.
const GROUPS: { label: string; endpoints: Endpoint[] }[] = [
  {
    label: "Start here",
    endpoints: [
      {
        id: "system-info",
        method: "GET",
        path: "/api/v1/system/info",
        title: "System info",
        desc: "Needs no key — send it first to confirm the console reaches the API.",
        noAuth: true,
      },
      {
        id: "list-monitors",
        method: "GET",
        path: "/api/v1/monitors",
        title: "List monitors",
        desc: "Everything you are watching.",
        params: [
          { name: "status", where: "query", placeholder: "down", hint: "up | down | degraded | paused" },
          { name: "search", where: "query", placeholder: "example.com" },
          { name: "limit", where: "query", placeholder: "50" },
        ],
      },
      {
        id: "usage",
        method: "GET",
        path: "/api/v1/monitors/usage",
        title: "Plan usage",
        desc: "How many monitors you have, against your plan's limit.",
      },
      {
        id: "alerts",
        method: "GET",
        path: "/api/v1/alerts",
        title: "Firing alerts",
        desc: "What is broken right now.",
        params: [{ name: "severity", where: "query", placeholder: "critical", hint: "critical | warning" }],
      },
      {
        id: "billing",
        method: "GET",
        path: "/api/v1/billing",
        title: "Billing",
        desc: "Plan, credit, and what remains.",
      },
    ],
  },
  {
    label: "Projects & monitors",
    endpoints: [
      {
        id: "list-projects",
        method: "GET",
        path: "/api/v1/projects",
        title: "List projects",
        desc: "Your projects — you'll need a project id to create a monitor.",
      },
      {
        id: "create-project",
        method: "POST",
        path: "/api/v1/projects",
        title: "Create a project",
        desc: "A group for monitors.",
        writes: true,
        body: `{
  "name": "Production",
  "environment": "production"
}`,
      },
      {
        id: "create-monitor",
        method: "POST",
        path: "/api/v1/monitors",
        title: "Create a monitor",
        desc: "Put a real project_id from the call above.",
        writes: true,
        body: `{
  "project_id": "PASTE-A-PROJECT-ID",
  "name": "example",
  "type": "https",
  "target": "https://example.com",
  "interval_seconds": 60
}`,
      },
      {
        id: "get-monitor",
        method: "GET",
        path: "/api/v1/monitors/{id}",
        title: "Get one monitor",
        desc: "By id.",
        params: [{ name: "id", where: "path", required: true, placeholder: "a monitor id" }],
      },
      {
        id: "pause-monitor",
        method: "POST",
        path: "/api/v1/monitors/{id}/pause",
        title: "Pause a monitor",
        desc: "Stop probing without deleting.",
        writes: true,
        params: [{ name: "id", where: "path", required: true, placeholder: "a monitor id" }],
      },
      {
        id: "delete-monitor",
        method: "DELETE",
        path: "/api/v1/monitors/{id}",
        title: "Delete a monitor",
        desc: "Removes it and its history. Confirmed before sending.",
        writes: true,
        params: [{ name: "id", where: "path", required: true, placeholder: "a monitor id" }],
      },
    ],
  },
  {
    label: "Declarative sync",
    endpoints: [
      {
        id: "sync-dryrun",
        method: "POST",
        path: "/api/v1/sync",
        title: "Sync (dry run)",
        desc: "Shows what applying this file would do, and changes nothing.",
        writes: true,
        body: `{
  "project": "production",
  "dry_run": true,
  "monitors": [
    { "name": "www", "type": "https", "target": "https://example.com" }
  ]
}`,
      },
    ],
  },
];

const ALL = GROUPS.flatMap((g) => g.endpoints);

const METHOD_STYLE: Record<Method, string> = {
  GET: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300",
  POST: "bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300",
  PATCH: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  DELETE: "bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300",
};

interface Result {
  status: number;
  statusText: string;
  ms: number;
  body: string;
  ok: boolean;
}

export function ApiConsole() {
  const [apiKey, setApiKey] = useApiKey();
  const [selectedId, setSelectedId] = useState(ALL[0].id);
  const endpoint = useMemo(() => ALL.find((e) => e.id === selectedId) ?? ALL[0], [selectedId]);

  return (
    <div className="not-prose space-y-5">
      <KeyField apiKey={apiKey} setApiKey={setApiKey} />

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
        {/* Keyed on the endpoint id so switching endpoints REMOUNTS the request panel
            with fresh inputs — a POST body must not linger into a GET. A remount is the
            honest way to reset per-endpoint state, rather than an effect reconciling it
            after the fact. */}
        <RequestPanel key={endpoint.id} endpoint={endpoint} apiKey={apiKey} onSelect={setSelectedId} selectedId={selectedId} />
      </div>
    </div>
  );
}

function KeyField({ apiKey, setApiKey }: { apiKey: string; setApiKey: (v: string) => void }) {
  return (
    <div className="rounded-xl border border-slate-200 p-4 dark:border-slate-800">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <label htmlFor="console-key" className="text-sm font-semibold text-slate-900 dark:text-white">
          Your API key
        </label>
        {apiKey && (
          <span className="inline-flex items-center gap-1 text-xs text-emerald-700 dark:text-emerald-400">
            <CheckIcon className="h-3 w-3" /> saved in this browser
          </span>
        )}
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <input
          id="console-key"
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="bp_…"
          autoComplete="off"
          spellCheck={false}
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-sm text-slate-900 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
        />
        {apiKey && (
          <button
            type="button"
            onClick={() => setApiKey("")}
            className="shrink-0 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-600 hover:bg-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            Clear
          </button>
        )}
      </div>
      <p className="mt-2 flex items-start gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <LockIcon className="mt-0.5 h-3 w-3 shrink-0" />
        <span>
          Stored only in this browser, and sent only to this Beacon API. Use a{" "}
          <span className="font-medium">viewer</span> key while exploring —{" "}
          <Link href="/api-keys" className="text-blue-700 underline dark:text-blue-400">
            create one
          </Link>{" "}
          (you&apos;ll be asked to sign in).
        </span>
      </p>
    </div>
  );
}

function RequestPanel({
  endpoint,
  apiKey,
  onSelect,
  selectedId,
}: {
  endpoint: Endpoint;
  apiKey: string;
  onSelect: (id: string) => void;
  selectedId: string;
}) {
  // Fresh on every mount, and the panel remounts per endpoint (see the key above), so
  // no effect is needed to reset these.
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const [bodyText, setBodyText] = useState(endpoint.body ?? "");
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const [error, setError] = useState<string | null>(null);

  const buildUrl = (): string => {
    let path = endpoint.path;
    const query = new URLSearchParams();
    for (const p of endpoint.params ?? []) {
      const v = paramValues[p.name]?.trim();
      if (!v) continue;
      if (p.where === "path") path = path.replace(`{${p.name}}`, encodeURIComponent(v));
      else query.set(p.name, v);
    }
    const qs = query.toString();
    return path + (qs ? `?${qs}` : "");
  };

  const send = async () => {
    setError(null);

    for (const p of endpoint.params ?? []) {
      if (p.required && !paramValues[p.name]?.trim()) {
        setError(`${p.name} is required.`);
        return;
      }
    }
    if (!endpoint.noAuth && !apiKey.trim()) {
      setError("Add an API key above first.");
      return;
    }
    if (endpoint.body && bodyText.trim()) {
      try {
        JSON.parse(bodyText);
      } catch (e) {
        setError(`The request body is not valid JSON: ${e instanceof Error ? e.message : "parse error"}`);
        return;
      }
    }
    if (endpoint.method === "DELETE") {
      if (!window.confirm(`This sends a real DELETE to ${buildUrl()} and cannot be undone. Continue?`)) {
        return;
      }
    }

    setSending(true);
    setResult(null);
    const started = performance.now();
    try {
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (apiKey.trim()) headers.Authorization = `Bearer ${apiKey.trim()}`;

      const res = await fetch(buildUrl(), {
        method: endpoint.method,
        headers,
        body: endpoint.body && bodyText.trim() ? bodyText : undefined,
      });
      const text = await res.text();
      let pretty = text;
      try {
        pretty = JSON.stringify(JSON.parse(text), null, 2);
      } catch {
        /* not JSON — show it raw */
      }
      setResult({
        status: res.status,
        statusText: res.statusText,
        ms: Math.round(performance.now() - started),
        body: pretty || "(empty response)",
        ok: res.ok,
      });
    } catch (e) {
      setError(
        e instanceof Error ? `Request failed before reaching the API: ${e.message}` : "Request failed.",
      );
    } finally {
      setSending(false);
    }
  };

  return (
    <>
      <div className="rounded-xl border border-slate-200 p-4 dark:border-slate-800">
        <label htmlFor="console-endpoint" className="mb-2 block text-sm font-semibold text-slate-900 dark:text-white">
          Endpoint
        </label>
        <select
          id="console-endpoint"
          value={selectedId}
          onChange={(e) => onSelect(e.target.value)}
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
        >
          {GROUPS.map((g) => (
            <optgroup key={g.label} label={g.label}>
              {g.endpoints.map((e) => (
                <option key={e.id} value={e.id}>
                  {e.method} · {e.title}
                </option>
              ))}
            </optgroup>
          ))}
        </select>

        <div className="mt-3 flex flex-wrap items-center gap-2">
          <span className={`rounded px-2 py-0.5 font-mono text-xs font-bold ${METHOD_STYLE[endpoint.method]}`}>
            {endpoint.method}
          </span>
          <code className="break-all font-mono text-xs text-slate-700 dark:text-slate-300">{endpoint.path}</code>
          {endpoint.writes && (
            <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-900 dark:bg-amber-950/60 dark:text-amber-300">
              changes data
            </span>
          )}
        </div>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">{endpoint.desc}</p>

        {endpoint.params && endpoint.params.length > 0 && (
          <div className="mt-4 space-y-3">
            {endpoint.params.map((p) => (
              <div key={p.name}>
                <label htmlFor={`param-${p.name}`} className="mb-1 block text-xs font-medium text-slate-700 dark:text-slate-300">
                  {p.name}
                  <span className="ml-1.5 font-normal text-slate-400">
                    {p.where}
                    {p.required ? " · required" : ""}
                  </span>
                </label>
                <input
                  id={`param-${p.name}`}
                  value={paramValues[p.name] ?? ""}
                  onChange={(e) => setParamValues((v) => ({ ...v, [p.name]: e.target.value }))}
                  placeholder={p.placeholder}
                  className="w-full rounded-lg border border-slate-300 bg-white px-3 py-1.5 font-mono text-sm text-slate-900 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
                />
                {p.hint && <p className="mt-1 text-xs text-slate-400">{p.hint}</p>}
              </div>
            ))}
          </div>
        )}

        {endpoint.body !== undefined && (
          <div className="mt-4">
            <label htmlFor="console-body" className="mb-1 block text-xs font-medium text-slate-700 dark:text-slate-300">
              Request body
            </label>
            <textarea
              id="console-body"
              value={bodyText}
              onChange={(e) => setBodyText(e.target.value)}
              rows={(endpoint.body?.split("\n").length ?? 4) + 1}
              spellCheck={false}
              className="w-full resize-y rounded-lg border border-slate-300 bg-white px-3 py-2 font-mono text-[13px] text-slate-900 focus:border-blue-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
            />
          </div>
        )}

        <button
          type="button"
          onClick={send}
          disabled={sending}
          className="mt-4 w-full rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 disabled:opacity-60 motion-reduce:transition-none dark:focus-visible:ring-offset-slate-950"
        >
          {sending ? "Sending…" : `Send ${endpoint.method}`}
        </button>

        {error && (
          <p role="alert" className="mt-3 text-sm text-red-700 dark:text-red-400">
            {error}
          </p>
        )}
      </div>

      <div className="rounded-xl border border-slate-200 dark:border-slate-800">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-2.5 dark:border-slate-800">
          <span className="text-sm font-semibold text-slate-900 dark:text-white">Response</span>
          {result && (
            <span className="flex items-center gap-3 text-xs">
              <span
                className={`rounded px-2 py-0.5 font-mono font-bold ${
                  result.ok
                    ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300"
                    : "bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300"
                }`}
              >
                {result.status} {result.statusText}
              </span>
              <span className="tabular-nums text-slate-400">{result.ms} ms</span>
            </span>
          )}
        </div>
        <div className="max-h-[28rem] overflow-auto p-4">
          {result ? (
            <pre className="text-[13px] leading-relaxed">
              <code className="font-mono text-slate-800 dark:text-slate-200">{result.body}</code>
            </pre>
          ) : (
            <p className="py-8 text-center text-sm text-slate-400">
              {sending ? "Waiting for the API…" : "Send a request to see the response here."}
            </p>
          )}
        </div>
      </div>
    </>
  );
}
