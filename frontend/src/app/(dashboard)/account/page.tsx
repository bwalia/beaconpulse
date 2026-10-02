"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

import { Button, Card, Field, Input } from "@/components/ui";
import { api, ApiRequestError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// Account is the signed-in person's own page: who they are, how to change their
// password, and the permanent "delete my account" control that GDPR and the App
// Store both require to be self-serve.
export default function AccountPage() {
  const { user, logout } = useAuth();
  const router = useRouter();
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (!user) return null; // the dashboard layout redirects signed-out visitors
  const isOwner = user.role === "owner";
  const matches = confirm.trim().toLowerCase() === user.email.toLowerCase();

  const onDelete = async (e: FormEvent) => {
    e.preventDefault();
    if (!matches) return;
    setError(null);
    setBusy(true);
    try {
      await api.post("/api/v1/account/delete", { confirm_email: confirm.trim() });
      await logout();
      router.replace("/");
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "Something went wrong. Nothing was deleted — try again.");
      setBusy(false);
    }
  };

  return (
    <div className="max-w-3xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Account</h1>
        <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">Your profile, password and account controls.</p>
      </div>

      <Card>
        <h2 className="text-base font-semibold">Profile</h2>
        <dl className="mt-4 grid grid-cols-[8rem_1fr] gap-y-2 text-sm">
          <dt className="text-slate-500 dark:text-slate-400">Name</dt>
          <dd>{user.name}</dd>
          <dt className="text-slate-500 dark:text-slate-400">Email</dt>
          <dd>{user.email}</dd>
          <dt className="text-slate-500 dark:text-slate-400">Role</dt>
          <dd className="capitalize">{user.role}</dd>
          <dt className="text-slate-500 dark:text-slate-400">Member since</dt>
          <dd>{new Date(user.created_at).toLocaleDateString()}</dd>
        </dl>
      </Card>

      <Card>
        <h2 className="text-base font-semibold">Password</h2>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          We&apos;ll email you a secure link to choose a new password. Changing it signs you out on every other device.
        </p>
        <Link href="/forgot-password" className="mt-3 inline-block text-sm font-medium text-brand-700 hover:underline dark:text-brand-400">
          Reset password by email →
        </Link>
      </Card>

      <Card>
        <h2 className="text-base font-semibold">Legal</h2>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
          Read our <Link href="/terms" className="font-medium text-brand-700 underline dark:text-brand-400">Terms of Service</Link> and{" "}
          <Link href="/privacy" className="font-medium text-brand-700 underline dark:text-brand-400">Privacy Policy</Link>.
        </p>
      </Card>

      <section className="rounded-xl border border-red-300 bg-red-50/50 p-5 dark:border-red-900/60 dark:bg-red-950/20">
        <h2 className="text-base font-semibold text-red-800 dark:text-red-300">Delete account</h2>
        {isOwner ? (
          <form onSubmit={onDelete} className="mt-2 space-y-4">
            <p className="text-sm text-slate-700 dark:text-slate-300">
              This permanently deletes your organisation and everything in it — monitors, alert history, status page,
              notification channels, API keys and your login. Any active subscription is cancelled immediately. Past
              invoices stay available from Stripe. <strong>This can&apos;t be undone.</strong>
            </p>
            {error && (
              <p role="alert" className="rounded-lg bg-red-100 px-4 py-3 text-sm text-red-800 dark:bg-red-900/40 dark:text-red-200">
                {error}
              </p>
            )}
            <Field label={`Type ${user.email} to confirm`}>
              <Input autoComplete="off" spellCheck={false} value={confirm} onChange={(e) => setConfirm(e.target.value)} />
            </Field>
            <Button type="submit" variant="danger" disabled={!matches || busy}>
              {busy ? "Deleting…" : "Delete my account permanently"}
            </Button>
          </form>
        ) : (
          <p className="mt-2 text-sm text-slate-700 dark:text-slate-300">
            Only the organisation owner can delete this account. Ask your owner, or contact support.
          </p>
        )}
      </section>
    </div>
  );
}
