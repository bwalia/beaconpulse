import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, H2, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Status pages",
  description: "A public page your customers can check instead of emailing you.",
};

export default function StatusPages() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        Status pages
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        A page anyone can load to see whether your service is working — so during an
        incident your inbox is not the status page.
      </p>

      <H2 id="publishing">Nothing is public until you say so</H2>
      <p>
        Two switches, both off by default. The page itself must be published, and{" "}
        <em>each monitor</em> must be published individually.
      </p>
      <p>
        That is deliberate. Publishing is a security decision — a status page lists the
        hostnames you monitor, which is a map of your infrastructure — so there is no
        &ldquo;publish everything&rdquo; button to click by accident.
      </p>
      <Note kind="warn">
        <p>
          A live page with no monitors on it is worse than no page: visitors read a blank
          page as &ldquo;fine&rdquo;. The dashboard warns you if you publish a page and
          leave it empty.
        </p>
      </Note>

      <H2 id="address">Your address</H2>
      <p>
        The page lives at <C>/status/your-slug</C>. Pick the slug yourself under{" "}
        <strong>Status page</strong> — it defaults to your organization name, which is
        rarely what you want customers to see.
      </p>

      <H2 id="what">What visitors see</H2>
      <p>
        Only what you published: the monitor name, whether it is up, and when it was last
        checked. Never the target URL, the response, or anything about your other
        monitors.
      </p>
      <p>
        Status is never colour alone — every state is a coloured dot <em>and</em> a word.
        Roughly one in twelve men cannot reliably tell the green from the red, and this is
        the page where that matters most.
      </p>
      <p>
        The page updates itself while it is open, so someone watching during an incident
        sees recovery without refreshing.
      </p>

      <H2 id="api">From the API</H2>
      <Code lang="bash">{`
# Publish one monitor
curl -X PATCH https://beaconpulse.net/api/v1/monitors/$ID \\
  -H "Authorization: Bearer $BEACON_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"public": true}'

# The public read model — no credential, this is what your customers get
curl -s https://beaconpulse.net/api/v1/public/status/your-slug
`}</Code>
      <p>
        In a <Link href="/docs/automation">declared file</Link>, set{" "}
        <C>&quot;public&quot;: true</C> on the monitors you want listed.
      </p>
    </article>
  );
}
