"use client";

// Shared, server-side list controls used across every paginated page (monitors,
// projects, maintenance, notifications, alerts, status-page domains) so the
// "Showing X–Y of N" footer and the search box look and behave identically.

import { Button } from "@/components/ui";
import { ArrowRightIcon, SearchIcon } from "@/components/icons";

export function SearchInput({
  value,
  onChange,
  placeholder = "Search…",
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  label: string;
}) {
  return (
    <div className="relative flex-1">
      <SearchIcon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={label}
        className="w-full rounded-lg border border-slate-300 bg-white py-2.5 pl-9 pr-3 text-[15px] text-slate-900 placeholder:text-slate-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:placeholder:text-slate-500"
      />
    </div>
  );
}

// Pagination renders the count summary and Prev/Next. `page` is zero-based; the
// caller owns the state and the server fetch, so this is purely presentational.
export function Pagination({
  page,
  pageSize,
  total,
  unit = "items",
  busy,
  onPageChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  unit?: string;
  busy?: boolean;
  onPageChange: (page: number) => void;
}) {
  if (total === 0) return null;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const from = page * pageSize + 1;
  const to = Math.min(total, (page + 1) * pageSize);
  return (
    <div className="flex flex-col items-center justify-between gap-3 sm:flex-row">
      <p className="text-sm text-slate-600 dark:text-slate-400">
        Showing{" "}
        <span className="font-medium tabular-nums text-slate-900 dark:text-slate-200">
          {from}–{to}
        </span>{" "}
        of <span className="font-medium tabular-nums text-slate-900 dark:text-slate-200">{total}</span> {unit}
      </p>
      <div className="flex items-center gap-2">
        <Button
          variant="secondary"
          size="sm"
          disabled={page === 0 || busy}
          onClick={() => onPageChange(page - 1)}
        >
          <ArrowRightIcon className="h-4 w-4 rotate-180" />
          Previous
        </Button>
        <span className="px-1 text-sm tabular-nums text-slate-500 dark:text-slate-400">
          {page + 1} / {pageCount}
        </span>
        <Button
          variant="secondary"
          size="sm"
          disabled={page + 1 >= pageCount || busy}
          onClick={() => onPageChange(page + 1)}
        >
          Next
          <ArrowRightIcon className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
