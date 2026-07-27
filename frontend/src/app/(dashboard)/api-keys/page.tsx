"use client";

import { motion } from "framer-motion";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useState } from "react";

import { useConfirm } from "@/components/confirm";
import { CheckCircleIcon, LockIcon, PlusIcon, XIcon } from "@/components/icons";
import { Button, Card, EmptyState, Field, Input, PageHeader, Select, Skeleton } from "@/components/ui";
import { ApiRequestError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useApiKeys, useCreateApiKey, useRevokeApiKey } from "@/lib/hooks";
import { useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { useNow } from "@/lib/time";
import type { ApiKey, ApiKeyCreated } from "@/lib/types";

/**
 * API keys — the credentials machines authenticate with.
 *
 * The page is built around one fact that cannot be undone: the secret exists exactly
 * once, in the response that creates it. Everything about how it is shown follows from
 * that — it is unmissable, it explains itself, and it does not disappear on a stray
 * click, because "we cannot show you this again" is only fair if we made it obvious
 * the first time.
 */

/** The secret, shown once. */
function SecretPanel({ created, onDone }: { created: ApiKeyCreated; onDone: () => void }) {
  const t = useTranslations("pages.apiKeys");
  const c = useTranslations("common");
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(created.secret);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard blocked — the field is selectable regardless */
    }
  };

  return (
    <Card className="border-l-4 border-l-emerald-500 p-5">
      <div className="flex items-start gap-3">
        <CheckCircleIcon className="mt-0.5 h-6 w-6 shrink-0 text-emerald-600 dark:text-emerald-400" />
        <div className="min-w-0 flex-1">
          <h3 className="font-semibold text-slate-900 dark:text-white">
            {t("secretReady", { name: created.key.name })}
          </h3>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
            {t("secretWarning")}
          </p>

          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <input
              readOnly
              value={created.secret}
              onFocus={(e) => e.currentTarget.select()}
              aria-label={t("secretAriaLabel")}
              className="w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
            />
            <Button variant="secondary" onClick={copy} className="shrink-0">
              {copied ? c("copied") : c("copy")}
            </Button>
          </div>

          <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">
            {t.rich("githubHint", {
              mono: (chunks) => <span className="font-mono">{chunks}</span>,
              code: (chunks) => <span className="font-mono">{chunks}</span>,
              link: (chunks) => (
                <Link href="/docs/automation" className="text-brand-700 hover:underline dark:text-brand-400">
                  {chunks}
                </Link>
              ),
            })}
          </p>

          <div className="mt-4">
            <Button onClick={onDone}>{t("doneSaved")}</Button>
          </div>
        </div>
      </div>
    </Card>
  );
}

function CreateForm({ onCreated, onCancel }: { onCreated: (c: ApiKeyCreated) => void; onCancel: () => void }) {
  const t = useTranslations("pages.apiKeys");
  const c = useTranslations("common");
  const create = useCreateApiKey();
  const [name, setName] = useState("");
  const [role, setRole] = useState("");
  const [expiresInDays, setExpiresInDays] = useState("");
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const created = await create.mutateAsync({
        name: name.trim(),
        role: role || undefined,
        expires_in_days: expiresInDays ? Number(expiresInDays) : undefined,
      });
      onCreated(created);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : t("createFailed"));
    }
  };

  return (
    <Card className="p-5">
      <form onSubmit={submit} className="space-y-4">
        <Field label={t("nameLabel")} hint={t("nameHint")}>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("namePlaceholder")}
            autoFocus
            required
          />
        </Field>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("accessLabel")} hint={t("accessHint")}>
            <Select value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="">{t("roleOptionInherit")}</option>
              <option value="member">{t("roleOptionMember")}</option>
              <option value="viewer">{t("roleOptionViewer")}</option>
            </Select>
          </Field>

          <Field label={t("expiresLabel")} hint={t("expiresHint")}>
            <Select value={expiresInDays} onChange={(e) => setExpiresInDays(e.target.value)}>
              <option value="">{t("expiresNever")}</option>
              <option value="30">{t("expires30")}</option>
              <option value="90">{t("expires90")}</option>
              <option value="365">{t("expires365")}</option>
            </Select>
          </Field>
        </div>

        {error && (
          <p role="alert" className="text-sm text-red-700 dark:text-red-400">
            {error}
          </p>
        )}

        <div className="flex gap-2">
          <Button type="submit" disabled={create.isPending || !name.trim()}>
            {create.isPending ? t("creating") : t("createKey")}
          </Button>
          <Button type="button" variant="secondary" onClick={onCancel}>
            {c("cancel")}
          </Button>
        </div>
      </form>
    </Card>
  );
}

