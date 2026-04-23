CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Organisations: top-level multi-tenant boundary
CREATE TABLE orgs (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users belong to one org
CREATE TABLE users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    username   TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    role       TEXT        NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, username),
    UNIQUE (org_id, email)
);

CREATE INDEX idx_users_org ON users (org_id);

-- Identity providers: one active per org (local | oidc | ldap)
CREATE TABLE identity_providers (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider   TEXT        NOT NULL CHECK (provider IN ('local', 'oidc', 'ldap')),
    config     JSONB       NOT NULL DEFAULT '{}',
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_identity_providers_org ON identity_providers (org_id);

-- Local auth passwords (phase 1 only)
CREATE TABLE user_passwords (
    user_id       UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash TEXT        NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Refresh tokens (all providers)
CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);

-- Boards: owned by an org
CREATE TABLE boards (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_boards_org ON boards (org_id);

-- Columns: ordered status lanes within a board
CREATE TABLE columns (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    board_id   UUID        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    position   FLOAT       NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_columns_board ON columns (board_id, position);
CREATE INDEX idx_columns_org   ON columns (org_id);

-- Tags: per-board labels
CREATE TABLE tags (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id   UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    color    TEXT NOT NULL DEFAULT '#6B7280',
    UNIQUE (board_id, name)
);

CREATE INDEX idx_tags_board ON tags (board_id);
CREATE INDEX idx_tags_org   ON tags (org_id);

-- Tickets: cards on the kanban board
CREATE TABLE tickets (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    board_id      UUID        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id     UUID        NOT NULL REFERENCES columns(id),
    assignee_id   UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_by    UUID        NOT NULL REFERENCES users(id),
    title         TEXT        NOT NULL,
    body          TEXT        NOT NULL DEFAULT '',
    priority      TEXT        NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    position      FLOAT       NOT NULL DEFAULT 0,
    external_ref  TEXT,
    search_vector TSVECTOR    GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(title, '') || ' ' || coalesce(body, ''))
    ) STORED,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_board  ON tickets (board_id, column_id, position);
CREATE INDEX idx_tickets_org    ON tickets (org_id);
CREATE INDEX idx_tickets_search ON tickets USING GIN (search_vector);

-- Ticket ↔ tag join
CREATE TABLE ticket_tags (
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    tag_id    UUID NOT NULL REFERENCES tags(id)    ON DELETE CASCADE,
    PRIMARY KEY (ticket_id, tag_id)
);
