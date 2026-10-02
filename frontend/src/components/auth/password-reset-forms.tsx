"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";

import { Button, Field, Input } from "@/components/ui";
import { api, ApiRequestError } from "@/lib/api";

const errorText = (err: unknown) =>
  err instanceof ApiRequestError ? err.message : "Something went wrong. Try again.";

// ForgotPasswordForm asks for an email and always ends on the same "check your inbox"
// message — the API answers identically for registered and unknown addresses.
export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.post("/api/v1/auth/forgot-password", { email: email.trim() }, false);
      setSent(true);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  if (sent) {
    return (
      <p role="status" className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200">
        If an account exists for <strong>{email.trim()}</strong>, we&apos;ve emailed a link to reset its password. It
        expires in 1 hour — check your spam folder if it doesn&apos;t arrive in a few minutes.
      </p>
    );
  }
  return (
    <form onSubmit={onSubmit} className="space-y-4">
      {error && (
        <p role="alert" className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300">
          {error}
        </p>
      )}
      <Field label="Email">
        <Input
          type="email"
          inputMode="email"
          autoComplete="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="you@company.com"
        />
      </Field>
      <Button type="submit" className="w-full" disabled={busy}>
        {busy ? "Sending…" : "Email me a reset link"}
      </Button>
    </form>
  );
}

// ResetPasswordForm sets a new password from the emailed token. On success every
// existing session is signed out server-side, so the user signs in fresh.
export function ResetPasswordForm({ token }: { token: string }) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (!token) {
    return (
      <p role="alert" className="text-sm text-slate-700 dark:text-slate-300">
        This reset link is incomplete. <Link href="/forgot-password" className="font-medium text-brand-700 underline dark:text-brand-400">Request a new one</Link>.
      </p>
    );
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password.length < 8) return setError("Use at least 8 characters.");
    if (password !== confirm) return setError("The two passwords don't match.");
    setBusy(true);
    try {
      await api.post("/api/v1/auth/reset-password", { token, password }, false);
      setDone(true);
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  if (done) {
    return (
      <div role="status" className="space-y-4">
        <p className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200">
          Your password has been changed and you&apos;ve been signed out everywhere else.
        </p>
        <Link
          href="/login"
          className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-brand-600 px-4 text-sm font-medium text-white hover:bg-brand-700"
        >
          Sign in
        </Link>
      </div>
    );
  }
  return (
    <form onSubmit={onSubmit} className="space-y-4">
      {error && (
        <p role="alert" className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-300">
          {error}{" "}
          {error.includes("expired") && (
            <Link href="/forgot-password" className="font-medium underline">
              Request a new link
            </Link>
          )}
        </p>
      )}
      <Field label="New password" hint="At least 8 characters.">
        <Input type="password" autoComplete="new-password" required minLength={8} value={password} onChange={(e) => setPassword(e.target.value)} />
      </Field>
      <Field label="Confirm new password">
        <Input type="password" autoComplete="new-password" required minLength={8} value={confirm} onChange={(e) => setConfirm(e.target.value)} />
      </Field>
      <Button type="submit" className="w-full" disabled={busy}>
        {busy ? "Saving…" : "Set new password"}
      </Button>
    </form>
  );
}
