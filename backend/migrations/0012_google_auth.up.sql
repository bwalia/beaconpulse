-- "Sign in with Google" (OpenID Connect). A user now authenticates EITHER with a
-- password OR a linked Google account (or both), so password_hash becomes optional
-- and a stable Google subject id is stored for lookup and linking.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users ADD COLUMN google_sub TEXT;

-- One Google identity maps to at most one account. Partial, so the many password-only
-- users (NULL google_sub) never collide with each other.
CREATE UNIQUE INDEX ux_users_google_sub ON users (google_sub) WHERE google_sub IS NOT NULL;

-- Every account must keep at least one way to authenticate.
ALTER TABLE users ADD CONSTRAINT ck_users_auth_method
    CHECK (password_hash IS NOT NULL OR google_sub IS NOT NULL);
