-- 0017_github_runs — per-run history for github_actions monitors.
--
-- Until now a github_actions monitor kept only its LATEST outcome (monitors.last_status):
-- a green run overwrote the red one before it, so the dashboard could never show "this
-- failed on Tuesday, passed on Wednesday". This append-only log keeps every reported run,
-- so the UI can draw a real history strip and the owner can see the trend, not just the
-- last word.
--
-- One row per ingested run (the "Beacon Notify" Action POSTs once per run). All columns
-- but status/created_at are best-effort context straight off the Action's payload.
CREATE TABLE github_runs (
    id          BIGSERIAL PRIMARY KEY,
    monitor_id  UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    -- our classification of the run, same vocabulary as monitors.last_status.
    status      TEXT NOT NULL,
    -- the raw GitHub conclusion (success / failure / cancelled / timed_out / …), kept
    -- verbatim so a run we don't alert on (cancelled, skipped) still reads truthfully.
    conclusion  TEXT NOT NULL DEFAULT '',
    workflow    TEXT NOT NULL DEFAULT '',
    run_number  TEXT NOT NULL DEFAULT '',
    run_attempt TEXT NOT NULL DEFAULT '',
    branch      TEXT NOT NULL DEFAULT '',
    sha         TEXT NOT NULL DEFAULT '',
    actor       TEXT NOT NULL DEFAULT '',
    event_name  TEXT NOT NULL DEFAULT '',
    run_url     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The only read is "this monitor's recent runs, newest first".
CREATE INDEX ix_github_runs_monitor ON github_runs (monitor_id, created_at DESC);

-- ponytail: no retention — CI volume is low, and Postgres eats millions of these rows.
-- Add a periodic "keep last N per monitor" prune if a busy tenant ever makes it grow.
