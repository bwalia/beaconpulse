-- API keys: machine authentication for the public API.
--
-- A key is an opaque random secret. It carries no information about who holds it or
-- what they are entitled to — it RESOLVES to an organization, and the plan, credit and
-- limits are then read live from that organization. Embedding a plan or balance in the
-- key itself would be a snapshot of a moving number: pay-as-you-go credit changes every
-- minute, a downgrade would not take effect until the key was regenerated, and
-- revocation would need a blocklist, which is a lookup, which is this table.
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY,
    org_id      UUID NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    -- Who minted it. SET NULL rather than CASCADE: an offboarded employee's keys must
    -- keep working until someone deliberately revokes them, or removing a user would
    -- silently break a customer's CI at 3am.
    created_by  UUID REFERENCES users (id) ON DELETE SET NULL,
    name        TEXT NOT NULL,

    -- SHA-256 of the whole key, hex. UNIQUE so authentication is one indexed lookup
    -- rather than a scan-and-compare over every key in the table.
    --
    -- SHA-256 and not bcrypt, deliberately. bcrypt is slow BY DESIGN — that is what
    -- makes it right for passwords, which are low-entropy and guessable. This secret is
    -- 256 bits of CSPRNG output, so there is nothing to brute-force, and bcrypt's ~100ms
    -- would be paid on every single API request.
    key_hash    TEXT NOT NULL UNIQUE,
    -- The first few characters, stored in clear for display: a user with four keys has
    -- to be able to tell which one to revoke, and the secret is unrecoverable by then.
    key_prefix  TEXT NOT NULL,

    -- Capped at the creating user's role, so a key can never out-rank the person who
    -- minted it. A read-only key for a status dashboard is the common case.
    role        TEXT NOT NULL,

    -- Optional lifetime. NULL means no expiry.
    expires_at  TIMESTAMPTZ,
    -- Revocation is a tombstone, not a delete: the row is the audit trail of a key that
    -- existed and was withdrawn, which is exactly what you want to read after an
    -- incident.
    revoked_at  TIMESTAMPTZ,
    -- Written coarsely (see the repository), so "is this key still in use?" is
    -- answerable without a database write on every authenticated request.
    last_used_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Listing a org's keys in the dashboard, newest first.
CREATE INDEX IF NOT EXISTS ix_api_keys_org ON api_keys (org_id, created_at DESC);
