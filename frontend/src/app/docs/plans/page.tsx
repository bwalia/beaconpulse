import type { Metadata } from "next";
import { brand } from "@/brand";
import Link from "next/link";

import { C, Code, Fields, H2, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Plans & billing",
  description: "What each plan includes, and how pay-as-you-go credit works.",
};

export default function Plans() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        Plans &amp; billing
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Pay monthly, or by the hour. Both work; neither locks you in.
      </p>

      <H2 id="plans">Plans</H2>
      <Fields
        rows={[
          { name: "Free", type: "$0", desc: "10 monitors · 60s minimum interval · Telegram alerts" },
          { name: "Starter", type: "$19/mo", desc: "50 monitors · 30s interval · all alert channels · AI diagnosis" },
          { name: "Pro", type: "$79/mo", desc: "500 monitors · 10s interval · priority alerting" },
          { name: "Pay-as-you-go", type: "from $1", desc: "500 monitors · 30s interval · no subscription; you spend credit while monitors run" },
        ]}
      />

      <H2 id="payg">How pay-as-you-go works</H2>
      <p>
        Add any amount of credit and it becomes <strong>monitor-hours</strong>. $1 buys 5.
        One monitor running for one hour spends one monitor-hour, so two monitors spend
        two per hour — you pay for what you actually watch.
      </p>
      <Code lang="text">{`
$5 → 25 monitor-hours
2 monitors running → 12.5 hours of monitoring
5 monitors running → 5 hours
`}</Code>
      <p>
        The billing page shows what you have used, what is left, and roughly when it runs
        out at your current monitor count. When credit reaches zero you drop to the Free
        limits — nothing is deleted, and topping up picks straight back up.
      </p>
      <Note>
        <p>
          Credit never expires, and a subscription does not consume it. If you subscribe
          with credit left over, it sits there until you need it.
        </p>
      </Note>

      <H2 id="ai-cost">AI diagnosis</H2>
      <p>
        Included on Starter (100/month) and Pro (1000/month). On pay-as-you-go it costs 5
        monitor-minutes — about 1.7¢ — and the price is shown on the button before you
        press it.
      </p>
      <p>
        You are only charged if a diagnosis is actually returned. If the analysis fails you
        get the raw measurements and the credit back.
      </p>

      <H2 id="limits">What happens at a limit</H2>
      <p>
        Creating a monitor past your plan&apos;s limit returns a clear error rather than
        silently succeeding, and existing monitors keep running. Over the API you get a{" "}
        <C>422</C> naming the limit — which is why a{" "}
        <Link href="/docs/automation">declared file</Link> reports that one monitor as an
        error and still applies the rest.
      </p>

      <H2 id="check">Check from the API</H2>
      <Code lang="bash">{`
curl -s https://${brand.apiHost}/api/v1/billing \\
  -H "Authorization: Bearer $BEACON_API_KEY" | jq

curl -s https://${brand.apiHost}/api/v1/monitors/usage \\
  -H "Authorization: Bearer $BEACON_API_KEY" | jq
`}</Code>
    </article>
  );
}
