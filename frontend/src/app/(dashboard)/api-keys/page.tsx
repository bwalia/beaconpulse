"use client";

import { motion } from "framer-motion";
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

function ago(iso: string | undefined, now: number | null): string {
  if (!iso) return "never";
  if (now === null) return "";
  const s = Math.max(0, Math.floor((now - Date.parse(iso)) / 1000));
  if (s < 60) return "just now";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 48) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function state(k: ApiKey, now: number | null): { label: string; className: string } | null {
  if (k.revoked_at) {
    return { label: "Revoked", className: "bg-slate-200 text-slate-600 dark:bg-slate-800 dark:text-slate-400" };
  }
  if (k.expires_at && now !== null && Date.parse(k.expires_at) < now) {
    return { label: "Expired", className: "bg-amber-100 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300" };
  }
  return null;
}

/** The secret, shown once. */
function SecretPanel({ created, onDone }: { created: ApiKeyCreated; onDone: () => void }) {
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
            “{created.key.name}” is ready
          </h3>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
            Copy it now and store it somewhere safe. We only keep a hash, so this is the
            one and only time it can be shown — if it is lost, revoke this key and create
            another.
          </p>

          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <input
              readOnly
              value={created.secret}
              onFocus={(e) => e.currentTarget.select()}
              aria-label="Your new API key"
              className="w-full rounded-lg border border-slate-300 bg-slate-50 px-3 py-2 font-mono text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
            />
            <Button variant="secondary" onClick={copy} className="shrink-0">
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>

          <p className="mt-3 text-xs text-slate-500 dark:text-slate-400">
            In GitHub: <span className="font-mono">Settings → Secrets and variables → Actions</span>, as{" "}
            <span className="font-mono">BEACON_API_KEY</span>. See{" "}
            <Link href="/docs/automation" className="text-brand-700 hover:underline dark:text-brand-400">
              the automation guide
            </Link>{" "}
            for a ready-made workflow.
          </p>

          <div className="mt-4">
            <Button onClick={onDone}>Done — I have saved it</Button>
          </div>
        </div>
      </div>
    </Card>
  );
}

function CreateForm({ onCreated, onCancel }: { onCreated: (c: ApiKeyCreated) => void; onCancel: () => void }) {
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
      setError(err instanceof ApiRequestError ? err.message : "Could not create the key");
    }
  };

  return (
    <Card className="p-5">
      <form onSubmit={submit} className="space-y-4">
        <Field label="Name" hint="Name it after whatever will use it, so you know what you are revoking later.">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="github-actions"
            autoFocus
            required
          />
        </Field>

        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Access" hint="Give it the least it needs. A key can never have more access than you.">
            <Select value={role} onChange={(e) => setRole(e.target.value)}>
              <option value="">Same as mine</option>
              <option value="member">Member — read and write monitors</option>
              <option value="viewer">Viewer — read only</option>
            </Select>
          </Field>

          <Field label="Expires after" hint="Optional. A key with an end date is one you cannot forget about.">
            <Select value={expiresInDays} onChange={(e) => setExpiresInDays(e.target.value)}>
              <option value="">Never</option>
              <option value="30">30 days</option>
              <option value="90">90 days</option>
              <option value="365">1 year</option>
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
            {create.isPending ? "Creating…" : "Create key"}
          </Button>
          <Button type="button" variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
        </div>
      </form>
    </Card>
  );
}

export default function ApiKeysPage() {
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

  return (
    <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-6">
      <PageHeader
        title="API keys"
        subtitle="Manage monitors from a script, a pipeline, or your own tooling."
        actions={
          canManage && !showForm && !created ? (
            <Button onClick={() => setShowForm(true)}>
              <PlusIcon className="h-4 w-4" />
              Create key
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
          title="No API keys yet"
          action={
            canManage ? <Button onClick={() => setShowForm(true)}>Create your first key</Button> : undefined
          }
        >
          A key lets a script or a CI pipeline manage your monitors. Commit the domains you
          watch alongside the code that serves them, and keep the two in step.
        </EmptyState>
      ) : (
        <motion.div variants={reveal}>
          <Card className="overflow-hidden p-0">
            <table className="w-full text-left">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs uppercase tracking-wide text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
                <tr>
                  <th className="px-4 py-3 font-semibold">Name</th>
                  <th className="px-4 py-3 font-semibold">Key</th>
                  <th className="px-4 py-3 font-semibold">Access</th>
                  <th className="px-4 py-3 font-semibold">Last used</th>
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
                        {k.role}
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
                                    title: `Revoke “${k.name}”?`,
                                    body: "Anything using this key stops working immediately. This cannot be undone — you would need to create a new key and update whatever uses it.",
                                    confirmLabel: "Revoke key",
                                    danger: true,
                                  })
                                ) {
                                  revoke.mutate(k.id);
                                }
                              }}
                            >
                              <XIcon className="h-3.5 w-3.5" />
                              Revoke
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
          Only owners and admins can create or revoke API keys.
        </p>
      )}
    </motion.div>
  );
}
