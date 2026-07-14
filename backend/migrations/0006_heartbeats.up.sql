-- 0006_heartbeats — push-based "dead man's switch" monitors.
--
-- A heartbeat is a monitor that Beacon does NOT probe. Instead the customer's job
-- (cron, backup, ETL) pings a capability URL on success; if no ping arrives within
-- interval + grace, Beacon alerts. This is the one failure mode black-box probing
-- cannot see: a probe says the site answers, never that the nightly backup
-- silently stopped.
--
-- A heartbeat reuses the monitors table (it already has org, project, interval,
-- alert routing) rather than a parallel object. Three columns and one new type
-- value carry the difference.

-- Admit the new type. The inline CHECK from 0001 is named monitors_type_check by
-- Postgres; drop and re-add it with 'heartbeat' included, preserving every prior
-- value so existing rows stay valid.
ALTER TABLE monitors DROP CONSTRAINT monitors_type_check;
ALTER TABLE monitors ADD CONSTRAINT monitors_type_check CHECK (type IN (
    'http', 'https', 'tcp', 'icmp', 'ssl', 'dns', 'domain',
    'api', 'server', 'kubernetes', 'health',
    'grafana', 'prometheus', 'gatus',
    'heartbeat'));

ALTER TABLE monitors
    -- The capability token embedded in the ping URL. High-entropy and opaque (NOT
    -- the monitor's UUID, which is enumerable and would leak the internal id).
    -- NULL for every non-heartbeat monitor.
    ADD COLUMN ping_token TEXT,
    -- When the last successful ping was received. Seeded to created_at on a new
    -- heartbeat so a monitor that is created and never wired up still alerts —
    -- silence from the start is itself a failure worth surfacing.
    ADD COLUMN last_ping_at TIMESTAMPTZ,
    -- Slack beyond the interval before a missed ping alerts. Absorbs a slightly
    -- late cron. Defaults (in code) to one interval, floored at 30s.
    ADD COLUMN grace_seconds INTEGER NOT NULL DEFAULT 60
        CHECK (grace_seconds BETWEEN 0 AND 86400);

-- The ping endpoint looks a monitor up by token on every request, unauthenticated,
-- so this index makes that an O(1) unique probe. Partial: only heartbeats have a
-- token, and a soft-deleted monitor's token must not resolve.
CREATE UNIQUE INDEX ux_monitors_ping_token ON monitors (ping_token)
    WHERE ping_token IS NOT NULL AND deleted_at IS NULL;
