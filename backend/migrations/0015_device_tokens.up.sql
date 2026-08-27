-- 0015_device_tokens — per-user push-notification device tokens.
-- The mobile app registers a device's push token here after the user signs in;
-- the apns notifier fans an organization's alerts out to every token its members
-- have enrolled. The token is the credential the push provider (APNs) trusts, so
-- it is stored opaquely and never returned by the API.

CREATE TABLE device_tokens (
    id            UUID PRIMARY KEY,
    org_id        UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id       UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    platform      TEXT        NOT NULL DEFAULT 'ios'
                      CHECK (platform IN ('ios', 'android')),
    token         TEXT        NOT NULL CHECK (length(token) BETWEEN 1 AND 512),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per device token. Re-registering the same token upserts
-- (ON CONFLICT (token)) so a device that reinstalls or refreshes its token does
-- not accumulate duplicate rows.
CREATE UNIQUE INDEX ux_device_tokens_token ON device_tokens (token);
-- Alert fan-out reads every token for an org.
CREATE INDEX ix_device_tokens_org ON device_tokens (org_id);
-- A user signing out (or being removed) drops their own device(s).
CREATE INDEX ix_device_tokens_user ON device_tokens (user_id);

-- Allow the 'apns' channel type now that its notifier exists. The original CHECK
-- is unnamed (Postgres auto-names it <table>_<column>_check); drop and recreate
-- it with the extra value.
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams', 'apns'));
