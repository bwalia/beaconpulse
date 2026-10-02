import Link from "next/link";
import type { ReactNode } from "react";

import { brand } from "@/brand";
import { BeaconMark } from "@/components/icons";

// AuthCard is the small centred shell for the single-purpose account pages (forgot
// and reset password): brand home link, a heading, the form, and a way back to sign in.
export function AuthCard({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <main className="flex min-h-dvh items-center justify-center bg-slate-50 px-6 py-12 dark:bg-slate-950">
      <div className="w-full max-w-md">
        <Link href="/" className="mb-8 inline-flex items-center gap-2.5">
          <BeaconMark className="h-8 w-8 text-brand-600 dark:text-brand-400" />
          <span className="text-xl font-semibold tracking-tight text-slate-900 dark:text-white">{brand.name}</span>
        </Link>
        <div className="rounded-2xl border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h1 className="text-2xl font-semibold tracking-tight text-slate-900 dark:text-white">{title}</h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">{subtitle}</p>
          <div className="mt-6">{children}</div>
        </div>
        <p className="mt-6 text-center text-sm text-slate-600 dark:text-slate-400">
          <Link href="/login" className="font-medium text-brand-700 hover:underline dark:text-brand-400">
            Back to sign in
          </Link>
        </p>
      </div>
    </main>
  );
}
