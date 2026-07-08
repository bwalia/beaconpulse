"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  useChannels,
  useCreateChannel,
  useDeleteChannel,
  useSetChannelEnabled,
  useTestChannel,
} from "@/lib/hooks";
import { ApiRequestError } from "@/lib/api";
import { Button, Card, EmptyState, Field, Input, PageHeader, Skeleton } from "@/components/ui";
import type { NotificationChannel } from "@/lib/types";
import { BellIcon, LockIcon, PlusIcon, XIcon } from "@/components/icons";

const schema = z.object({
  name: z.string().min(1, "Name is required"),
  bot_token: z.string().min(1, "Bot token is required"),
  chat_id: z.string().min(1, "Chat ID is required"),
});
type Values = z.infer<typeof schema>;

type Notice = { kind: "ok" | "err"; text: string } | null;

export default function NotificationsPage() {
  const { data, isLoading } = useChannels();
  const [showForm, setShowForm] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Notifications"
        subtitle="Get alerted on Telegram the moment something goes down."
        actions={
          <Button onClick={() => setShowForm((v) => !v)}>
            {showForm ? <XIcon className="h-4 w-4" /> : <PlusIcon className="h-4 w-4" />}
            {showForm ? "Close" : "Add Telegram"}
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

      {showForm && <CreateTelegramForm onDone={() => setShowForm(false)} setNotice={setNotice} />}

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
              Add Telegram
            </Button>
          }
        >
          Connect a Telegram channel and Beacon will message you the moment a monitor goes down — enriched with AI
          triage when enabled.
        </EmptyState>
      ) : (
        <div className="space-y-3">
          {data.data.map((c) => (
            <ChannelRow key={c.id} channel={c} setNotice={setNotice} />
          ))}
        </div>
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

  return (
    <Card className="flex items-center justify-between">
      <div>
        <div className="flex items-center gap-2">
          <span className="font-medium">{channel.name}</span>
          <span className="rounded-full bg-brand-50 px-2 py-0.5 text-xs uppercase text-brand-700 dark:bg-brand-900/30 dark:text-brand-300">
            {channel.type}
          </span>
          {!channel.enabled && (
            <span className="rounded-full bg-slate-200 px-2 py-0.5 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-400">
              paused
            </span>
          )}
        </div>
        <p className="mt-0.5 inline-flex items-center gap-1 text-xs text-slate-500 dark:text-slate-400">
          chat: {channel.config.chat_id} · token{" "}
          {channel.has_secret ? (
            <span className="inline-flex items-center gap-1">
              stored <LockIcon className="h-3 w-3" />
            </span>
          ) : (
            "missing"
          )}
        </p>
      </div>
      <div className="flex gap-2">
        <Button
          variant="secondary"
          disabled={test.isPending}
          onClick={async () => {
            try {
              await test.mutateAsync(channel.id);
              setNotice({ kind: "ok", text: `Test message sent to "${channel.name}" — check Telegram.` });
            } catch (e) {
              setNotice({
                kind: "err",
                text: e instanceof ApiRequestError ? e.message : "Test failed",
              });
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
          onClick={() => {
            if (confirm(`Delete channel "${channel.name}"?`)) del.mutate(channel.id);
          }}
        >
          Delete
        </Button>
      </div>
    </Card>
  );
}

function CreateTelegramForm({
  onDone,
  setNotice,
}: {
  onDone: () => void;
  setNotice: (n: Notice) => void;
}) {
  const createChannel = useCreateChannel();
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<Values>({ resolver: zodResolver(schema) });

  const onSubmit = async (values: Values) => {
    try {
      await createChannel.mutateAsync({
        name: values.name,
        type: "telegram",
        config: { chat_id: values.chat_id },
        secret: values.bot_token,
      });
      setNotice({ kind: "ok", text: "Telegram channel added. Use “Send test” to verify it." });
      onDone();
    } catch (e) {
      setNotice({ kind: "err", text: e instanceof ApiRequestError ? e.message : "Failed to add channel" });
    }
  };

  return (
    <Card>
      <div className="mb-4 rounded-lg bg-slate-50 p-3 text-xs text-slate-500 dark:bg-slate-800/50 dark:text-slate-400">
        <p className="font-medium text-slate-600 dark:text-slate-300">How to set up a Telegram bot:</p>
        <ol className="mt-1 list-decimal space-y-0.5 pl-4">
          <li>Message <span className="font-mono">@BotFather</span>, send <span className="font-mono">/newbot</span>, copy the token.</li>
          <li>Start a chat with your new bot (send it any message).</li>
          <li>Message <span className="font-mono">@userinfobot</span> to get your chat ID.</li>
        </ol>
      </div>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <Field label="Channel name" error={errors.name?.message}>
          <Input placeholder="Ops Telegram" {...register("name")} />
        </Field>
        <Field label="Bot token" error={errors.bot_token?.message}>
          <Input type="password" placeholder="123456:ABC-DEF…" {...register("bot_token")} />
        </Field>
        <Field label="Chat ID" error={errors.chat_id?.message}>
          <Input placeholder="123456789" {...register("chat_id")} />
        </Field>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving…" : "Save channel"}
        </Button>
      </form>
    </Card>
  );
}
