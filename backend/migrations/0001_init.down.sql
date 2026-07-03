-- 0001_init (down) — drop the base schema in reverse dependency order.
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS monitors;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
DROP FUNCTION IF EXISTS set_updated_at();
