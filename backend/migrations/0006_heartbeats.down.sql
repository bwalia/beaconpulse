DROP INDEX IF EXISTS ux_monitors_ping_token;

ALTER TABLE monitors
    DROP COLUMN IF EXISTS grace_seconds,
    DROP COLUMN IF EXISTS last_ping_at,
    DROP COLUMN IF EXISTS ping_token;

-- Restore the pre-heartbeat type CHECK. Any heartbeat rows must be gone first
-- (the down migration assumes the feature was rolled back cleanly).
ALTER TABLE monitors DROP CONSTRAINT monitors_type_check;
ALTER TABLE monitors ADD CONSTRAINT monitors_type_check CHECK (type IN (
    'http', 'https', 'tcp', 'icmp', 'ssl', 'dns', 'domain',
    'api', 'server', 'kubernetes', 'health',
    'grafana', 'prometheus', 'gatus'));
