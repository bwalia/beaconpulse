-- 0007_maintenance_windows — planned-downtime windows.
--
-- A maintenance window declares "expect this to be down, on purpose, during this
-- period." It does two things: it SUPPRESSES alert notifications for the covered
-- monitors while active (checked once in the Dispatcher, so it covers probed
-- monitors and heartbeats uniformly), and it RELABELS those monitors on the public
-- status page as "under maintenance" instead of "down" — so a routine deploy no
-- longer pages the on-call rotation or screams "Major outage" at the customer's
-- own users.
--
-- Suppression is deliberately NOT an Alertmanager silence: keeping it here means
-- one source of truth, and a suppressed alert is recorded (audited/metered), not
-- silently vanished.

CREATE TABLE maintenance_windows (
    id           UUID PRIMARY KEY,
    org_id       UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    title        TEXT        NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    description  TEXT        NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL,
    -- One model, three grains. 'org' covers every monitor in the org; 'project'
    -- covers the monitors in the listed projects; 'monitor' covers the listed
    -- monitors. Recurrence (RRULE) is intentionally deferred to v2 — one-off only.
    scope        TEXT        NOT NULL CHECK (scope IN ('org', 'project', 'monitor')),
    -- Project ids (scope='project') or monitor ids (scope='monitor'); empty for
    -- 'org'. Never NULL — an empty array is the org-wide case.
    scope_ids    UUID[]      NOT NULL DEFAULT '{}',
    created_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    updated_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,

    -- A window that ends before it starts covers nothing and is a data-entry bug.
    CONSTRAINT maintenance_windows_range_valid CHECK (ends_at > starts_at),
    -- Scope and its id list must agree: org carries no ids; project/monitor need
    -- at least one, else the window silently covers nothing.
    CONSTRAINT maintenance_windows_scope_ids_agree CHECK (
        (scope = 'org'     AND cardinality(scope_ids) = 0) OR
        (scope <> 'org'    AND cardinality(scope_ids) > 0)
    )
);

-- The hot path is "active windows for this org right now": org equality + a
-- starts_at/ends_at range straddling T. Partial on the live rows only.
CREATE INDEX ix_maintenance_windows_active
    ON maintenance_windows (org_id, starts_at, ends_at)
    WHERE deleted_at IS NULL;

-- Membership tests (id = ANY(scope_ids)) can use containment against a GIN index.
CREATE INDEX ix_maintenance_windows_scope_ids
    ON maintenance_windows USING GIN (scope_ids);

CREATE TRIGGER trg_maintenance_windows_updated_at BEFORE UPDATE ON maintenance_windows
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