export default function ApiKeysPage() {
  const t = useTranslations("pages.apiKeys");
  const { user } = useAuth();
  const { data, isLoading } = useApiKeys();
  const revoke = useRevokeApiKey();
  const confirm = useConfirm();
  const now = useNow(60_000);
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.05);

  const [showForm, setShowForm] = useState(false);
  const [created, setCreated] = useState<ApiKeyCreated | null>(null);

  const canManage = user?.role === "owner" || user?.role === "admin";
  const keys = data?.data ?? [];

  const ago = (iso: string | undefined, now: number | null): string => {
    if (!iso) return t("agoNever");
    if (now === null) return "";
    const s = Math.max(0, Math.floor((now - Date.parse(iso)) / 1000));
    if (s < 60) return t("agoJustNow");
    const m = Math.floor(s / 60);
    if (m < 60) return t("agoMinutes", { count: m });
    const h = Math.floor(m / 60);
    if (h < 48) return t("agoHours", { count: h });
    return t("agoDays", { count: Math.floor(h / 24) });
  };

  const state = (k: ApiKey, now: number | null): { label: string; className: string } | null => {
    if (k.revoked_at) {
      return { label: t("statusRevoked"), className: "bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-400" };
    }
    if (k.expires_at && now !== null && Date.parse(k.expires_at) < now) {
      return { label: t("statusExpired"), className: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300" };
    }
    return null;
  };

  const roleLabel = (r: string): string => {
    const keyMap: Record<string, string> = {
      owner: "roleOwner",
      admin: "roleAdmin",
      member: "roleMember",
      viewer: "roleViewer",
    };
    return keyMap[r] ? t(keyMap[r]) : r;
  };

  return (
    <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-6">
      <PageHeader
        title={t("title")}
        subtitle={t("subtitle")}
        actions={
          canManage && !showForm && !created ? (
            <Button onClick={() => setShowForm(true)}>
              <PlusIcon className="h-4 w-4" />
              {t("createKey")}
            </Button>
          ) : null
        }
      />

      {created && (
        <motion.div variants={reveal}>
          <SecretPanel created={created} onDone={() => setCreated(null)} />
        </motion.div>
      )}

      {showForm && !created && (
        <motion.div variants={reveal}>
          <CreateForm
            onCreated={(c) => {
              setCreated(c);
              setShowForm(false);
            }}
            onCancel={() => setShowForm(false)}
          />
        </motion.div>
      )}

      {isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : keys.length === 0 && !showForm ? (
        <EmptyState
          icon={<LockIcon className="h-5 w-5" />}
          title={t("empty")}
          action={
            canManage ? <Button onClick={() => setShowForm(true)}>{t("createFirstKey")}</Button> : undefined
          }
        >
          {t("emptyBody")}
        </EmptyState>
      ) : (
        <motion.div variants={reveal}>
          <Card className="overflow-hidden p-0">
            <table className="w-full text-left">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
                <tr>
                  <th className="px-4 py-3 font-semibold">{t("colName")}</th>
                  <th className="px-4 py-3 font-semibold">{t("colKey")}</th>
                  <th className="px-4 py-3 font-semibold">{t("colAccess")}</th>
                  <th className="px-4 py-3 font-semibold">{t("colLastUsed")}</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => {
                  const st = state(k, now);
                  return (
                    <tr
                      key={k.id}
                      className={`border-b border-slate-100 last:border-0 dark:border-slate-800/60 ${st ? "opacity-60" : ""}`}
                    >
                      <td className="px-4 py-3.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-medium text-slate-900 dark:text-white">{k.name}</span>
                          {st && (
                            <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${st.className}`}>
                              {st.label}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 font-mono text-sm text-slate-600 dark:text-slate-300">
                        {k.prefix}…
                      </td>
                      <td className="px-4 py-3.5 text-sm capitalize text-slate-600 dark:text-slate-300">
                        {roleLabel(k.role)}
                      </td>
                      <td className="px-4 py-3.5 text-sm text-slate-600 dark:text-slate-300">
                        {/* Coarse on purpose: this answers "is anything still using
                            this key?", which is the question you ask before revoking. */}
                        {ago(k.last_used_at, now)}
                      </td>
                      <td className="px-4 py-3">
                        {canManage && !k.revoked_at && (
                          <div className="flex justify-end">
                            <Button
                              size="sm"
                              variant="ghost"
                              className="text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/50"
                              disabled={revoke.isPending}
                              onClick={async () => {
                                if (
                                  await confirm({
                                    title: t("revokeConfirmTitle", { name: k.name }),
                                    body: t("revokeConfirmBody"),
                                    confirmLabel: t("revokeConfirmLabel"),
                                    danger: true,
                                  })
                                ) {
                                  revoke.mutate(k.id);
                                }
                              }}
                            >
                              <XIcon className="h-3.5 w-3.5" />
                              {t("revoke")}
                            </Button>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </Card>
        </motion.div>
      )}

      {!canManage && (
        <p className="text-sm text-slate-500 dark:text-slate-400">
          {t("ownerAdminOnly")}
        </p>
      )}
    </motion.div>
  );
}
