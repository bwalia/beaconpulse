-- 0016_github_actions — push-based CI monitors driven by a GitHub Action.
--
-- A github_actions monitor is, like a heartbeat, one Beacon does NOT probe. The
-- customer adds the "Beacon Notify" Action to a workflow; on completion the Action
-- POSTs the run's result to a capability URL. A failed run alerts through the org's
-- normal notification channels. This is push, not poll: no GitHub token is stored
-- and the GitHub API is never called, so there is nothing to rate-limit or leak.
--
-- It reuses the monitors table (org, project, alert routing already live there); a
-- new type value plus one hashed credential column carry the difference.

-- Admit the new type. Re-add the CHECK with 'github_actions' included, preserving
-- every prior value so existing rows stay valid (same approach as 0006).
ALTER TABLE monitors DROP CONSTRAINT monitors_type_check;
ALTER TABLE monitors ADD CONSTRAINT monitors_type_check CHECK (type IN (
    'http', 'https', 'tcp', 'icmp', 'ssl', 'dns', 'domain',
    'api', 'server', 'kubernetes', 'health',
    'grafana', 'prometheus', 'gatus',
    'heartbeat', 'github_actions'));

ALTER TABLE monitors
    -- SHA-256 (hex) of the ingest token. We only ever VERIFY an inbound call, never
    -- read the token back, so we store just its hash — the database (and any backup)
    -- never holds a usable credential. NULL for every non-github_actions monitor.
    ADD COLUMN github_token_hash TEXT,
    -- The first characters of the token, kept in clear so a user can recognise which
    -- token a monitor uses after the one-time reveal. Never enough to be useful alone.
    ADD COLUMN github_token_prefix TEXT;

-- The ingest endpoint resolves a monitor by token hash on every (unauthenticated)
-- request, so this makes that an O(1) unique probe. Partial: only github monitors
-- carry a hash, and a soft-deleted monitor's token must not resolve.
CREATE UNIQUE INDEX ux_monitors_github_token_hash ON monitors (github_token_hash)
    WHERE github_token_hash IS NOT NULL AND deleted_at IS NULL;
