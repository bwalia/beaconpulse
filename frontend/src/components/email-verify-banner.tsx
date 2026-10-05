"use client";

import { useState } from "react";

import { api, ApiRequestError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// EmailVerifyBanner asks a password sign-up to confirm their address, on every
// dashboard page until they do. It gates nothing: it exists so resets and alerts
// are known to reach a real inbox. Renders nothing once the address is confirmed.
export function EmailVerifyBanner() {
  const { user } = useAuth();
  const [state, setState] = useState<"idle" | "sending" | "sent" | "verified">("idle");
  const [error, setError] = useState<string | null>(null);

  if (!user || user.email_verified !== false || state === "verified") return null;

  const resend = async () => {
    setError(null);
    setState("sending");
    try {
      const res = await api.post<{ status: string }>("/api/v1/me/verify-email/resend", {});
      setState(res.status === "already_verified" ? "verified" : "sent");
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "Couldn't send the email. Try again shortly.");
      setState("idle");
    }
  };

  return (
    <div
      role="status"
      className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-100"
    >
      <span>
        <strong className="font-semibold">Confirm your email.</strong>{" "}
        {state === "sent"
          ? `We've sent a new link to ${user.email}.`
          : `We sent a link to ${user.email} so alerts and password resets reach you.`}
        {error && <span className="ml-1 text-red-700 dark:text-red-300">{error}</span>}
      </span>
      {state !== "sent" && (
        <button
          type="button"
          onClick={resend}
          disabled={state === "sending"}
          className="shrink-0 rounded-md border border-amber-400 px-3 py-1.5 font-medium hover:bg-amber-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-500 disabled:opacity-60 dark:border-amber-700 dark:hover:bg-amber-900/40"
        >
          {state === "sending" ? "Sending…" : "Resend email"}
        </button>
      )}
    </div>
  );
}
