CREATE TABLE api_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by   UUID        NOT NULL REFERENCES users(id),
    name         TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,
    scope        TEXT        NOT NULL CHECK (scope IN ('metrics:read', 'api:read', 'api:write')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX idx_api_tokens_org  ON api_tokens (org_id);
CREATE INDEX idx_api_tokens_hash ON api_tokens (token_hash);
