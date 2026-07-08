"use client";

import { useEffect, useState } from "react";
import { DisplayIcon, MoonIcon, SunIcon } from "@/components/icons";

export type Theme = "light" | "dark" | "system";

/** Must match the key used by the no-FOUC script in app/layout.tsx. */
export const THEME_KEY = "beacon-theme";

const prefersDark = () => window.matchMedia("(prefers-color-scheme: dark)").matches;

/** Tailwind is configured with `darkMode: "class"`, so the class on <html> is the
 *  single source of truth. `color-scheme` is set alongside it so native controls,
 *  scrollbars and form widgets match the page instead of staying light. */
function applyTheme(theme: Theme) {
  const dark = theme === "dark" || (theme === "system" && prefersDark());
  const root = document.documentElement;
  root.classList.toggle("dark", dark);
  root.style.colorScheme = dark ? "dark" : "light";
}

function readTheme(): Theme {
  if (typeof window === "undefined") return "system";
  const raw = window.localStorage.getItem(THEME_KEY);
  return raw === "light" || raw === "dark" ? raw : "system";
}

export function useTheme(): [Theme, (t: Theme) => void] {
  // Start at "system" so server and first client render agree; the inline script
  // has already painted the correct colours, so there is no flash to correct.
  const [theme, setThemeState] = useState<Theme>("system");

  useEffect(() => {
    setThemeState(readTheme());
  }, []);

  // Follow the OS while (and only while) the user has chosen "system".
  useEffect(() => {
    if (theme !== "system") return;
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system");
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [theme]);

  const setTheme = (next: Theme) => {
    setThemeState(next);
    if (next === "system") window.localStorage.removeItem(THEME_KEY);
    else window.localStorage.setItem(THEME_KEY, next);
    applyTheme(next);
  };

  return [theme, setTheme];
}

const OPTIONS: { value: Theme; label: string; Icon: (p: { className?: string }) => React.ReactElement }[] = [
  { value: "light", label: "Light", Icon: SunIcon },
  { value: "dark", label: "Dark", Icon: MoonIcon },
  { value: "system", label: "System", Icon: DisplayIcon },
];

export function ThemeToggle() {
  const [theme, setTheme] = useTheme();
  return (
    <div
      role="group"
      aria-label="Colour theme"
      className="inline-flex items-center rounded-lg border border-slate-200 bg-white p-0.5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
    >
      {OPTIONS.map(({ value, label, Icon }) => {
        const active = theme === value;
        return (
          <button
            key={value}
            type="button"
            onClick={() => setTheme(value)}
            aria-pressed={active}
            aria-label={label}
            title={label}
            className={`grid h-7 w-8 place-items-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 motion-reduce:transition-none ${
              active
                ? "bg-brand-600 text-white shadow-sm"
                : "text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-100"
            }`}
          >
            <Icon className="h-4 w-4" />
          </button>
        );
      })}
    </div>
  );
}
