import type { Config } from "tailwindcss";
import defaultTheme from "tailwindcss/defaultTheme";

const config: Config = {
  darkMode: "class",
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      // Fira Sans (UI) + Fira Code (data/mono) — the "Dashboard Data" pairing.
      // Loaded via next/font in app/layout.tsx, which self-hosts the files at build
      // time and exposes them as CSS variables: no runtime request to Google, and
      // no layout shift. The system stack remains as the fallback.
      fontFamily: {
        sans: ["var(--font-sans)", ...defaultTheme.fontFamily.sans],
        mono: ["var(--font-mono)", ...defaultTheme.fontFamily.mono],
      },
      colors: {
        // The brand accent, pointed at CSS variables the root layout fills from the
        // active brand (src/brand). The `rgb(var(...) / <alpha-value>)` form is what
        // keeps opacity modifiers working — `bg-brand-500/50` still means half-opacity.
        // Change the brand, change every one of these at once, no rebuild of a single
        // class needed.
        brand: {
          50: "rgb(var(--brand-50) / <alpha-value>)",
          100: "rgb(var(--brand-100) / <alpha-value>)",
          200: "rgb(var(--brand-200) / <alpha-value>)",
          300: "rgb(var(--brand-300) / <alpha-value>)",
          400: "rgb(var(--brand-400) / <alpha-value>)",
          500: "rgb(var(--brand-500) / <alpha-value>)",
          600: "rgb(var(--brand-600) / <alpha-value>)",
          700: "rgb(var(--brand-700) / <alpha-value>)",
          800: "rgb(var(--brand-800) / <alpha-value>)",
          900: "rgb(var(--brand-900) / <alpha-value>)",
        },
      },
    },
  },
  plugins: [],
};

export default config;
