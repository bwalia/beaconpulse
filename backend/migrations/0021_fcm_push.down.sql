DELETE FROM notification_channels WHERE type = 'fcm';
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams', 'apns', 'webpush'));
