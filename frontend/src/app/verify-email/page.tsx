import type { Metadata } from "next";
import Link from "next/link";

import { AuthCard } from "@/components/auth/auth-card";
import { apiInternal } from "@/lib/site";

export const metadata: Metadata = {
  title: "Confirm your email",
  robots: { index: false, follow: false },
  // The token is in this page's URL; never send it onward in a Referer header.
  referrer: "no-referrer",
};

// Confirms the address server-side while rendering, so the link works in any
// browser — signed in or not — with no client round trip. Verifying is idempotent,
// so a mail scanner that opens the link first does no harm.
async function confirm(token: string): Promise<{ ok: boolean; message?: string }> {
  if (!token) return { ok: false, message: "This confirmation link is incomplete." };
  try {
    const res = await fetch(`${apiInternal}/api/v1/auth/verify-email`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
      cache: "no-store",
    });
    if (res.ok) return { ok: true };
    const body = (await res.json().catch(() => ({}))) as { error?: { message?: string } };
    return { ok: false, message: body.error?.message };
  } catch {
    return { ok: false, message: "We couldn't reach the server. Try the link again in a moment." };
  }
}

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string | string[] }>;
}) {
  const { token } = await searchParams;
  const result = await confirm(typeof token === "string" ? token : "");

  return result.ok ? (
    <AuthCard title="Email confirmed" subtitle="Thanks — alerts and account emails will reach you here.">
      <Link
        href="/dashboard"
        className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-brand-600 px-4 text-sm font-medium text-white hover:bg-brand-700"
      >
        Go to your dashboard
      </Link>
    </AuthCard>
  ) : (
    <AuthCard title="Couldn't confirm your email" subtitle={result.message || "This confirmation link is invalid or has expired."}>
      <p className="text-sm text-slate-600 dark:text-slate-300">
        Sign in and use <strong>Resend email</strong> on the banner at the top of your dashboard to get a new link.
      </p>
    </AuthCard>
  );
}
