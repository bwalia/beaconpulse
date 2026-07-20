import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, H2, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Quickstart",
  description: "Sign up and get your first domain monitored in about five minutes.",
};

export default function Quickstart() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">Quickstart</h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        From nothing to a monitored domain with alerts, in about five minutes.
      </p>

      <H2 id="1-account">1. Create an account</H2>
      <p>
        <Link href="/register">Sign up</Link> with your email and a name for your
        organization. You get the Free plan immediately — ten monitors, checked every
        sixty seconds. No card.
      </p>
      <Note>
        <p>
          Use an address you can actually be reached at. It is where we tell you that
          your monitoring stopped, which is the one message you cannot afford to miss.
        </p>
      </Note>

      <H2 id="2-monitor">2. Add your first monitor</H2>
      <p>
        Go to <strong>Monitors → Add monitor</strong>. The only two things that matter to
        start with:
      </p>
      <ul>
        <li>
          <strong>Type</strong> — <C>https</C> for a website or API.
        </li>
        <li>
          <strong>Target</strong> — the full URL, e.g. <C>https://example.com</C>.
        </li>
      </ul>
      <p>
        The first probe runs within a minute. A monitor that has never been checked shows
        as <C>unknown</C> rather than up, because we have not looked yet and saying
        otherwise would be a guess.
      </p>

      <H2 id="3-alerts">3. Tell it where to shout</H2>
      <p>
        A monitor with nowhere to send an alert is a dashboard you have to remember to
        look at. Under <strong>Notifications</strong>, add a channel — Telegram is the
        quickest to set up, and Slack, email and plain webhooks all work.
      </p>
      <p>
        Send a test notification while you are there. Discovering that a channel was
        misconfigured during your first real outage is a bad way to find out.
      </p>

      <H2 id="4-api">4. Optional: do it from your terminal</H2>
      <p>
        Everything above is available over the API. Create a key under{" "}
        <strong>API keys</strong>, then:
      </p>
      <Code lang="bash">{`
# What am I watching, and is any of it broken?
curl -s https://beaconpulse.net/api/v1/monitors \\
  -H "Authorization: Bearer $BEACON_API_KEY" | jq '.data[] | {name, last_status}'
`}</Code>
      <p>
        See <Link href="/docs/authentication">Authentication</Link> to create the key,
        and <Link href="/docs/automation">CI &amp; automation</Link> to keep your monitors
        in a file in your repository instead of clicking.
      </p>

      <H2 id="next">Where to go next</H2>
      <ul>
        <li>
          <Link href="/docs/monitors">Monitor types</Link> — what else you can watch, incl.
          certificate expiry and cron jobs that stop running.
        </li>
        <li>
          <Link href="/docs/alerts">Alerts &amp; maintenance</Link> — tuning how quickly a
          blip becomes an alert, and silencing planned work.
        </li>
        <li>
          <Link href="/docs/status-pages">Status pages</Link> — a public page your
          customers can check instead of emailing you.
        </li>
      </ul>
    </article>
  );
}
