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
  /** i18n key (relative to the `channels` namespace) for the field label. */
  label: string;
  /** Technical example placeholder (URL/host/port/token) — rendered verbatim. */
  placeholder?: string;
  /** i18n key for an instructional-prose placeholder — localized by the page.
   *  Mutually exclusive with `placeholder`. */
  placeholderKey?: string;
  /** i18n key for the field hint. */
  hint?: string;
  required?: boolean;
  /** Rendered as a password input and — when true — sent as the channel SECRET
   *  (encrypted, write-only) rather than in plaintext config. Exactly one secret
   *  field per type. */
  secret?: boolean;
  /** Render a <select> with these options instead of a text input.
   *  `value` is data; `label` is an i18n key localized by the page. */
  options?: { value: string; label: string }[];
}

export interface ChannelTypeDef {
  value: ChannelTypeValue;
  /** i18n key for the channel type name. */
  label: string;
  /** i18n key for the channel type blurb. */
  blurb: string;
  fields: ChannelField[];
  /** i18n key for the one-line summary of a saved channel (never shows a secret). */
  summaryKey: string;
  /** ICU values the page interpolates into `summaryKey`, when the summary
   *  references a config field. */
  summaryVars?: (c: NotificationChannel) => Record<string, string>;
}

export const CHANNEL_TYPES: ChannelTypeDef[] = [
  {
    value: "slack",
    label: "slack.label",
    blurb: "slack.blurb",
    fields: [
      {
        key: "webhook_url",
        label: "slack.webhookUrlLabel",
        placeholder: "https://hooks.slack.com/services/T…/B…/…",
        secret: true,
        required: true,
        hint: "slack.webhookUrlHint",
      },
    ],
    summaryKey: "slack.summary",
  },
  {
    value: "email",
    label: "email.label",
    blurb: "email.blurb",
    fields: [
      { key: "host", label: "email.hostLabel", placeholder: "smtp.example.com", required: true },
      { key: "port", label: "email.portLabel", placeholder: "587" },
      {
        key: "security",
        label: "email.securityLabel",
        options: [
          { value: "starttls", label: "email.secStarttls" },
          { value: "tls", label: "email.secTls" },
          { value: "none", label: "email.secNone" },
        ],
      },
      { key: "from", label: "email.fromLabel", placeholder: "alerts@example.com", required: true },
      { key: "to", label: "email.toLabel", placeholder: "oncall@example.com, ops@example.com", required: true, hint: "email.toHint" },
      { key: "username", label: "email.usernameLabel", placeholderKey: "email.usernamePlaceholder" },
      { key: "password", label: "email.passwordLabel", placeholder: "••••••••", secret: true },
    ],
    summaryKey: "email.summary",
    summaryVars: (c) => ({ to: c.config.to ?? "—" }),
  },
  {
    value: "webhook",
    label: "webhook.label",
    blurb: "webhook.blurb",
    fields: [
      { key: "url", label: "webhook.urlLabel", placeholder: "https://example.com/hooks/beacon", required: true },
      {
        key: "method",
        label: "webhook.methodLabel",
        options: [
          { value: "POST", label: "POST" },
          { value: "PUT", label: "PUT" },
        ],
      },
      {
        key: "signing_key",
        label: "webhook.signingKeyLabel",
        placeholderKey: "webhook.signingKeyPlaceholder",
        secret: true,
        hint: "webhook.signingKeyHint",
      },
    ],
    summaryKey: "webhook.summary",
    summaryVars: (c) => ({ url: c.config.url ?? "—" }),
  },
  {
    value: "telegram",
    label: "telegram.label",
    blurb: "telegram.blurb",
    fields: [
      { key: "chat_id", label: "telegram.chatIdLabel", placeholder: "123456789", required: true },
      { key: "bot_token", label: "telegram.botTokenLabel", placeholder: "123456:ABC-DEF…", secret: true, required: true },
    ],
    summaryKey: "telegram.summary",
    summaryVars: (c) => ({ chatId: c.config.chat_id ?? "—" }),
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
