"use client";

import { useEffect, useState } from "react";

import { tokenStore } from "@/lib/api";

// The backend OIDC callback delivers the session in the URL FRAGMENT
// (#access_token=…&refresh_token=…&expires_in=…) so the tokens never reach a
// server log or Referer header. On error it sends #error=<code> instead.
const ERROR_MESSAGES: Record<string, string> = {
  invalid_state: "Your sign-in link expired. Please try again.",
  invalid_request: "The sign-in response was incomplete. Please try again.",
  exchange_failed: "We couldn't complete sign-in with the provider. Please try again.",
  userinfo_failed: "We couldn't read your profile from the provider. Please try again.",
  login_failed: "We couldn't sign you in. Your account may be inactive.",
  access_denied: "Sign-in was cancelled.",
};

/**
 * OIDC landing page. Adopts the session from the URL fragment, then hard-navigates
 * to the dashboard so the AuthProvider re-hydrates the user from /me on mount —
 * the same end state as a password or Google login.
 */
export default function OidcCallbackPage() {
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Wrapped in an inner function so the failure-path setState is never a direct
    // synchronous call in the effect body (avoids cascading-render lint/runtime).
    const run = () => {
      const frag = new URLSearchParams(window.location.hash.replace(/^#/, ""));

      const err = frag.get("error");
      if (err) {
        setError(ERROR_MESSAGES[err] ?? "Sign-in failed. Please try again.");
        return;
      }

      const access = frag.get("access_token");
      const refresh = frag.get("refresh_token");
      if (!access || !refresh) {
        setError("Sign-in was incomplete. Please try again.");
        return;
      }

      tokenStore.set(access, refresh);
      // Full navigation (not the client router) so AuthProvider remounts and
      // fetches /me; also drops the fragment from the address bar.
      window.location.replace("/dashboard");
    };
    run();
  }, []);

  return (
    <div className="grid min-h-dvh place-items-center bg-slate-50 p-6 dark:bg-slate-950">
      {error ? (
        <div className="max-w-sm text-center">
          <p role="alert" className="text-base font-medium text-red-600 dark:text-red-400">
            {error}
          </p>
          <a
            href="/login"
            className="mt-6 inline-block rounded-full border border-slate-300 px-6 py-2.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            Back to sign in
          </a>
        </div>
      ) : (
        <div className="flex items-center gap-3 text-slate-600 dark:text-slate-300">
          <span className="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600" />
          Signing you in…
        </div>
      )}
    </div>
  );
}
