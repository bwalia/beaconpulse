-- 0018_platform_support_email — the public support/contact address, edited at /platform.
--
-- Shown on the public Terms and Privacy pages (and anywhere else the site tells people
-- how to reach us). Lives in the operator-editable settings row so it can change
-- without a redeploy. Empty = not set; the web app then falls back to its brand default.
ALTER TABLE platform_settings ADD COLUMN support_email TEXT NOT NULL DEFAULT '';
