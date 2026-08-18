"use client";

// API origin: empty in the gateway setup (same-origin), else the configured host.
// Mirrors the BASE_URL derivation in lib/api.ts.
const API_BASE = (process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080").replace(/\/$/, "");

// SSO is opt-in per deployment: the button self-hides unless the frontend is told
// the backend has OIDC configured. The provider label is cosmetic.
const OIDC_ENABLED = process.env.NEXT_PUBLIC_OIDC_ENABLED === "true";
const OIDC_PROVIDER = process.env.NEXT_PUBLIC_OIDC_PROVIDER || "OpsAPI";

// Shared look for the federated sign-in buttons: a full-width sibling of the
// primary CTA (h-12, rounded-lg, app font) so Google + OpsAPI + "Sign in" read as
// one consistent set. Kept in sync with google-button.tsx by copy — a single
// style string in two files, not an abstraction.
export const socialButtonClass =
  "flex h-12 w-full items-center justify-center gap-3 rounded-lg border border-slate-300 " +
  "bg-white px-6 text-base font-medium text-slate-800 transition-colors hover:bg-slate-50 " +
  "focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 " +
  "motion-reduce:transition-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 " +
  "dark:hover:bg-slate-700 dark:focus-visible:ring-offset-slate-950";

/** The OpsAPI mark — a small brand badge (rose aperture) matching the provider's logo. */
function OpsapiMark() {
  return (
    <span
      aria-hidden
      className="grid h-5 w-5 shrink-0 place-items-center rounded-md bg-gradient-to-br from-rose-500 to-rose-600 text-white shadow-sm"
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="7" stroke="currentColor" strokeWidth="2.5" />
        <circle cx="12" cy="12" r="1.75" fill="currentColor" />
      </svg>
    </span>
  );
}

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
    <button type="button" onClick={start} className={socialButtonClass}>
      <OpsapiMark />
      Continue with {OIDC_PROVIDER}
    </button>
  );
}
