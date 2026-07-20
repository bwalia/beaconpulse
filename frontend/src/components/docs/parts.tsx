"use client";

import { useState } from "react";

import { AlertTriangleIcon, CheckIcon, LockIcon } from "@/components/icons";

/**
 * The pieces documentation is made of.
 *
 * Reading docs is mostly copying code out of them, so the code block is the component
 * that matters: it copies in one click, says what it copied, and never hides the end of
 * a long line behind an invisible overflow.
 */

/** Copy-to-clipboard code block. */
export function Code({
  children,
  lang,
  title,
}: {
  children: string;
  lang?: string;
  title?: string;
}) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(children.trim());
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch {
      /* clipboard blocked — the text is still selectable */
    }
  };

  return (
    <figure className="group relative my-5 overflow-hidden rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/70">
      {(title || lang) && (
        <figcaption className="flex items-center justify-between border-b border-slate-200 px-4 py-2 text-xs dark:border-slate-800">
          <span className="font-mono text-slate-500 dark:text-slate-400">{title ?? lang}</span>
        </figcaption>
      )}
      <button
        type="button"
        onClick={copy}
        aria-label={copied ? "Copied" : "Copy to clipboard"}
        className="absolute right-2 top-2 z-10 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs font-medium text-slate-600 opacity-0 transition-opacity focus:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 group-hover:opacity-100 motion-reduce:transition-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
        style={{ top: title || lang ? "2.6rem" : "0.5rem" }}
      >
        {copied ? (
          <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
            <CheckIcon className="h-3 w-3" /> Copied
          </span>
        ) : (
          "Copy"
        )}
      </button>
      {/* Scrolls inside itself: a long curl line must never make the page scroll
          sideways, which on a phone makes the whole document unreadable. */}
      <pre className="overflow-x-auto px-4 py-3.5 text-[13px] leading-relaxed">
        <code className="font-mono text-slate-800 dark:text-slate-200">{children.trim()}</code>
      </pre>
    </figure>
  );
}

/** Inline code. */
export function C({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[0.9em] text-slate-800 dark:bg-slate-800 dark:text-slate-200">
      {children}
    </code>
  );
}

type NoteKind = "note" | "warn" | "security";

const NOTE: Record<NoteKind, { className: string; Icon: (p: { className?: string }) => React.ReactElement; label: string }> = {
  note: {
    className: "border-blue-500/40 bg-blue-500/[0.06] text-blue-900 dark:text-blue-200",
    Icon: CheckIcon,
    label: "Note",
  },
  warn: {
    className: "border-amber-500/40 bg-amber-500/[0.06] text-amber-900 dark:text-amber-200",
    Icon: AlertTriangleIcon,
    label: "Careful",
  },
  security: {
    className: "border-red-500/40 bg-red-500/[0.06] text-red-900 dark:text-red-200",
    Icon: LockIcon,
    label: "Security",
  },
};

/** A callout. Used sparingly — a page of highlights highlights nothing. */
export function Note({ kind = "note", children }: { kind?: NoteKind; children: React.ReactNode }) {
  const n = NOTE[kind];
  return (
    <div className={`my-5 flex gap-3 rounded-xl border px-4 py-3.5 text-sm ${n.className}`}>
      <n.Icon className="mt-0.5 h-4 w-4 shrink-0" />
      <div className="min-w-0 [&>p:first-child]:mt-0 [&>p:last-child]:mb-0">{children}</div>
    </div>
  );
}

const METHOD: Record<string, string> = {
  GET: "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300",
  POST: "bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300",
  PATCH: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  PUT: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  DELETE: "bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300",
};

/** One endpoint: method, path, and what it does. */
export function Endpoint({
  method,
  path,
  children,
  auth,
}: {
  method: keyof typeof METHOD | string;
  path: string;
  children?: React.ReactNode;
  /** Omit for the normal case (any credential). "session" marks the few that refuse API keys. */
  auth?: "session" | "public";
}) {
  return (
    <div className="border-b border-slate-200 py-3 last:border-0 dark:border-slate-800">
      <div className="flex flex-wrap items-center gap-2">
        <span className={`rounded px-2 py-0.5 font-mono text-xs font-bold ${METHOD[method] ?? METHOD.GET}`}>
          {method}
        </span>
        <span className="break-all font-mono text-sm text-slate-800 dark:text-slate-200">{path}</span>
        {auth === "session" && (
          <span
            title="API keys cannot call this — sign-in required"
            className="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300"
          >
            session only
          </span>
        )}
        {auth === "public" && (
          <span className="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
            no auth
          </span>
        )}
      </div>
      {children && <div className="mt-1.5 text-sm text-slate-600 dark:text-slate-300">{children}</div>}
    </div>
  );
}

/** A field reference table. Scrolls inside itself on narrow screens. */
export function Fields({ rows }: { rows: { name: string; type?: string; required?: boolean; desc: React.ReactNode }[] }) {
  return (
    <div className="my-5 overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-800">
      <table className="w-full min-w-[36rem] text-left text-sm">
        <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
          <tr>
            <th className="px-4 py-2.5 font-semibold">Field</th>
            <th className="px-4 py-2.5 font-semibold">Type</th>
            <th className="px-4 py-2.5 font-semibold">Notes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-b border-slate-100 last:border-0 dark:border-slate-800/60">
              <td className="whitespace-nowrap px-4 py-3 align-top">
                <span className="font-mono text-slate-900 dark:text-slate-100">{r.name}</span>
                {r.required && (
                  <span className="ml-1.5 text-[11px] font-semibold uppercase text-red-600 dark:text-red-400">
                    required
                  </span>
                )}
              </td>
              <td className="whitespace-nowrap px-4 py-3 align-top font-mono text-xs text-slate-500 dark:text-slate-400">
                {r.type ?? "string"}
              </td>
              <td className="px-4 py-3 align-top text-slate-600 dark:text-slate-300">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** A linkable section heading. */
export function H2({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h2
      id={id}
      className="group mt-12 scroll-mt-24 text-2xl font-semibold tracking-tight text-slate-900 first:mt-0 dark:text-white"
    >
      <a href={`#${id}`} className="no-underline">
        {children}
        <span
          aria-hidden
          className="ml-2 text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 motion-reduce:transition-none dark:text-slate-600"
        >
          #
        </span>
      </a>
    </h2>
  );
}

export function H3({ id, children }: { id?: string; children: React.ReactNode }) {
  return (
    <h3 id={id} className="mt-8 scroll-mt-24 text-lg font-semibold text-slate-900 dark:text-white">
      {children}
    </h3>
  );
}
