import type { Metadata } from "next";

import { DocsShell } from "@/components/docs/shell";

export const metadata: Metadata = {
  title: { default: "Docs — Beacon Pulse", template: "%s — Beacon Pulse Docs" },
  description:
    "How to monitor uptime, latency, SSL and DNS with Beacon Pulse: guides, the full API reference, and CI automation.",
};

export default function DocsLayout({ children }: { children: React.ReactNode }) {
  return <DocsShell>{children}</DocsShell>;
}
