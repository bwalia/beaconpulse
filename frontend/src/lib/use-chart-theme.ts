"use client";

import { useEffect, useState } from "react";
import { CHART_SURFACE, type ChartSurface } from "./viz";

/**
 * Resolves the chart chrome (grid, axis, tooltip) for the active theme.
 *
 * Tailwind is configured with `darkMode: "class"`, so the `dark` class on <html>
 * is the single source of truth. Reading `prefers-color-scheme` here instead
 * would desync the SVG chrome from the surrounding page whenever the two
 * disagree. Starts on the light tokens so SSR and first paint match, then syncs.
 */
export function useChartTheme(): ChartSurface {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    const root = document.documentElement;
    const sync = () => setDark(root.classList.contains("dark"));
    sync();
    const observer = new MutationObserver(sync);
    observer.observe(root, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return dark ? CHART_SURFACE.dark : CHART_SURFACE.light;
}
