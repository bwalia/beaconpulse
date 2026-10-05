-- 0020_web_push — browser (PWA) push subscriptions.
-- A web push subscription is stored as a device_tokens row with platform 'web':
-- token holds the subscription endpoint URL (the push service's capability URL),
-- and web_p256dh / web_auth hold the browser's message-encryption keys (RFC 8291).
-- The webpush notifier fans an org's alerts out to every such row, like apns.

ALTER TABLE device_tokens DROP CONSTRAINT IF EXISTS device_tokens_platform_check;
ALTER TABLE device_tokens ADD CONSTRAINT device_tokens_platform_check
    CHECK (platform IN ('ios', 'android', 'web'));

-- Endpoint URLs are longer than APNs tokens (WNS's run to several hundred chars).
ALTER TABLE device_tokens DROP CONSTRAINT IF EXISTS device_tokens_token_check;
ALTER TABLE device_tokens ADD CONSTRAINT device_tokens_token_check
    CHECK (length(token) BETWEEN 1 AND 2048);

ALTER TABLE device_tokens
    ADD COLUMN web_p256dh TEXT,
    ADD COLUMN web_auth   TEXT;

ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams', 'apns', 'webpush'));
