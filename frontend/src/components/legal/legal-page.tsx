import type { ReactNode } from "react";

import { MarketingNav } from "@/components/marketing/nav";
import { Footer } from "@/components/marketing/sections";
import { LEGAL_UPDATED } from "@/lib/legal";

// LegalPage is the shell for /terms and /privacy: the site's own nav and footer (so
// the pages are reachable and link onward like any other public page) around a
// readable, single-column document. Prose styling lives here as descendant variants,
// so the page files stay plain semantic HTML.
export function LegalPage({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="min-h-dvh bg-white text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      <MarketingNav />
      <main className="mx-auto w-full max-w-3xl px-6 pb-20 pt-28 sm:pt-32">
        <h1 className="text-4xl font-semibold tracking-tight">{title}</h1>
        <p className="mt-3 text-sm text-slate-500 dark:text-slate-400">Last updated {LEGAL_UPDATED}</p>
        <article className="mt-10 text-base leading-7 text-slate-700 dark:text-slate-300 [&_a]:font-medium [&_a]:text-brand-700 [&_a]:underline [&_a]:underline-offset-2 dark:[&_a]:text-brand-400 [&_h2]:mt-12 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:text-slate-900 dark:[&_h2]:text-white [&_li]:mt-2 [&_p]:mt-4 [&_strong]:font-semibold [&_strong]:text-slate-900 dark:[&_strong]:text-white [&_ul]:mt-4 [&_ul]:list-disc [&_ul]:pl-6">
          {children}
        </article>
      </main>
      <Footer />
    </div>
  );
}
