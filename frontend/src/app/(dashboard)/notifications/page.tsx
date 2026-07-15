"use client";

import { useState } from "react";
import { motion } from "framer-motion";

import {
  useChannels,
  useCreateChannel,
  useDeleteChannel,
  useSetChannelEnabled,
  useTestChannel,
} from "@/lib/hooks";
import { useRevealVariants, useStaggerVariants } from "@/lib/motion";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Field, Input, PageHeader, Skeleton } from "@/components/ui";
import { useConfirm } from "@/components/confirm";
import type { NotificationChannel } from "@/lib/types";
import { BellIcon, LockIcon, PlusIcon, XIcon } from "@/components/icons";
import {
  CHANNEL_TYPES,
  channelTypeDef,
  toChannelPayload,
  type ChannelTypeDef,
} from "@/lib/channels";

type Notice = { kind: "ok" | "err"; text: string } | null;

export default function NotificationsPage() {
  const { data, isLoading } = useChannels();
  const [showForm, setShowForm] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);
  const reveal = useRevealVariants();
  const stagger = useStaggerVariants(0.05);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Notifications"
        subtitle="Get alerted the moment something goes down — on Slack, email, a webhook or Telegram."
        actions={
          <Button onClick={() => setShowForm((v) => !v)}>
            {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
            {showForm ? "Close" : "Add channel"}
          </Button>
        }
      />

      {notice && (
        <div
          role={notice.kind === "ok" ? "status" : "alert"}
          className={`rounded-lg px-4 py-2 text-sm font-medium ${
            notice.kind === "ok"
              ? "bg-emerald-50 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200"
              : "bg-red-50 text-red-800 dark:bg-red-900/30 dark:text-red-200"
          }`}
        >
          {notice.text}
        </div>
      )}

      {showForm && <CreateChannelForm onDone={() => setShowForm(false)} setNotice={setNotice} />}

      {isLoading ? (
        <div className="space-y-3">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      ) : !data?.data.length ? (
        <EmptyState
          icon={<BellIcon className="h-5 w-5" />}
          title="No channels yet"
          action={
            <Button onClick={() => setShowForm(true)}>
              <PlusIcon className="h-4 w-4" />
              Add channel
            </Button>
          }
        >
          Connect Slack, email, a webhook or Telegram and Beacon Pulse will alert you the moment a monitor
          goes down — enriched with AI triage when enabled.
        </EmptyState>
      ) : (
        <motion.div initial="hidden" animate="show" variants={stagger} className="space-y-3">
          {data.data.map((c) => (
            <motion.div key={c.id} variants={reveal}>
              <ChannelRow channel={c} setNotice={setNotice} />
            </motion.div>
          ))}
        </motion.div>
      )}
    </div>
  );
}

function ChannelRow({
  channel,
  setNotice,
}: {
  channel: NotificationChannel;
  setNotice: (n: Notice) => void;
}) {
  const test = useTestChannel();
  const setEnabled = useSetChannelEnabled();
  const del = useDeleteChannel();
  const confirm = useConfirm();

  const def = channelTypeDef(channel.type);

  return (
    <Card className="flex flex-wrap items-center justify-between gap-3">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium">{channel.name}</span>
          <span className="rounded-full bg-brand-50 px-2 py-0.5 text-xs uppercase text-brand-700 dark:bg-brand-900/30 dark:text-brand-300">
            {def?.label ?? channel.type}
          </span>
          {!channel.enabled && (
            <span className="rounded-full bg-slate-200 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400">
              paused
            </span>
          )}
        </div>
        <p className="mt-0.5 inline-flex items-center gap-1.5 truncate text-xs text-slate-500 dark:text-slate-400">
          <span className="truncate font-mono">{def?.summary(channel) ?? channel.type}</span>
          {channel.has_secret && (
            <span className="inline-flex shrink-0 items-center gap-1">
              · secret <LockIcon className="h-3 w-3" />
            </span>
          )}
        </p>
      </div>
      <div className="flex shrink-0 gap-2">
        <Button
          variant="secondary"
          disabled={test.isPending}
          onClick={async () => {
            try {
              await test.mutateAsync(channel.id);
              setNotice({ kind: "ok", text: `Test sent to "${channel.name}".` });
            } catch (e) {
              setNotice({ kind: "err", text: e instanceof ApiRequestError ? e.message : "Test failed" });
            }
          }}
        >
          {test.isPending ? "Sending…" : "Send test"}
        </Button>
        <Button
          variant="secondary"
          disabled={setEnabled.isPending}
          onClick={() => setEnabled.mutate({ id: channel.id, enabled: !channel.enabled })}
        >
          {channel.enabled ? "Pause" : "Resume"}
        </Button>
        <Button
          variant="danger"
          disabled={del.isPending}
          onClick={async () => {
            if (
              await confirm({
                title: `Delete “${channel.name}”?`,
                body: "Alerts will no longer be delivered to this channel.",
                confirmLabel: "Delete channel",
                danger: true,
              })
            ) {
              del.mutate(channel.id);
            }
          }}
        >
          Delete
        </Button>
      </div>
    </Card>
  );
}

