-- Revert 0015_device_tokens.

-- Remove any apns channels before restoring the narrower CHECK, or the ADD would
-- fail on existing rows.
DELETE FROM notification_channels WHERE type = 'apns';
ALTER TABLE notification_channels DROP CONSTRAINT IF EXISTS notification_channels_type_check;
ALTER TABLE notification_channels ADD CONSTRAINT notification_channels_type_check
    CHECK (type IN ('telegram', 'slack', 'discord', 'email', 'webhook', 'teams'));

DROP INDEX IF EXISTS ix_device_tokens_user;
DROP INDEX IF EXISTS ix_device_tokens_org;
DROP INDEX IF EXISTS ux_device_tokens_token;
DROP TABLE IF EXISTS device_tokens;
