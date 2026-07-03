-- 0001_init — Beacon base schema.
-- Establishes multi-tenant core (organizations, users, auth) plus the first
-- monitored-resource tables (projects, monitors) and the audit trail.
-- Conventions used throughout Beacon:
--   * UUID primary keys (application-generated for portability).
--   * created_at / updated_at managed by the set_updated_at() trigger.
--   * created_by / updated_by audit columns referencing users.
--   * deleted_at for soft deletes; unique constraints are partial on
--     deleted_at IS NULL so a name can be reused after deletion.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Trigger function that stamps updated_at on every UPDATE.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- organizations — the top-level tenant boundary.
-- ---------------------------------------------------------------------------
CREATE TABLE organizations (
    id          UUID PRIMARY KEY,
    name        TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    slug        TEXT        NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_organizations_slug ON organizations (slug) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- users — members of an organization. Email is unique per system.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id             UUID PRIMARY KEY,
    org_id         UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email          TEXT        NOT NULL CHECK (position('@' IN email) > 1),
    password_hash  TEXT        NOT NULL,
    name           TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    role           TEXT        NOT NULL DEFAULT 'member'
                       CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    is_active      BOOLEAN     NOT NULL DEFAULT TRUE,
    twofa_enabled  BOOLEAN     NOT NULL DEFAULT FALSE,
    twofa_secret   TEXT,
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_users_email ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX ix_users_org ON users (org_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- refresh_tokens — hashed, rotating refresh tokens for JWT auth.
-- We store only a SHA-256 hash of the opaque token; the plaintext is returned
-- to the client once and never persisted.
-- ---------------------------------------------------------------------------
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    user_agent  TEXT,
    ip          TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_refresh_tokens_hash ON refresh_tokens (token_hash);
CREATE INDEX ix_refresh_tokens_user ON refresh_tokens (user_id);
CREATE INDEX ix_refresh_tokens_expires ON refresh_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- projects — logical grouping of monitored resources within an organization.
-- ---------------------------------------------------------------------------
CREATE TABLE projects (
    id           UUID PRIMARY KEY,
    org_id       UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    slug         TEXT        NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    description  TEXT        NOT NULL DEFAULT '',
    environment  TEXT        NOT NULL DEFAULT 'production'
                     CHECK (environment IN ('production', 'staging', 'development')),
    is_active    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    updated_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX ux_projects_org_slug ON projects (org_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX ix_projects_org ON projects (org_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- monitors — a single monitored resource. The `type` column selects the
-- probe kind; type-specific settings live in the `config` JSONB column so new
-- monitor types can be added without schema churn. `last_status` caches the
-- most recently observed state for fast list rendering.
-- ---------------------------------------------------------------------------
CREATE TABLE monitors (
    id                UUID PRIMARY KEY,
    org_id            UUID        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    project_id        UUID        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name              TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    type              TEXT        NOT NULL
                          CHECK (type IN (
                              'http', 'https', 'tcp', 'icmp', 'ssl', 'dns', 'domain',
                              'api', 'server', 'kubernetes', 'health',
                              'grafana', 'prometheus', 'gatus')),
    target            TEXT        NOT NULL CHECK (length(target) BETWEEN 1 AND 2048),
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    interval_seconds  INTEGER     NOT NULL DEFAULT 60 CHECK (interval_seconds BETWEEN 10 AND 86400),
    timeout_seconds   INTEGER     NOT NULL DEFAULT 10 CHECK (timeout_seconds BETWEEN 1 AND 300),
    config            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    last_status       TEXT        NOT NULL DEFAULT 'unknown'
                          CHECK (last_status IN ('up', 'down', 'degraded', 'unknown', 'paused')),
    last_checked_at   TIMESTAMPTZ,
    created_by        UUID        REFERENCES users (id) ON DELETE SET NULL,
    updated_by        UUID        REFERENCES users (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);
CREATE INDEX ix_monitors_org ON monitors (org_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_monitors_project ON monitors (project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_monitors_enabled ON monitors (enabled) WHERE deleted_at IS NULL;
CREATE INDEX ix_monitors_type ON monitors (type) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_monitors_updated_at BEFORE UPDATE ON monitors
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- audit_logs — append-only record of security- and change-relevant actions.
-- ---------------------------------------------------------------------------
CREATE TABLE audit_logs (
    id             UUID PRIMARY KEY,
    org_id         UUID        REFERENCES organizations (id) ON DELETE SET NULL,
    user_id        UUID        REFERENCES users (id) ON DELETE SET NULL,
    action         TEXT        NOT NULL,
    resource_type  TEXT        NOT NULL,
    resource_id    TEXT,
    metadata       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    ip             TEXT,
    user_agent     TEXT,
    request_id     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_logs_org_created ON audit_logs (org_id, created_at DESC);
CREATE INDEX ix_audit_logs_resource ON audit_logs (resource_type, resource_id);
CREATE INDEX ix_audit_logs_user ON audit_logs (user_id, created_at DESC);
