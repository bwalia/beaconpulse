import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, Endpoint, Fields, H2, H3, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "API reference",
  description: "Every Beacon Pulse endpoint, with fields and examples.",
};

export default function ApiReference() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        API reference
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Base URL <C>https://beaconpulse.net</C>. Everything is under <C>/api/v1</C>, takes
        and returns JSON, and needs an{" "}
        <Link href="/docs/authentication">API key</Link>.
      </p>

      <Note>
        <p>
          Prefer to try before you read? The <Link href="/docs/console">API console</Link>{" "}
          lets you paste a key and send any of these requests from your browser.
        </p>
      </Note>
      <Note>
        <p>
          This is the same API the dashboard uses. There is no reduced &ldquo;public
          API&rdquo; — if you can do it in the UI, you can do it here, under the same
          permissions and the same plan limits.
        </p>
      </Note>

      <H2 id="conventions">Conventions</H2>
      <H3>Collections</H3>
      <Code lang="json">{`
{
  "data": [ ... ],
  "pagination": { "total": 132, "limit": 50, "offset": 0 }
}
`}</Code>
      <p>
        <C>?limit=</C> (default 50, max 200) and <C>?offset=</C>. Most list endpoints also
        take <C>?search=</C>.
      </p>

      <H3>Errors</H3>
      <Code lang="json">{`
{
  "error": {
    "code": "validation",
    "message": "monitor limit reached for the free plan",
    "fields": { "target": "must be a valid URL" },
    "request_id": "01HQ..."
  }
}
`}</Code>
      <p>
        Quote the <C>request_id</C> if you ever need to ask us what happened — it is how
        we find your request in the logs.
      </p>
      <Fields
        rows={[
          { name: "400 / 422", type: "validation", desc: "The request was malformed, or it hit a plan limit." },
          { name: "401", type: "unauthorized", desc: "Missing, invalid, revoked or expired credential." },
          { name: "403", type: "forbidden", desc: "Authenticated, but your role does not allow it." },
          { name: "404", type: "not_found", desc: "No such resource — or it belongs to another organization. The two are indistinguishable by design." },
          { name: "429", type: "rate_limited", desc: <>Slow down; honour <C>Retry-After</C>.</> },
        ]}
      />

      <H3>Tenancy</H3>
      <p>
        Every resource is scoped to the organization the credential belongs to. Another
        tenant&apos;s monitor is <strong>not found</strong>, never
        &ldquo;forbidden&rdquo; — telling you it exists would be the leak.
      </p>

      <H2 id="monitors">Monitors</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/monitors">
          List. Filters: <C>search</C>, <C>status</C>, <C>type</C>, <C>project_id</C>.
        </Endpoint>
        <Endpoint method="POST" path="/api/v1/monitors">Create one.</Endpoint>
        <Endpoint method="GET" path="/api/v1/monitors/{id}">Fetch one.</Endpoint>
        <Endpoint method="PATCH" path="/api/v1/monitors/{id}">Update. Send only what changes.</Endpoint>
        <Endpoint method="DELETE" path="/api/v1/monitors/{id}">Delete, with its history.</Endpoint>
        <Endpoint method="POST" path="/api/v1/monitors/{id}/pause">
          Stop probing without deleting.
        </Endpoint>
        <Endpoint method="POST" path="/api/v1/monitors/{id}/resume">Start again.</Endpoint>
        <Endpoint method="GET" path="/api/v1/monitors/{id}/metrics">Recent uptime and response times.</Endpoint>
        <Endpoint method="GET" path="/api/v1/monitors/usage">How many monitors you have, against your plan.</Endpoint>
      </div>

      <H3>Create a monitor</H3>
      <Code lang="bash">{`
curl -X POST https://beaconpulse.net/api/v1/monitors \\
  -H "Authorization: Bearer $BEACON_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "project_id": "b1f2...",
    "name": "api",
    "type": "https",
    "target": "https://api.example.com",
    "interval_seconds": 60,
    "settings": { "valid_status_codes": [200], "body_keyword": "ok" }
  }'
`}</Code>
      <Fields
        rows={[
          { name: "project_id", required: true, type: "uuid", desc: "Which project it belongs to." },
          { name: "name", required: true, desc: "How you will recognise it." },
          { name: "type", required: true, desc: <>See <Link href="/docs/monitors">monitor types</Link>.</> },
          { name: "target", desc: <>Required for everything except <C>heartbeat</C>.</> },
          { name: "interval_seconds", type: "number", desc: "How often to check. Floor depends on your plan." },
          { name: "timeout_seconds", type: "number", desc: "How long to wait before calling it a failure." },
          { name: "grace_seconds", type: "number", desc: <>Heartbeat only: slack before a missed ping alerts.</> },
          { name: "enabled", type: "boolean", desc: "Defaults to true." },
          { name: "public", type: "boolean", desc: "Publish on your status page. Defaults to false — nothing is exposed by accident." },
          { name: "settings", type: "object", desc: <>Per-type options; see <Link href="/docs/monitors">monitor types</Link>.</> },
        ]}
      />

      <H2 id="sync">Sync (declarative)</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="POST" path="/api/v1/sync">
          Apply a whole set of monitors at once. Idempotent.
        </Endpoint>
      </div>
      <p>
        The endpoint to use from CI. It takes the monitors you want and works out the
        difference, so running it on every push is safe.{" "}
        <Link href="/docs/automation">Full guide →</Link>
      </p>

      <H2 id="projects">Projects</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/projects">List.</Endpoint>
        <Endpoint method="POST" path="/api/v1/projects">
          Create. <C>{`{ "name", "description?", "environment?" }`}</C>
        </Endpoint>
        <Endpoint method="PATCH" path="/api/v1/projects/{id}">Update.</Endpoint>
        <Endpoint method="DELETE" path="/api/v1/projects/{id}">Delete.</Endpoint>
      </div>

      <H2 id="alerts">Alerts</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/alerts">
          Everything firing right now. Filter with <C>?severity=critical</C>.
        </Endpoint>
      </div>
      <Code lang="bash" title="What is broken right now?">{`
curl -s "https://beaconpulse.net/api/v1/alerts" \\
  -H "Authorization: Bearer $BEACON_API_KEY" \\
  | jq -r '.data[] | "\\(.severity)\\t\\(.monitor_name)\\t\\(.target)"'
`}</Code>

      <H2 id="channels">Notification channels</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/notification-channels">List.</Endpoint>
        <Endpoint method="POST" path="/api/v1/notification-channels">
          Create. Types: <C>telegram</C>, <C>slack</C>, <C>email</C>, <C>webhook</C>.
        </Endpoint>
        <Endpoint method="PATCH" path="/api/v1/notification-channels/{id}">Update or enable/disable.</Endpoint>
        <Endpoint method="DELETE" path="/api/v1/notification-channels/{id}">Delete.</Endpoint>
        <Endpoint method="POST" path="/api/v1/notification-channels/{id}/test">
          Send a test message. Do this when you create one.
        </Endpoint>
      </div>

      <H2 id="maintenance">Maintenance windows</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/maintenance-windows">List.</Endpoint>
        <Endpoint method="POST" path="/api/v1/maintenance-windows">
          Schedule one. Alerts are suppressed; probing continues.
        </Endpoint>
        <Endpoint method="PATCH" path="/api/v1/maintenance-windows/{id}">Update.</Endpoint>
        <Endpoint method="DELETE" path="/api/v1/maintenance-windows/{id}">Cancel.</Endpoint>
      </div>
      <Note>
        <p>
          Prefer a maintenance window to pausing a monitor during a deploy. Alerts stay
          quiet, but the probes keep running — so you still have the history of what
          happened while you were working.
        </p>
      </Note>

      <H2 id="status-page">Status page</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/status-page">Your settings.</Endpoint>
        <Endpoint method="PATCH" path="/api/v1/status-page">
          Update title, custom slug, or publish/unpublish.
        </Endpoint>
        <Endpoint method="GET" path="/api/v1/public/status/{slug}" auth="public">
          The public read model. No credential — this is what your customers see.
        </Endpoint>
      </div>

      <H2 id="billing">Billing &amp; diagnosis</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/billing">
          Plan, credit, and what remains.
        </Endpoint>
        <Endpoint method="POST" path="/api/v1/monitors/{id}/diagnose">
          AI root-cause analysis for a failing monitor. Paid plans.
        </Endpoint>
      </div>

      <H2 id="ops">Operational</H2>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/system/info" auth="public">
          Version, environment and uptime of the API you are talking to.
        </Endpoint>
        <Endpoint method="GET" path="/api/v1/ping/{token}" auth="public">
          Heartbeat check-in. The URL a cron job calls on success.
        </Endpoint>
      </div>

      <H2 id="limits">Rate limits</H2>
      <p>
        Limits are per organization for authenticated calls and per address for anything
        anonymous. A refusal is a <C>429</C> with a <C>Retry-After</C> header saying how
        long to wait — respect it and you will not be refused twice.
      </p>
      <p>
        Normal use does not come close. If you are hitting them, you are probably polling
        where you could be reacting: a{" "}
        <Link href="/docs/alerts">webhook channel</Link> tells you the moment something
        breaks, without asking every few seconds.
      </p>
    </article>
  );
}
