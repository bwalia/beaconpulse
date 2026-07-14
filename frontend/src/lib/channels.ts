// Channel-type descriptors: the single source of truth for what each notification
// channel needs and how it is summarised.
//
// The form and the channel list are both generated from this table, so adding a
// new channel type (or a field) is a data change here — not new JSX in two places
// that can drift apart. It mirrors the backend's per-type validation
// (domain/notification/service.go): keep the required fields in sync.

import type { NotificationChannel } from "@/lib/types";

export type ChannelTypeValue = "telegram" | "slack" | "email" | "webhook";

export interface ChannelField {
  /** Form + payload key. */
  key: string;
  label: string;
  placeholder?: string;
  hint?: string;
  required?: boolean;
  /** Rendered as a password input and — when true — sent as the channel SECRET
   *  (encrypted, write-only) rather than in plaintext config. Exactly one secret
   *  field per type. */
  secret?: boolean;
  /** Render a <select> with these options instead of a text input. */
  options?: { value: string; label: string }[];
}

export interface ChannelTypeDef {
  value: ChannelTypeValue;
  label: string;
  blurb: string;
  fields: ChannelField[];
  /** One-line summary of a saved channel for the list row (never shows a secret). */
  summary: (c: NotificationChannel) => string;
}

export const CHANNEL_TYPES: ChannelTypeDef[] = [
  {
    value: "slack",
    label: "Slack",
    blurb:
      "Create an Incoming Webhook in Slack (Apps → Incoming Webhooks), pick a channel, and paste the URL below. The URL is stored encrypted.",
    fields: [
      {
        key: "webhook_url",
        label: "Incoming webhook URL",
        placeholder: "https://hooks.slack.com/services/T…/B…/…",
        secret: true,
        required: true,
        hint: "Slack → Apps → Incoming Webhooks → Add to a channel.",
      },
    ],
    summary: () => "posts to a Slack channel",
  },
  {
    value: "email",
    label: "Email",
    blurb: "Deliver alerts over your own SMTP server. The password is stored encrypted.",
    fields: [
      { key: "host", label: "SMTP host", placeholder: "smtp.example.com", required: true },
      { key: "port", label: "Port", placeholder: "587" },
      {
        key: "security",
        label: "Security",
        options: [
          { value: "starttls", label: "STARTTLS (587)" },
          { value: "tls", label: "TLS / SSL (465)" },
          { value: "none", label: "None (internal relay)" },
        ],
      },
      { key: "from", label: "From address", placeholder: "alerts@example.com", required: true },
      { key: "to", label: "Recipients", placeholder: "oncall@example.com, ops@example.com", required: true, hint: "Comma-separated." },
      { key: "username", label: "SMTP username", placeholder: "(optional — leave blank for no auth)" },
      { key: "password", label: "SMTP password", placeholder: "••••••••", secret: true },
    ],
    summary: (c) => `to ${c.config.to ?? "—"}`,
  },
  {
    value: "webhook",
    label: "Webhook",
    blurb:
      "POST a signed JSON payload to your own endpoint. Set a signing key to verify requests (HMAC-SHA256, Stripe-style).",
    fields: [
      { key: "url", label: "Endpoint URL", placeholder: "https://example.com/hooks/beacon", required: true },
      {
        key: "method",
        label: "Method",
        options: [
          { value: "POST", label: "POST" },
          { value: "PUT", label: "PUT" },
        ],
      },
      {
        key: "signing_key",
        label: "Signing key",
        placeholder: "(optional) whsec_…",
        secret: true,
        hint: "If set, requests carry X-Beacon-Signature you can verify.",
      },
    ],
    summary: (c) => `POST ${c.config.url ?? "—"}`,
  },
  {
    value: "telegram",
    label: "Telegram",
    blurb:
      "Message @BotFather → /newbot to get a token, start a chat with your bot, then message @userinfobot for your chat ID.",
    fields: [
      { key: "chat_id", label: "Chat ID", placeholder: "123456789", required: true },
      { key: "bot_token", label: "Bot token", placeholder: "123456:ABC-DEF…", secret: true, required: true },
    ],
    summary: (c) => `chat ${c.config.chat_id ?? "—"}`,
  },
];

export function channelTypeDef(value: string): ChannelTypeDef | undefined {
  return CHANNEL_TYPES.find((t) => t.value === value);
}

/**
 * Split a flat form-values object into the API's config/secret shape, using the
 * type descriptor to decide which key is the secret. Empty optional values are
 * dropped so we never store an empty string where "unset" is meaningful.
 */
export function toChannelPayload(
  def: ChannelTypeDef,
  name: string,
  values: Record<string, string>,
): { name: string; type: ChannelTypeValue; config: Record<string, string>; secret: string } {
  const config: Record<string, string> = {};
  let secret = "";
  for (const f of def.fields) {
    const v = (values[f.key] ?? "").trim();
    if (f.secret) {
      secret = v;
    } else if (v !== "") {
      config[f.key] = v;
    }
  }
  return { name: name.trim(), type: def.value, config, secret };
}
