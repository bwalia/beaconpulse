import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { brand } from "@/brand";
import Link from "next/link";

import { ApiConsole } from "@/components/docs/api-console";
import { H2, Note } from "@/components/docs/parts";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("docs");
  return {
    title: t("navApiConsole"),
    description: t("consolePage.metaDesc", { brand: brand.name }),
  };
}

export default async function ConsolePage() {
  const t = await getTranslations("docs");
  return (
    <article className="prose-docs">
      <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        {t("navApiConsole")}
      </h1>
      <p className="mt-4 text-lg text-slate-600 dark:text-slate-300">
        {t.rich("consolePage.lead", {
          link: (chunks) => <Link href="/docs/api">{chunks}</Link>,
        })}
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
