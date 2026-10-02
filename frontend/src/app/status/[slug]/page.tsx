import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { StatusView } from "@/components/status/status-view";
import { apiInternal } from "@/lib/site";
import type { PublicStatusPage } from "@/lib/types";

// The public status page.
//
// Server-rendered on purpose. A status page is read at the exact moment your
// infrastructure is on fire and someone is frantically refreshing it, so it must
// paint on the first byte rather than after a client fetch waterfall. It is also
// the one page you want indexable and linkable.
//
// It deliberately does NOT use the browser api client: that client attaches auth
// tokens and refresh logic, none of which belongs on an anonymous page. This
// fetches the public endpoint directly, server-side.

// Server-side, requests go to the API service inside the cluster (apiInternal); the
// public gateway URL is only meaningful to a browser.

// Match the endpoint's own Cache-Control (30s). Longer would risk showing a
// cheerful "operational" during a live outage — the one failure a status page
// cannot afford.
export const revalidate = 30;

async function fetchStatus(slug: string): Promise<PublicStatusPage | null> {
  const res = await fetch(
    `${apiInternal}/api/v1/public/status/${encodeURIComponent(slug)}`,
    { next: { revalidate: 30 } },
  );
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`status page fetch failed: ${res.status}`);
  return (await res.json()) as PublicStatusPage;
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  let page: PublicStatusPage | null = null;
  try {
    page = await fetchStatus(slug);
  } catch {
    // A metadata failure must never take the page down with it.
  }
  if (!page) return { title: "Status — not found" };

  return {
    title: `${page.title} — Status`,
    description: `Live operational status for ${page.org_name}.`,
    openGraph: {
      title: `${page.title} — Status`,
      description: `Live operational status for ${page.org_name}.`,
    },
  };
}

export default async function StatusPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const page = await fetchStatus(slug);

  // An unpublished org and an unknown slug both 404, exactly as the API does —
  // the UI must not become the oracle the API refused to be.
  if (!page) notFound();

  return <StatusView page={page} />;
}
