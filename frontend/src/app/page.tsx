import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { brand } from "@/brand";

import { Hero } from "@/components/marketing/hero";
import { MarketingNav } from "@/components/marketing/nav";
import {
  Features,
  FinalCTA,
  Footer,
  HowItWorks,
  Pricing,
  StatusPreview,
} from "@/components/marketing/sections";
import { StructuredData } from "@/components/marketing/structured-data";
import type { LivePlans } from "@/lib/plans";

// Server-side, requests go to the API service inside the cluster (the browser base URL
// is deliberately empty for same-origin). `||`, not `??`: NEXT_PUBLIC_API_BASE_URL is
// the empty string in every deployment, which must fall through to the internal host.
const API_INTERNAL =
  process.env.BEACON_INTERNAL_API_URL ||
  process.env.NEXT_PUBLIC_API_BASE_URL ||
  "http://api:8080";

// Live, operator-tuned pricing for the cards and the schema.org Offers. Never throws:
// at build time (API unreachable) or on any error it returns null and the page renders
// the static PLANS fallback, correcting itself on the next revalidation once live.
async function fetchLivePlans(): Promise<LivePlans | null> {
  try {
    const res = await fetch(`${API_INTERNAL.replace(/\/$/, "")}/api/v1/public/plans`, {
      next: { revalidate: 60 },
    });
    if (!res.ok) return null;
    return (await res.json()) as LivePlans;
  } catch {
    return null;
  }
}

// Re-render at most once a minute so an operator's price change at /platform reaches
// the (statically served, SEO-friendly) landing page without a redeploy.
export const revalidate = 60;

// `/` used to redirect straight to /login, which meant the product had no public
// surface at all — nothing to point a campaign at, and nothing for a stranger to
// read. It is now the marketing page. Logged-in visitors are NOT bounced away:
// the nav swaps its CTA to "Go to dashboard", which is the least surprising
// behaviour and keeps the page shareable by people who are already customers.

const HERO_DESCRIPTION =
  "Self-hosted, multi-tenant infrastructure monitoring. Watch endpoints, certificates and DNS every 30 seconds, alert the right person, and publish a status page your customers trust.";

export const metadata: Metadata = {
  title: `${brand.name} — Know it's down before your customers do`,
  description: HERO_DESCRIPTION,
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    siteName: brand.name,
    url: "/",
    title: `${brand.name} — Infrastructure monitoring you own`,
    description: HERO_DESCRIPTION,
  },
  twitter: {
    card: "summary_large_image",
    title: `${brand.name} — Infrastructure monitoring you own`,
    description: HERO_DESCRIPTION,
  },
};

export default async function LandingPage() {
  const t = await getTranslations("marketing");
  const live = await fetchLivePlans();
  return (
    <div className="min-h-dvh bg-white text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      {/* Skip link: the nav is fixed, so keyboard users need a way past it. */}
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[60] focus:rounded-lg focus:bg-slate-900 focus:px-4 focus:py-2 focus:text-white dark:focus:bg-white dark:focus:text-slate-900"
      >
        {t("skipToContent")}
      </a>
      <StructuredData live={live} />
      <MarketingNav />
      <main id="main">
        <Hero />
        <Features />
        <HowItWorks />
        <Pricing live={live} />
        <StatusPreview />
        <FinalCTA />
      </main>
      <Footer />
    </div>
  );
}
