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
        brand: {
          50: "#eef7ff",
          100: "#d9edff",
          200: "#bce0ff",
          300: "#8ecdff",
          400: "#59b0ff",
          500: "#328cff",
          600: "#1b6ef5",
          700: "#1657e1",
          800: "#1847b6",
          900: "#1a3f8f",
        },
      },
    },
  },
  plugins: [],
};

export default config;
