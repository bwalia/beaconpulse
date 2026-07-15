DROP TABLE IF EXISTS billing_events;
DROP INDEX IF EXISTS ux_organizations_stripe_customer;
ALTER TABLE organizations
    DROP COLUMN IF EXISTS subscription_current_period_end,
    DROP COLUMN IF EXISTS subscription_status,
    DROP COLUMN IF EXISTS stripe_customer_id,
    DROP COLUMN IF EXISTS credit_seconds;
