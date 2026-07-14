DROP INDEX IF EXISTS ux_organizations_status_page_slug;
ALTER TABLE organizations DROP COLUMN IF EXISTS status_page_slug;
