-- 0018_platform_site_details — public company & contact details, edited at /platform.
--
-- Shown on the public Terms and Privacy pages: who operates the service, where to
-- write, and how to reach support. Lives in the operator-editable settings row so it
-- can change without a redeploy. Empty = not set; the web app then falls back to its
-- brand defaults.
ALTER TABLE platform_settings
    ADD COLUMN support_email TEXT NOT NULL DEFAULT '',
    ADD COLUMN legal_entity  TEXT NOT NULL DEFAULT '',
    ADD COLUMN legal_address TEXT NOT NULL DEFAULT '';
