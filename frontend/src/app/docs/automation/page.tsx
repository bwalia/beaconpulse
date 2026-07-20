import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, Fields, H2, H3, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "CI & automation",
  description: "Keep your monitors in your repository and apply them from a workflow.",
};

export default function Automation() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        CI &amp; automation
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Keep the domains you monitor next to the code that serves them, and keep the two
        in step automatically.
      </p>

      <H2 id="why">Why not just POST a monitor?</H2>
      <p>
        Because a workflow runs more than once. A pipeline that called{" "}
        <C>POST /monitors</C> on every push would create a duplicate on every re-run,
        retry and re-merge — and a fortnight later the same domain is being probed forty
        times, and billed forty times.
      </p>
      <p>
        <C>POST /api/v1/sync</C> asks a different question. Not &ldquo;add this&rdquo;,
        but <em>&ldquo;here is the set I want — work out the difference.&rdquo;</em>{" "}
        Running it a hundred times with an unchanged file makes zero writes and reports
        everything unchanged.
      </p>

      <H2 id="declare">1. Declare your monitors</H2>
      <Code lang="json" title="monitors.json">{`
{
  "project": "production",
  "monitors": [
    { "name": "www", "type": "https", "target": "https://example.com", "public": true },

    { "name": "api", "type": "https", "target": "https://api.example.com",
      "interval_seconds": 30,
      "settings": {
        "valid_status_codes": [200],
        "body_keyword": "ok",
        "ssl_expiry_warning_days": 14
      } },

    { "name": "db", "type": "tcp", "target": "db.example.com:5432" },

    { "name": "nightly-backup", "type": "heartbeat", "grace_seconds": 3600 }
  ]
}
`}</Code>
      <Note>
        <p>
          <C>name</C> is the identity — it is how a monitor is matched from one run to the
          next. Renaming one creates a new monitor and reports the old one as removable.
        </p>
      </Note>

      <H2 id="workflow">2. Apply it on push</H2>
      <Code lang="yaml" title=".github/workflows/monitors.yml">{`
name: Sync monitors

on:
  push:
    branches: [main]
    paths: ['monitors.json']
  pull_request:
    paths: ['monitors.json']

env:
  BEACON_URL: https://beaconpulse.net

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # On a pull request, show what WOULD change and apply none of it, so a
      # reviewer sees the plan before it is merged.
      - name: Plan
        if: github.event_name == 'pull_request'
        run: |
          jq '. + {dry_run: true}' monitors.json > payload.json
          curl -sS --fail-with-body -X POST "$BEACON_URL/api/v1/sync" \\
            -H "Authorization: Bearer $BEACON_API_KEY" \\
            -H "Content-Type: application/json" \\
            -d @payload.json > result.json
          jq -r '.items[] | "  \\(.action)\\t\\(.name)"' result.json
        env:
          BEACON_API_KEY: \${{ secrets.BEACON_API_KEY }}

      - name: Apply
        if: github.event_name == 'push'
        run: |
          curl -sS --fail-with-body -X POST "$BEACON_URL/api/v1/sync" \\
            -H "Authorization: Bearer $BEACON_API_KEY" \\
            -H "Content-Type: application/json" \\
            -d @monitors.json > result.json

          jq -r '.items[] | "  \\(.action)\\t\\(.name)"' result.json

          # A monitor rejected on its own merits still returns 200, because the
          # rest of the file WAS applied. Check the count, not just the status.
          failed=$(jq -r '.failed' result.json)
          if [ "$failed" != "0" ]; then
            jq -r '.items[] | select(.action=="error") | "::error::\\(.name): \\(.error)"' result.json
            exit 1
          fi
        env:
          BEACON_API_KEY: \${{ secrets.BEACON_API_KEY }}
`}</Code>
      <p>
        That is the whole integration. Push a change to <C>monitors.json</C> and your
        monitoring follows it.
      </p>

      <H2 id="response">3. Read the response</H2>
      <Code lang="json">{`
{
  "project": "production",
  "dry_run": false,
  "created": 1, "updated": 1, "unchanged": 2,
  "removed": 0, "would_remove": 1, "failed": 0,
  "items": [
    { "name": "api",     "action": "updated",      "id": "..." },
    { "name": "db",      "action": "unchanged",    "id": "..." },
    { "name": "old-api", "action": "would_remove", "id": "..." },
    { "name": "www",     "action": "created",      "id": "..." }
  ]
}
`}</Code>
      <Fields
        rows={[
          { name: "created", type: "action", desc: "Did not exist; now does." },
          { name: "updated", type: "action", desc: "Existed and differed; now matches the file." },
          { name: "unchanged", type: "action", desc: "Already matched — no write was performed." },
          { name: "would_remove", type: "action", desc: <>No longer declared. Pass <C>prune</C> to remove it.</> },
          { name: "removed", type: "action", desc: <>Deleted, because <C>prune</C> was set.</> },
          { name: "error", type: "action", desc: "This one failed; the others still applied." },
        ]}
      />
      <p>Items are sorted by name, so consecutive runs diff cleanly.</p>

      <H2 id="prune">Deleting is opt-in</H2>
      <p>
        Remove a monitor from the file and it is <strong>reported, not deleted</strong>.
        To actually remove it, add <C>&quot;prune&quot;: true</C>.
      </p>
      <Note kind="warn">
        <p>
          Understand this default before you turn it off. A workflow with a bad path
          filter, an empty matrix, or a failed template step declares <em>zero</em>{" "}
          monitors. With pruning on, that one mistake deletes your production
          monitoring — at the exact moment nobody notices, because the monitoring is what
          just went. Off, the same mistake changes nothing and tells you what it would
          have done.
        </p>
      </Note>
      <p>
        Turn it on once you trust the pipeline, and the file becomes the single source of
        truth: delete a line, and the monitor goes with it.
      </p>

      <H3>Preview before merging</H3>
      <p>
        <C>&quot;dry_run&quot;: true</C> computes the plan and applies none of it — which
        is what the pull-request step above does, so a reviewer sees &ldquo;this adds 2
        and removes 1&rdquo; before anyone merges.
      </p>

      <H2 id="partial">Partial failures</H2>
      <p>
        One rejected monitor does not discard the rest. If entry three hits your plan
        limit, entries one, two and four still apply, and three comes back as:
      </p>
      <Code lang="json">{`{ "name": "extra", "action": "error", "error": "monitor limit reached for the free plan" }`}</Code>
      <p>
        The response is <C>200</C> in that case — some of your declaration <em>was</em>{" "}
        applied, and a 4xx would tell your workflow nothing happened, prompting exactly
        the retry that creates duplicates. <strong>Check <C>failed</C> in the body</strong>,
        as the example workflow does.
      </p>

      <H2 id="other">Other things worth automating</H2>
      <Code lang="bash">{`
# Fail a deploy if anything is already down
down=$(curl -s "$BEACON_URL/api/v1/monitors?status=down" \\
  -H "Authorization: Bearer $BEACON_API_KEY" | jq '.pagination.total')
[ "$down" = "0" ] || { echo "::error::$down monitor(s) already down"; exit 1; }

# Suppress alerts for a planned deploy
curl -X POST "$BEACON_URL/api/v1/maintenance-windows" \\
  -H "Authorization: Bearer $BEACON_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"title":"Deploy","starts_at":"2026-01-01T22:00:00Z","ends_at":"2026-01-01T23:00:00Z"}'
`}</Code>
      <p>
        See the <Link href="/docs/api">API reference</Link> for everything else.
      </p>

      <H2 id="troubleshooting">Troubleshooting</H2>
      <Fields
        rows={[
          { name: "401", type: "error", desc: "Revoked, expired, or mistyped key. Every failure returns the same message on purpose — check its status in the dashboard." },
          { name: "403", type: "error", desc: <>The key&apos;s role is too low. A <C>viewer</C> key cannot write.</> },
          { name: "422", type: "error", desc: "A plan limit. The other monitors in the file still applied; this one is in items with action \"error\"." },
          { name: "Everything says created", type: "symptom", desc: <>The <C>name</C> changed between runs. Name is the identity; a renamed monitor is a new one.</> },
        ]}
      />
    </article>
  );
}
