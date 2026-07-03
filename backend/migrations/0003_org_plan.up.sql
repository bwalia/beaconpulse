-- 0003_org_plan — per-tenant plan for quota enforcement.
-- The plan drives limits (max monitors, minimum check interval) applied when a
-- tenant creates or updates monitors, protecting the shared Prometheus/Blackbox
-- from a single noisy tenant. Limit values live in code (internal/domain/plan)
-- so they can be tuned without a migration.
ALTER TABLE organizations
    ADD COLUMN plan TEXT NOT NULL DEFAULT 'free'
        CHECK (plan IN ('free', 'starter', 'pro'));
