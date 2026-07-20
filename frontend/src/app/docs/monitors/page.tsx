import type { Metadata } from "next";
import Link from "next/link";

import { C, Code, Fields, H2, H3, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Monitor types",
  description: "Every kind of check Beacon Pulse can run, and when to reach for each.",
};

export default function Monitors() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        Monitor types
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Seven kinds of check. The type decides what a target means and what counts as a
        failure.
      </p>

      <H2 id="https">https &amp; http</H2>
      <p>
        Fetches a URL and checks the response. The one you want for a website or an API.
      </p>
      <Code lang="json">{`
{ "name": "api", "type": "https", "target": "https://api.example.com",
  "settings": {
    "valid_status_codes": [200, 204],
    "body_keyword": "ok",
    "response_time_warning_ms": 2000
  } }
`}</Code>
      <p>
        Without <C>valid_status_codes</C>, any 2xx passes. Set it when a 301 to a login
        page would otherwise look healthy — which is the usual way a broken deploy passes
        monitoring.
      </p>

      <H3>Checking the body, not just the status</H3>
      <p>
        <C>body_keyword</C> is what separates &ldquo;the web server is running&rdquo;
        from &ldquo;the application works&rdquo;. A frontend that renders an error page
        still returns 200, and the difference matters at 3am.
      </p>

      <H2 id="ssl">ssl</H2>
      <p>
        Watches a TLS certificate and alerts before it expires, which is the outage
        everyone has once and never forgets. Set{" "}
        <C>ssl_expiry_warning_days</C> to however long you need to renew.
      </p>
      <Code lang="json">{`
{ "name": "cert", "type": "ssl", "target": "https://example.com",
  "settings": { "ssl_expiry_warning_days": 21 } }
`}</Code>

      <H2 id="tcp">tcp</H2>
      <p>
        Opens a connection to a host and port. For things that are not HTTP: a database,
        a message broker, an SMTP server.
      </p>
      <Code lang="json">{`{ "name": "db", "type": "tcp", "target": "db.example.com:5432" }`}</Code>

      <H2 id="dns">dns</H2>
      <p>
        Resolves a name and checks it answers. Catches an expired domain or a botched
        nameserver change — failures that look like everything being down at once.
      </p>
      <Code lang="json">{`
{ "name": "apex", "type": "dns", "target": "example.com",
  "settings": { "dns_query_type": "A" } }
`}</Code>

      <H2 id="icmp">icmp</H2>
      <p>Pings a host. Useful for network gear; a poor proxy for whether a service works.</p>

      <H2 id="heartbeat">heartbeat</H2>
      <p>
        The inside-out one. Instead of us checking your service, your service checks in
        with us — and we alert when it <em>stops</em>. This is how you monitor a cron job,
        a nightly backup, or a queue worker: things whose failure is silence.
      </p>
      <p>A heartbeat has no target. You get a URL; call it on success.</p>
      <Code lang="bash">{`0 2 * * *  /path/to/backup.sh && curl -fsS https://beaconpulse.net/api/v1/ping/<token>`}</Code>
      <Note>
        <p>
          Note the <C>&amp;&amp;</C>. The ping must only fire if the job actually
          succeeded, or you have built a monitor that reports success whenever cron is
          running.
        </p>
      </Note>
      <p>
        <C>grace_seconds</C> is the slack beyond the interval before a missed ping alerts.
        A nightly job that usually takes twenty minutes wants an hour of grace, so a slow
        night is not an incident.
      </p>

      <H2 id="settings">All settings</H2>
      <Fields
        rows={[
          { name: "method", desc: <>HTTP method. Defaults to <C>GET</C>.</> },
          { name: "valid_status_codes", type: "number[]", desc: "Codes that count as healthy. Default: any 2xx." },
          { name: "body_keyword", desc: "Must appear in the response body." },
          { name: "body_not_keyword", desc: "Must NOT appear. Good for catching an error page that returns 200." },
          { name: "follow_redirects", type: "boolean", desc: "Follow 3xx before judging the response." },
          { name: "headers", type: "object", desc: "Sent with the request — an API key for a protected health endpoint." },
          { name: "skip_tls_verify", type: "boolean", desc: "Accept an invalid certificate. Only for internal hosts with a private CA." },
          { name: "ssl_expiry_warning_days", type: "number", desc: "Warn this many days before the certificate expires." },
          { name: "response_time_warning_ms", type: "number", desc: "Alert when a response is slower than this, even though it succeeded." },
          { name: "dns_query_type", desc: <>For <C>dns</C> monitors: A, AAAA, CNAME, MX, TXT, NS, SOA, CAA.</> },
          { name: "alert_sensitivity", desc: <>How stubborn a failure must be before it alerts. See <Link href="/docs/alerts">Alerts</Link>.</> },
        ]}
      />

      <Note kind="warn">
        <p>
          Targets must be publicly reachable. A monitor pointed at a private address —{" "}
          <C>10.x</C>, <C>192.168.x</C>, <C>localhost</C> — is refused, because our probes
          run in our infrastructure and would otherwise be a way to scan it.
        </p>
      </Note>
    </article>
  );
}
