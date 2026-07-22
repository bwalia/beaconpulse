import type { Metadata } from "next";
import { brand } from "@/brand";
import Link from "next/link";

import { ApiConsole } from "@/components/docs/api-console";
import { H2, Note } from "@/components/docs/parts";

export const metadata: Metadata = {
  title: "API console",
  description: `Call the live ${brand.name} API from your browser — paste a key and send a request.`,
};

export default function ConsolePage() {
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        API console
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        Call the real API from here. Paste a key, pick an endpoint, send it, read the
        response — the same request a <Link href="/docs/api">curl</Link> would make, with
        nothing to install.
      </p>

      <Note>
        <p>
          Start with <strong>System info</strong> in the picker — it needs no key, so it
          confirms the console reaches the API before you paste a credential. Then create
          a <Link href="/api-keys">viewer key</Link> and try the rest.
        </p>
      </Note>

      <div className="mt-8">
        <ApiConsole />
      </div>

      <H2 id="notes">Good to know</H2>
      <ul>
        <li>
          Your key is stored <strong>only in this browser</strong> and sent only to this
          API. Clear it any time with the button by the field.
        </li>
        <li>
          Requests run with <strong>your key&apos;s permissions</strong>. A viewer key can
          read everything and change nothing, which is what you want while exploring.
        </li>
        <li>
          Endpoints marked <em>changes data</em> do exactly that. A delete is confirmed
          before it is sent — there is no sandbox, this is your live organization.
        </li>
      </ul>
    </article>
  );
}
