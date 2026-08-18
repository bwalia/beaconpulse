-- 0014_platform_settings — operator-tunable pricing, limits and premium access.
--
-- A single-row table (id is pinned to 1) holding platform-GLOBAL configuration the
-- operator edits live from the admin page: the pay-as-you-go rate, per-tier pricing
-- and limits, and the premium email/domain allowlist (accounts that get Pro free).
-- No row is inserted here: the app seeds the row on first start from its built-in
-- defaults (already overlaid with any env baseline), so a fresh install and an
-- upgrade behave identically.
--
-- Values live in this row rather than in code so they can change without a redeploy;
-- the app loads them into an in-memory snapshot that enforcement reads, and reloads
-- on every change.
CREATE TABLE platform_settings (
    id                        SMALLINT     PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    monitor_hours_per_dollar  INT          NOT NULL DEFAULT 5 CHECK (monitor_hours_per_dollar > 0),
    -- Per-tier pricing + limits, as a JSON array of
    -- {plan, price_monthly, max_monitors, min_interval_seconds, monthly_diagnoses}.
    plans                     JSONB        NOT NULL DEFAULT '[]'::jsonb,
    -- Emails and/or bare domains granted the Pro tier for free.
    premium_grants            TEXT[]       NOT NULL DEFAULT '{}',
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_by                UUID
);
