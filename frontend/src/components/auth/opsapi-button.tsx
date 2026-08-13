"use client";

// API origin: empty in the gateway setup (same-origin), else the configured host.
// Mirrors the BASE_URL derivation in lib/api.ts.
const API_BASE = (process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

// SSO is opt-in per deployment: the button self-hides unless the frontend is told
// the backend has OIDC configured. The provider label is cosmetic.
const OIDC_ENABLED = process.env.NEXT_PUBLIC_OIDC_ENABLED === "true";
const OIDC_PROVIDER = process.env.NEXT_PUBLIC_OIDC_PROVIDER || "OpsAPI";

/**
 * "Continue with <provider>" — starts the server-side OIDC Authorization-Code
 * flow. It is a plain full-page navigation to the backend's /auth/oidc/start,
 * which mints state + PKCE and redirects to the provider. Nothing to verify in
 * the browser, so no SDK and no dependency — the inverse of the Google button,
 * which runs entirely client-side.
 *
 * Renders nothing unless NEXT_PUBLIC_OIDC_ENABLED is "true", matching the Google
 * button's opt-in/self-hide behaviour.
 */
export function OpsapiButton() {
  if (!OIDC_ENABLED) return null;

  const start = () => {
    window.location.href = `${API_BASE}/api/v1/auth/oidc/start`;
  };

  return (
    <button
      type="button"
      onClick={start}
      className="flex w-full items-center justify-center gap-2.5 rounded-full border border-slate-300 bg-white px-6 py-2.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path
          d="M12 2 4 6v6c0 5 3.4 8.3 8 10 4.6-1.7 8-5 8-10V6l-8-4Z"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinejoin="round"
        />
        <path d="m9 12 2 2 4-4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      Continue with {OIDC_PROVIDER}
    </button>
  );
}
