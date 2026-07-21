import type { Metadata } from "next";
import { brand } from "@/brand";

import { DocsShell } from "@/components/docs/shell";

export const metadata: Metadata = {
  title: { default: `Docs — ${brand.name}`, template: `%s — ${brand.name} Docs` },
  description: `How to monitor uptime, latency, SSL and DNS with ${brand.name}: guides, the full API reference, and CI automation.`,
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return <DocsShell>{children}</DocsShell>;
}
