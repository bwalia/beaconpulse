DROP TABLE IF EXISTS platform_secrets;
DELETE FROM notification_channels WHERE type = 'webpush';
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams', 'apns'));

DELETE FROM device_tokens WHERE platform = 'web';
ALTER TABLE device_tokens DROP COLUMN web_p256dh, DROP COLUMN web_auth;
ALTER TABLE device_tokens DROP CONSTRAINT IF EXISTS device_tokens_token_check;
ALTER TABLE device_tokens ADD CONSTRAINT device_tokens_token_check
    CHECK (length(token) BETWEEN 1 AND 512);
ALTER TABLE device_tokens DROP CONSTRAINT IF EXISTS device_tokens_platform_check;
ALTER TABLE device_tokens ADD CONSTRAINT device_tokens_platform_check
    CHECK (platform IN ('ios', 'android'));
