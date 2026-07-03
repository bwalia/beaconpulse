-- 0002_notifications — notification channels.
-- A channel is a delivery destination (Telegram, Slack, …). Non-secret settings
-- live in `config` (JSONB); the sensitive credential (e.g. a Telegram bot token)
-- is stored in `secret_encrypted` as AES-256-GCM ciphertext and is never
-- returned by the API.

CREATE TABLE notification_channels (
    id                UUID PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name              TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    type              TEXT        NOT NULL
                          CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams')),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    config            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    secret_encrypted  TEXT,
    created_by        UUID        REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);
CREATE INDEX ix_notification_channels_org ON notification_channels (org_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_notification_channels_enabled ON notification_channels (enabled) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_notification_channels_updated_at BEFORE UPDATE ON notification_channels
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
