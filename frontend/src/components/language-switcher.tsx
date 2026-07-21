"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useTransition } from "react";

import { GlobeIcon } from "@/components/icons";
import { LOCALE_COOKIE, LOCALES, resolveLocale } from "@/i18n/config";

const ONE_YEAR = 60 * 60 * 24 * 365;

/**
 * The language picker.
 *
 * A native <select> on purpose: it is one control, works with a keyboard and a screen
 * reader out of the box, and — for a list of sixteen languages including RTL scripts —
 * the OS renders each name correctly with no custom menu to get wrong. Each option shows
 * its name in its OWN language, because someone looking for their language scans for how
 * THEY write it, not the English label.
 *
 * The cookie is set CLIENT-SIDE rather than through a server action, deliberately. A
 * server action is a POST that Next.js guards with an origin check and signs with a
 * build-time key — behind a reverse proxy, or across more than one frontend replica,
 * that POST can fail and take down the whole page (which is exactly what a language
 * switch was doing). A locale is a preference, not a credential, so there is nothing to
 * protect by routing it through the server: write the cookie here, then refresh so the
 * server components re-render reading it. No POST, no origin check, no signing key —
 * nothing that a proxy can break.
 */
export function LanguageSwitcher({ className }: { className?: string }) {
  const current = useLocale();
  const t = useTranslations("common");
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  const onChange = (value: string) => {
    const locale = resolveLocale(value);
    // Lax + a year: a language choice should survive the session and follow normal
    // top-level navigation, and it is deliberately NOT HttpOnly so the client can set it.
    document.cookie = `${LOCALE_COOKIE}=${locale}; path=/; max-age=${ONE_YEAR}; samesite=lax`;
    startTransition(() => {
      // Re-fetch server components with the new cookie now in place.
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
