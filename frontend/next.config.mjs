import { fileURLToPath } from "node:url";
import createNextIntlPlugin from "next-intl/plugin";

// Points next-intl at the request config that resolves the locale (from a cookie) and
// loads its messages. Wrapping the export is all the plugin needs.
const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Don't advertise the framework in every response.
  poweredByHeader: false,
  // Baseline security headers. HSTS is set at the TLS edge (wslproxy). Framing is
  // denied everywhere except public status pages, which customers may embed.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=()" },
        ],
      },
      {
        source: "/:path((?!status/).*)",
        headers: [{ key: "X-Frame-Options", value: "DENY" }],
      },
    ];
  },
  // Standalone output produces a minimal self-contained server for the Docker image.
  output: "standalone",
  // The React Compiler memoizes components and values automatically, which is why
  // there is almost no hand-written memo/useCallback/useMemo in here: hand-memoizing
  // is easy to get subtly wrong (a stale dep freezes part of the UI — the one failure
  // a monitoring product cannot afford) and it rots as the code changes. The compiler
  // re-derives it from scratch every build instead.
  //
  // It is only sound because render is pure. It memoizes on the inputs it can SEE, so
  // anything reading a hidden one — Date.now() during render, most of all — risks
  // being frozen at its first value. Those reads go through the clock store in
  // lib/time.ts, and the react-hooks lint rules (purity, immutability,
  // set-state-in-effect) are what keep it true. Keep them passing.
  reactCompiler: true,

  // Shared ISR/data cache (see cache-handler.mjs): a Redis-backed handler so the OpsAPI
  // webhook's revalidateTag("cms-posts") busts every replica, not just one pod. Active
  // only in production; local-only when BEACON_ISR_REDIS_URL is unset.
  // cacheMaxMemorySize: 0 defers entirely to the handler (no extra in-memory tier).
  cacheHandler:
    process.env.NODE_ENV === "production"
      ? fileURLToPath(new URL("./cache-handler.mjs", import.meta.url))
      : undefined,
  cacheMaxMemorySize: 0,
};

export default withNextIntl(nextConfig);
