-- 0019_email_verification — record when a user proved they own their email address.
--
-- Password sign-ups confirm by clicking an emailed link. Google, Apple and SSO
-- sign-ins are verified by the identity provider (sign-in already refuses an
-- unverified provider email), so those accounts are marked verified at sign-in —
-- and backfilled here. Verification gates nothing; it tells us resets and alerts
-- will actually reach the person, and drives the "confirm your email" banner.
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

UPDATE users SET email_verified_at = created_at
 WHERE google_sub IS NOT NULL OR oidc_sub IS NOT NULL;
