"use client";

import { useTranslations } from "next-intl";

import { useSystemInfo } from "@/lib/hooks";
import { useNow } from "@/lib/time";

/**
 * What is running here, and since when.
 *
 * It exists for promotion. Once the same commit moves int → test → acc → prod, the
 * only hard question is which of them you are actually looking at and whether it has
 * the change you just shipped — and answering that with kubectl, per environment, is
 * how people end up testing the old build and trusting the result.
 *
 * Every value is RUNTIME, read from the API rather than compiled into this bundle.
 * That is not incidental: a promoted image can only ever name the environment it was
 * BUILT for, so a baked-in "INT" would follow the artifact into prod and lie there.
 */

// The environment is the one thing here that must be unmissable, so it is the only
// thing with colour. Prod is red not because it is broken but because "am I on prod?"
// is the question worth answering before you touch anything.
const ENV_STYLE: Record<string, string> = {
  prod: "bg-red-100 text-red-800 dark:bg-red-950/60 dark:text-red-300",
  acc: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300",
  test: "bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300",
  int: "bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300",
};

export function BuildFooter() {
  const t = useTranslations("buildFooter");
  const { data } = useSystemInfo();
  // "2h ago" — coarse on purpose: nobody promoting a build cares about seconds.
  const ago = (ms: number): string => {
    const s = Math.max(0, Math.floor(ms / 1000));
    if (s < 60) return t("agoSeconds", { s });
    const m = Math.floor(s / 60);
    if (m < 60) return t("agoMinutes", { m });
    const h = Math.floor(m / 60);
    if (h < 48) return t("agoHours", { h });
    return t("agoDays", { d: Math.floor(h / 24) });
  };
  // Ticks itself, so a footer left open still reads true — and derived from the
  // shared clock rather than Date.now() in render, which would freeze at whatever
  // the compiler memoized.
  const now = useNow(30_000);

  // Nothing at all until it loads: a footer that flashes "unknown" on every page is
  // worse than one that arrives a moment late.
  if (!data) return null;

  const envKey = data.env?.toLowerCase() ?? "";
  const envStyle = ENV_STYLE[envKey] ?? ENV_STYLE.int;
  const started = Date.parse(data.started_at);
  const deployed = now !== null && Number.isFinite(started) ? ago(now - started) : null;

  return (
    <footer className="mt-10 border-t border-slate-200 px-1 py-4 dark:border-slate-800">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs text-slate-500 dark:text-slate-400">
        <span
          className={`rounded px-1.5 py-0.5 font-semibold uppercase tracking-wide ${envStyle}`}
          title={t("envTitle", { env: data.env })}
        >
          {data.env || t("unknown")}
        </span>

        <span className="font-mono" title={t("buildTitle", { version: data.version })}>
          {data.version}
        </span>

        {deployed && (
          <>
            <span aria-hidden className="text-slate-300 dark:text-slate-700">
              ·
            </span>
            {/* The exact instant lives in the tooltip. "Deployed" is honest but not
                exact — this is when the process started, which is the deploy unless a
                pod restarted on its own, and the tooltip says so rather than letting
                the short version quietly overstate itself. */}
            <span title={t("processStartedTitle", { time: new Date(started).toLocaleString() })}>
              {t("deployed", { ago: deployed })}
            </span>
          </>
        )}
      </div>
    </footer>
  );
}
