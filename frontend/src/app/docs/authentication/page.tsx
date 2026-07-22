import type { Metadata } from "next";
import { brand } from "@/brand";
import Link from "next/link";

import { C, Code, Endpoint, Fields, H2, H3, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "Authentication",
  description: "Create an API key and make your first authenticated request.",
};

export default function Authentication() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        Authentication
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Every API request carries a bearer token. For anything automated, that token is
        an API key.
      </p>

      <H2 id="create">Create a key</H2>
      <p>
        In the dashboard, go to <strong>API keys → Create key</strong>. Name it after
        whatever will use it — <C>github-actions</C>, <C>terraform</C>,{" "}
        <C>status-dashboard</C> — because that name is what you will be looking at in six
        months deciding whether it is safe to revoke.
      </p>

      <Note kind="security">
        <p>
          <strong>The secret is shown once.</strong> We store only a hash, so it cannot be
          shown again or recovered — not by you, and not by us. Copy it straight into your
          secret store. If it is lost, revoke the key and make another.
        </p>
      </Note>

      <H3>Choose the least access it needs</H3>
      <Fields
        rows={[
          { name: "viewer", type: "role", desc: "Read everything, change nothing. The right choice for a dashboard, an export job, or anything that only looks." },
          { name: "member", type: "role", desc: "Read and write monitors, projects and channels. What CI needs." },
          { name: "admin", type: "role", desc: "Everything a member can do, plus billing and settings." },
        ]}
      />
      <p>
        A key can never have more access than the person who created it. An admin cannot
        mint an owner key, which stops a key being a quiet route to promoting yourself.
      </p>

      <H2 id="use">Use it</H2>
      <Code lang="bash" title="Your first authenticated request">{`
curl -s https://${brand.apiHost}/api/v1/monitors \\
  -H "Authorization: Bearer bp_xxxxxxxxxxxxxxxxxxxxxxxx"
`}</Code>
      <p>Store it as an environment variable rather than pasting it into scripts:</p>
      <Code lang="bash">{`
export BEACON_API_KEY="bp_xxxxxxxxxxxxxxxxxxxxxxxx"

curl -s https://${brand.apiHost}/api/v1/monitors \\
  -H "Authorization: Bearer $BEACON_API_KEY" | jq
`}</Code>
      <p>
        In GitHub Actions, put it in{" "}
        <C>Settings → Secrets and variables → Actions</C> and read it as{" "}
        <C>${"{{"} secrets.BEACON_API_KEY {"}}"}</C>.
      </p>

      <H2 id="what-a-key-is">What a key is, and is not</H2>
      <H3>It belongs to the organization, not to you</H3>
      <p>
        A key keeps working after you leave, and its changes are attributed to the
        organization rather than to your user. That is deliberate — a credential that
        stopped working when someone changed jobs would take a customer&apos;s deploy
        pipeline down with it. Revoke keys when the thing using them is retired, not when
        people move on.
      </p>

      <H3>It carries no plan or balance</H3>
      <p>
        A key resolves to your organization; your plan, credit and limits are read fresh
        on every request. Upgrade, downgrade or top up and the same key reflects it
        immediately. There is nothing to regenerate, and a key can never be
        &ldquo;stuck&rdquo; on an old tier.
      </p>

      <H3>Keys cannot manage keys</H3>
      <p>
        Creating and revoking keys requires signing in. A leaked key therefore cannot mint
        itself a successor that survives you revoking the original — which is the
        difference between an incident and a permanent problem.
      </p>

      <H2 id="revoke">Revoke a key</H2>
      <p>
        <strong>API keys → Revoke</strong>. It stops working immediately, everywhere. The
        list shows when each key was last used, so you can tell what is still in service
        before you pull it.
      </p>
      <Note kind="warn">
        <p>
          If a key has leaked, revoke it first and investigate second. A revoked key
          cannot be un-revoked, which is the correct trade in that direction.
        </p>
      </Note>

      <H2 id="endpoints">Key management endpoints</H2>
      <p>These are the ones an API key cannot call.</p>
      <div className="my-5 rounded-xl border border-slate-200 px-4 dark:border-slate-800">
        <Endpoint method="GET" path="/api/v1/api-keys" auth="session">
          List your organization&apos;s keys. Secrets are not stored, so none are returned.
        </Endpoint>
        <Endpoint method="POST" path="/api/v1/api-keys" auth="session">
          Create one. The response contains the secret, once.
        </Endpoint>
        <Endpoint method="DELETE" path="/api/v1/api-keys/{id}" auth="session">
          Revoke. Safe to call twice.
        </Endpoint>
      </div>

      <H2 id="errors">When it does not work</H2>
      <Fields
        rows={[
          { name: "401", type: "status", desc: "Invalid, revoked, expired, or mistyped. Every one of those returns the same message on purpose — a stolen key should learn nothing from the reply." },
          { name: "403", type: "status", desc: <>The key&apos;s role is too low. A <C>viewer</C> key cannot write.</> },
          { name: "429", type: "status", desc: <>Too many requests. Back off by the <C>Retry-After</C> header.</> },
        ]}
      />

      <p>
        Next: the <Link href="/docs/api">API reference</Link>, or{" "}
        <Link href="/docs/automation">wiring it into CI</Link>.
      </p>
    </article>
  );
}
