ALTER TABLE users DROP CONSTRAINT IF EXISTS ck_users_auth_method;
ALTER TABLE users ADD CONSTRAINT ck_users_auth_method
    CHECK (password_hash IS NOT NULL OR google_sub IS NOT NULL);
DROP INDEX IF EXISTS ux_users_oidc_sub;
ALTER TABLE users DROP COLUMN IF EXISTS oidc_sub;
