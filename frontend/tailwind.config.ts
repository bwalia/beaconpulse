import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
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
