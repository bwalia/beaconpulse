DROP INDEX IF EXISTS ux_monitors_github_token_hash;

ALTER TABLE monitors
    DROP COLUMN IF EXISTS github_token_prefix,
    DROP COLUMN IF EXISTS github_token_hash;

-- Restore the pre-github_actions type CHECK. Any github_actions rows must be gone
-- first (the down migration assumes the feature was rolled back cleanly).
ALTER TABLE monitors DROP CONSTRAINT monitors_type_check;
ALTER TABLE monitors ADD CONSTRAINT monitors_type_check CHECK (type IN (
    'http', 'https', 'tcp', 'icmp', 'ssl', 'dns', 'domain',
    'api', 'server', 'kubernetes', 'health',
    'grafana', 'prometheus', 'gatus',
    'heartbeat'));
