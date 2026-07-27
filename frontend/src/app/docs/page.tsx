import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { brand } from "@/brand";
import Link from "next/link";

import { C, H2, Note } from "@/components/docs/parts";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("docs");
  return {
    title: t("navIntroduction"),
    description: t("indexMetaDesc", { brand: brand.name }),
  };
}

// Card titles reuse the nav label keys; each card's one-line description has its own key.
const PATHS = [
  { href: "/docs/quickstart", titleKey: "navQuickstart", descKey: "cardQuickstartDesc" },
  { href: "/docs/authentication", titleKey: "navAuthentication", descKey: "cardAuthDesc" },
  { href: "/docs/automation", titleKey: "navAutomation", descKey: "cardAutomationDesc" },
  { href: "/docs/api", titleKey: "navApiReference", descKey: "cardApiDesc" },
  { href: "/docs/console", titleKey: "navApiConsole", descKey: "cardConsoleDesc" },
];

export default async function DocsHome() {
  const t = await getTranslations("docs");
  return (
    <article className="prose-docs">
      <p className="text-sm font-semibold uppercase tracking-wider text-blue-600 dark:text-blue-400">
        {t("indexEyebrow")}
      </p>
      <h1 className="mt-2 text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
        {brand.name}
      </h1>
      <p className="mt-4 text-lg leading-relaxed text-slate-600 dark:text-slate-300">
        {t("indexLead", { brand: brand.name })}
      </p>

      {/* not-prose: these are navigation cards, not prose — the whole card is the
          link, so the prose underline would strike through every line of it. */}
      <div className="not-prose mt-8 grid gap-3 sm:grid-cols-2">
        {PATHS.map((p) => (
          <Link
            key={p.href}
            href={p.href}
            className="group rounded-xl border border-slate-200 p-4 transition-colors hover:border-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 motion-reduce:transition-none dark:border-slate-800 dark:hover:border-blue-500"
          >
            <p className="font-semibold text-slate-900 group-hover:text-blue-700 dark:text-white dark:group-hover:text-blue-400">
              {t(p.titleKey)}
            </p>
            <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">{t(p.descKey)}</p>
          </Link>
        ))}
      </div>

      <H2 id="how-it-works">How it works</H2>
      <p>
        You tell {brand.shortName} what to watch. It probes each target on a schedule from its own
        infrastructure — not from your servers, which is the point: a check that runs
        inside the thing it is checking goes quiet at exactly the moment it matters.
      </p>
      <p>
        When a probe fails often enough to mean something, {brand.shortName} raises an alert and
        sends it wherever you asked — Telegram, Slack, email, or your own webhook. If you
        publish a status page, your customers see the same truth without having to ask.
      </p>

      <H2 id="concepts">The four things to know</H2>
      <dl className="mt-4 space-y-4">
        <div>
          <dt className="font-semibold text-slate-900 dark:text-white">Monitor</dt>
          <dd className="mt-1 text-slate-600 dark:text-slate-300">
            One thing being watched, of one <Link href="/docs/monitors">type</Link> —{" "}
            <C>https</C>, <C>tcp</C>, <C>dns</C>, <C>heartbeat</C> and so on.
          </dd>
        </div>
        <div>
          <dt className="font-semibold text-slate-900 dark:text-white">Project</dt>
          <dd className="mt-1 text-slate-600 dark:text-slate-300">
            A group of monitors. Most teams use one per environment or per product.
          </dd>
        </div>
        <div>
          <dt className="font-semibold text-slate-900 dark:text-white">Notification channel</dt>
          <dd className="mt-1 text-slate-600 dark:text-slate-300">
            Where alerts go. Add as many as you like.
          </dd>
        </div>
        <div>
          <dt className="font-semibold text-slate-900 dark:text-white">Status page</dt>
          <dd className="mt-1 text-slate-600 dark:text-slate-300">
            A public page showing only the monitors you explicitly publish.
          </dd>
        </div>
      </dl>

      <Note>
        <p>
          Everything in the dashboard is available over the{" "}
          <Link href="/docs/api">API</Link>, using the same permissions. There is no
          second-class API surface, because the dashboard uses it too.
        </p>
      </Note>
    </article>
  );
}
