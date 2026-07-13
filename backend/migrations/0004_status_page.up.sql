-- 0004_status_page — opt-in public status pages.
--
-- A status page is served to ANYONE with the URL, so exposure is gated twice and
-- both gates default to closed:
--
--   1. organizations.status_page_enabled — the org must publish a page at all.
--   2. monitors.public                   — each monitor must be individually
--                                          published onto it.
--
-- Defaulting monitors.public to FALSE means enabling a status page never
-- retroactively leaks an endpoint someone added months ago. Opting a domain in
-- has to be a deliberate act, which is the only safe default for a surface that
-- needs no credentials to read.
--
-- The page URL reuses organizations.slug (already unique) rather than minting a
-- second public identifier that could drift from it.
--
-- Note what is NOT here: no column exposes a target, IP or check config. The
-- public projection is enforced in the query and the DTO, but keeping the schema
-- honest about intent matters too.
ALTER TABLE organizations
    ADD COLUMN status_page_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    -- Public-facing heading. Empty means "fall back to the org name", so an org
    -- that never sets it still renders a sensible page.
    ADD COLUMN status_page_title TEXT NOT NULL DEFAULT ''
        CHECK (length(status_page_title) <= 120);

ALTER TABLE monitors
    ADD COLUMN public BOOLEAN NOT NULL DEFAULT FALSE;

-- The public endpoint is unauthenticated and therefore the cheapest thing on the
-- internet to hammer. This partial index keeps its one query an index scan over
-- only the published rows, rather than a filter across every monitor in the table.
CREATE INDEX ix_monitors_public ON monitors (org_id, project_id)
    WHERE public AND enabled;
