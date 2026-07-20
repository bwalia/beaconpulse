import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, Fields, H2, H3, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Alerts & maintenance",
  description: "Where alerts go, how quickly they fire, and how to stay quiet during planned work.",
};

export default function Alerts() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        Alerts &amp; maintenance
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        An alert nobody sees is a dashboard. An alert that fires constantly is noise
        people learn to ignore — which is worse, because it teaches them to ignore the
        real one.
      </p>

      <H2 id="channels">Channels</H2>
      <p>Add as many as you need under <strong>Notifications</strong>.</p>
      <Fields
        rows={[
          { name: "Telegram", type: "channel", desc: "Fastest to set up. A bot token and a chat id." },
          { name: "Slack", type: "channel", desc: "An incoming webhook URL." },
          { name: "Email", type: "channel", desc: "Your own SMTP server — the credentials stay encrypted at rest." },
          { name: "Webhook", type: "channel", desc: "A POST to your endpoint. For PagerDuty, Opsgenie, or your own tooling." },
        ]}
      />
      <Note>
        <p>
          Send a test message when you add a channel. Finding out it was misconfigured
          during your first real outage is a bad way to find out.
        </p>
      </Note>

      <H2 id="sensitivity">How fast should it fire?</H2>
      <p>
        Set <C>alert_sensitivity</C> per monitor. The trade is always the same: fire early
        and you catch outages fast but page people for blips; fire late and every alert is
        real but you hear about it later.
      </p>
      <Fields
        rows={[
          { name: "immediate", desc: "Alert on the first failed check. For things where seconds matter and a false alarm is cheap." },
          { name: "balanced", desc: "Sustained failure. The default, and right for almost everything." },
          { name: "relaxed", desc: "Only prolonged outages (~5 min). For endpoints that are known to be flaky and not worth waking anyone for." },
        ]}
      />

      <H3>Slow, but not down</H3>
      <p>
        <C>response_time_warning_ms</C> alerts on a response that succeeded but took too
        long. This is usually the one that catches a problem before customers do —
        degradation almost always precedes failure.
      </p>

      <H2 id="ai">AI diagnosis</H2>
      <p>
        On paid plans, a failing monitor has a <strong>Diagnose with AI</strong> button.
        Beacon probes the target — DNS, the port, the TLS certificate, the response — and
        a model reads the results back in plain English: what broke, and what to do.
      </p>
      <p>
        The measurements are shown alongside the explanation, deliberately. The model is
        interpreting evidence and can be wrong; the evidence cannot. Its confidence is
        stated so an uncertain answer does not read like a certain one.
      </p>

      <H2 id="maintenance">Maintenance windows</H2>
      <p>
        Planned work should not page anyone. Schedule a window and alerts are suppressed
        for its duration — but <strong>probing continues</strong>, so you keep the history
        of what actually happened while you were working.
      </p>
      <Code lang="bash">{`
curl -X POST https://beaconpulse.net/api/v1/maintenance-windows \\
  -H "Authorization: Bearer $BEACON_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "title": "Database upgrade",
    "starts_at": "2026-02-01T22:00:00Z",
    "ends_at":   "2026-02-01T23:30:00Z"
  }'
`}</Code>
      <Note>
        <p>
          Prefer this to pausing a monitor. A paused monitor records nothing, so
          afterwards you cannot tell whether the work caused an outage — which is the one
          question you will want answered.
        </p>
      </Note>
      <p>
        A monitor under maintenance shows the state on your{" "}
        <Link href="/docs/status-pages">status page</Link> too, so customers see planned
        work as planned rather than as a failure.
      </p>
    </article>
  );
}
