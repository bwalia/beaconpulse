import type { Metadata } from "next";

import { Hero } from "@/components/marketing/hero";
import { MarketingNav } from "@/components/marketing/nav";
import {
  Features,
  FinalCTA,
  Footer,
  HowItWorks,
  StatusPreview,
} from "@/components/marketing/sections";

// `/` used to redirect straight to /login, which meant the product had no public
// surface at all — nothing to point a campaign at, and nothing for a stranger to
// read. It is now the marketing page. Logged-in visitors are NOT bounced away:
// the nav swaps its CTA to "Go to dashboard", which is the least surprising
// behaviour and keeps the page shareable by people who are already customers.

export const metadata: Metadata = {
  title: "Beacon — Know it's down before your customers do",
  description:
    "Self-hosted, multi-tenant infrastructure monitoring. Watch endpoints, certificates and DNS every 30 seconds, alert the right person, and publish a status page your customers trust.",
  openGraph: {
    title: "Beacon — Infrastructure monitoring you own",
    description:
      "Uptime, latency, SSL and DNS monitoring with alerting and public status pages. Self-hosted and multi-tenant.",
    type: "website",
  },
};

export default function LandingPage() {
  return (
    <div className="min-h-dvh bg-white text-slate-900 dark:bg-slate-950 dark:text-slate-100">
      {/* Skip link: the nav is fixed, so keyboard users need a way past it. */}
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[60] focus:rounded-lg focus:bg-slate-900 focus:px-4 focus:py-2 focus:text-white dark:focus:bg-white dark:focus:text-slate-900"
      >
        Skip to content
      </a>
      <MarketingNav />
      <main id="main">
        <Hero />
        <Features />
        <HowItWorks />
        <StatusPreview />
        <FinalCTA />
      </main>
      <Footer />
    </div>
  );
}
