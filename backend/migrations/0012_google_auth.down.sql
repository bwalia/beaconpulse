ALTER TABLE users DROP CONSTRAINT IF EXISTS ck_users_auth_method;
DROP INDEX IF EXISTS ux_users_google_sub;
ALTER TABLE users DROP COLUMN IF EXISTS google_sub;

-- Restore NOT NULL. Google-only users have no password, so give them an empty hash
-- (they could never password-login regardless) — a best-effort revert.
UPDATE users SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