/**
 * One form for every channel type. Fields are rendered from the selected type's
 * descriptor, so this component never mentions Slack/SMTP/etc. by name — adding a
 * channel type is a change to CHANNEL_TYPES, not to this form.
 */
function CreateChannelForm({
  onDone,
  setNotice,
}: {
  onDone: () => void;
  setNotice: (n: Notice) => void;
}) {
  const createChannel = useCreateChannel();
  const [def, setDef] = useState<ChannelTypeDef>(CHANNEL_TYPES[0]);
  const [name, setName] = useState("");
  const [values, setValues] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  function selectType(v: string) {
    const next = CHANNEL_TYPES.find((t) => t.value === v);
    if (next) {
      setDef(next);
      setValues({}); // fields differ per type; start clean
      setErrors({});
    }
  }

  function setField(key: string, v: string) {
    setValues((prev) => ({ ...prev, [key]: v }));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    // Client-side required check — the backend re-validates authoritatively and
    // returns field errors, which we surface below.
    const errs: Record<string, string> = {};
    if (!name.trim()) errs.name = "Name is required";
    for (const f of def.fields) {
      if (f.required && !(values[f.key] ?? "").trim()) errs[f.key] = `${f.label} is required`;
    }
    setErrors(errs);
    if (Object.keys(errs).length) return;

    setSubmitting(true);
    try {
      await createChannel.mutateAsync(toChannelPayload(def, name, values));
      setNotice({ kind: "ok", text: `${def.label} channel added. Use “Send test” to verify it.` });
      onDone();
    } catch (e) {
      setNotice({ kind: "err", text: e instanceof ApiRequestError ? e.message : "Failed to add channel" });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Card>
      {/* Type selector */}
      <div role="group" aria-label="Channel type" className="mb-4 flex flex-wrap gap-2">
        {CHANNEL_TYPES.map((t) => (
          <button
            key={t.value}
            type="button"
            onClick={() => selectType(t.value)}
            aria-pressed={def.value === t.value}
            className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors motion-reduce:transition-none ${
              def.value === t.value
                ? "border-brand-600 bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-300"
                : "border-slate-200 text-slate-600 hover:border-slate-300 dark:border-slate-700 dark:text-slate-300"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <p className="mb-4 rounded-lg bg-slate-50 p-3 text-xs text-slate-500 dark:bg-slate-800/50 dark:text-slate-400">
        {def.blurb}
      </p>

      <form onSubmit={submit} className="space-y-4" noValidate>
        <Field label="Channel name" error={errors.name}>
          <Input
            placeholder={`${def.label} — on-call`}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>

        {def.fields.map((f) => (
          <Field key={f.key} label={f.label} hint={f.hint} error={errors[f.key]}>
            {f.options ? (
              <select
                value={values[f.key] ?? f.options[0].value}
                onChange={(e) => setField(f.key, e.target.value)}
                className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-base text-slate-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
              >
                {f.options.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            ) : (
              <Input
                type={f.secret ? "password" : "text"}
                autoComplete={f.secret ? "new-password" : "off"}
                placeholder={f.placeholder}
                value={values[f.key] ?? ""}
                onChange={(e) => setField(f.key, e.target.value)}
              />
            )}
          </Field>
        ))}

        <Button type="submit" disabled={submitting}>
          {submitting ? "Saving…" : "Save channel"}
        </Button>
      </form>
    </Card>
  );
}
