-- Ledger of AI diagnoses, one row per run that actually delivered an analysis.
--
-- It exists to count: a subscribed org gets a fixed number per calendar month, and
-- this is what that number is counted from. Pay-as-you-go orgs are metered against
-- their credit instead, but their runs are recorded here too — the same question
-- ("what did we spend this on?") gets asked about both, and answering it from one
-- table beats reconstructing it from two.
--
-- Only successful runs are recorded. A diagnosis whose model failed is refunded and
-- left out, so a quota is never spent on an answer nobody received.
CREATE TABLE IF NOT EXISTS diagnose_runs (
    id              UUID PRIMARY KEY,
    org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    monitor_id      UUID,
    -- What it cost, in monitor-seconds. Zero for a subscribed org, which paid a flat
    -- fee and spends quota rather than credit. Kept per row rather than derived from
    -- today's price, so an old run still says what it actually cost.
    credit_seconds  BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The quota query is always "this org, since the start of the month", so the index
-- leads with org_id and carries created_at to answer it from the index alone.
CREATE INDEX IF NOT EXISTS ix_diagnose_runs_org_created
    ON diagnose_runs (org_id, created_at DESC);
