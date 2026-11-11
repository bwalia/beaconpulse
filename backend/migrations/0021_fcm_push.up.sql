-- 0021_fcm_push — Android push via Firebase Cloud Messaging.
-- Android tokens are device_tokens rows with platform 'android' (already allowed
-- since 0015); the fcm notifier fans an org's alerts out to them, like apns.
-- Only the channel type CHECK needs the new value.

ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams', 'apns', 'webpush', 'fcm'));
