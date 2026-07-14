-- 0008_status_page_slug — a customisable public status-page URL.
--
-- Until now the public page lived at /status/<org-slug>, where the org slug is
-- derived from the org name at signup and can't be changed. Owners want to choose
-- their own address (e.g. /status/acme-cloud). This adds an OPTIONAL override:
-- when set, the page is served at the custom slug; when null, it stays at the org
-- slug. Kept separate from organizations.slug so the org's internal identity
-- (used elsewhere) never changes when the public URL does.

ALTER TABLE organizations
    ADD COLUMN status_page_slug TEXT
        CHECK (status_page_slug ~ '^[a-z0-9][a-z0-9-]{0,62}$');

-- No two orgs may claim the same custom slug. Partial: only set values, live rows.
CREATE UNIQUE INDEX ux_organizations_status_page_slug
    ON organizations (status_page_slug)
    WHERE status_page_slug IS NOT NULL AND deleted_at IS NULL;
