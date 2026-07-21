"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useTransition } from "react";

import { GlobeIcon } from "@/components/icons";
import { LOCALES } from "@/i18n/config";
import { setLocale } from "@/i18n/actions";

/**
 * The language picker.
 *
 * A native <select> on purpose: it is one control, works with a keyboard and a screen
 * reader out of the box, and — for a list of sixteen languages including RTL scripts —
 * the OS renders each name correctly with no custom menu to get wrong. Each option shows
 * its name in its OWN language, because someone looking for their language scans for how
 * THEY write it, not the English label.
 *
 * Choosing persists the locale server-side and refreshes, so the whole tree re-renders
 * translated. useTransition keeps the control responsive while that round trip happens.
 */
export function LanguageSwitcher({ className }: { className?: string }) {
  const current = useLocale();
  const t = useTranslations("common");
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const onChange = (value: string) => {
    startTransition(async () => {
      await setLocale(value);
      // Re-fetch server components with the new cookie in effect.
      router.refresh();
    });
  };

  return (
    <label className={`relative inline-flex items-center ${className ?? ""}`}>
      <span className="sr-only">{t("language")}</span>
      <GlobeIcon className="pointer-events-none absolute left-2.5 h-4 w-4 text-slate-500 dark:text-slate-400" />
      <select
        value={current}
        onChange={(e) => onChange(e.target.value)}
        disabled={pending}
        aria-label={t("language")}
        className="appearance-none rounded-lg border border-slate-200 bg-white py-1.5 pl-8 pr-8 text-sm text-slate-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
      >
        {LOCALES.map((l) => (
          <option key={l.code} value={l.code}>
            {l.native}
          </option>
        ))}
      </select>
      {/* chevron */}
      <svg
        aria-hidden
        viewBox="0 0 24 24"
        className="pointer-events-none absolute right-2 h-4 w-4 text-slate-400"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="m6 9 6 6 6-6" />
      </svg>
    </label>
  );
}
