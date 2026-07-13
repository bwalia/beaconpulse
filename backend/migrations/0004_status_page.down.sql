DROP INDEX IF EXISTS ix_monitors_public;

ALTER TABLE monitors
    DROP COLUMN IF EXISTS public;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS status_page_title,
    DROP COLUMN IF EXISTS status_page_enabled;
