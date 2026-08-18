-- "Sign in with OpsAPI" (generic OIDC). A user may authenticate via an external
-- OIDC provider with NEITHER a password nor a Google account, so store the
-- provider's stable subject id and accept it as a valid credential.
ALTER TABLE users ADD COLUMN oidc_sub TEXT;

-- One OIDC identity maps to at most one account. Partial, like google_sub, so the
-- many accounts with NULL oidc_sub never collide with each other.
CREATE UNIQUE INDEX ux_users_oidc_sub ON users (oidc_sub) WHERE oidc_sub IS NOT NULL;

-- Widen "every account keeps at least one way to authenticate" to include OIDC.
ALTER TABLE users DROP CONSTRAINT ck_users_auth_method;
ALTER TABLE users ADD CONSTRAINT ck_users_auth_method
    CHECK (password_hash IS NOT NULL OR google_sub IS NOT NULL OR oidc_sub IS NOT NULL);
