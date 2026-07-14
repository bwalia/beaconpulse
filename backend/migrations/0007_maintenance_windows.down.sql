-- The trigger and indexes drop with the table, but DROP them explicitly first so
-- a partial-apply rollback is not left with orphans.
DROP TRIGGER IF EXISTS trg_maintenance_windows_updated_at ON maintenance_windows;
DROP INDEX IF EXISTS ix_maintenance_windows_scope_ids;
DROP INDEX IF EXISTS ix_maintenance_windows_active;
DROP TABLE IF EXISTS maintenance_windows;
