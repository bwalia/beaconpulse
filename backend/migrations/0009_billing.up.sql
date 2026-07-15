-- 0009_billing — pay-as-you-go credit and Stripe subscriptions.
--
-- Two ways to pay, both through Stripe:
--   * Pay-as-you-go: buy any amount; it converts to a balance of MONITOR-SECONDS
--     (credit_seconds). A worker deducts one second of credit per enabled monitor
--     per second, so more domains burn the balance faster. At zero, the org falls
--     back to the Free tier.
--   * Subscription: a recurring Stripe subscription for the Starter/Pro tiers;
--     subscription_status mirrors Stripe and gates whether the tier is active.
--
-- The effective plan (what limits actually apply) is computed in code from these
-- columns — subscribed tier if active, else pay-as-you-go if credit remains, else
-- Free — so it is never stored stale.

ALTER TABLE organizations
    -- Remaining pay-as-you-go balance, in monitor-seconds. Never negative.
    ADD COLUMN credit_seconds BIGINT NOT NULL DEFAULT 0 CHECK (credit_seconds >= 0),
    -- The Stripe Customer this org maps to (created lazily at first checkout).
    ADD COLUMN stripe_customer_id TEXT,
    -- Stripe subscription status: active, trialing, past_due, canceled, … NULL when
    -- the org has never subscribed. Only active/trialing grant the subscribed tier.
    ADD COLUMN subscription_status TEXT,
    -- When the current subscription period ends (for display / grace handling).
    ADD COLUMN subscription_current_period_end TIMESTAMPTZ;

CREATE UNIQUE INDEX ux_organizations_stripe_customer
    ON organizations (stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL AND deleted_at IS NULL;

-- billing_events is the idempotency + audit ledger for processed Stripe webhooks.
-- The unique stripe_event_id means a webhook retry (Stripe delivers at-least-once)
-- inserts-conflicts and is skipped, so credit is never granted twice.
CREATE TABLE billing_events (
    id                   UUID PRIMARY KEY,
    stripe_event_id      TEXT        NOT NULL,
    org_id               UUID        REFERENCES organizations (id) ON DELETE CASCADE,
    type                 TEXT        NOT NULL,
    amount_cents         BIGINT,
    credit_added_seconds BIGINT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_billing_events_stripe_event ON billing_events (stripe_event_id);
CREATE INDEX ix_billing_events_org ON billing_events (org_id, created_at DESC);
