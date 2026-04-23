-- Projects: top-level container with a short key used as ticket prefix (e.g. BE-42)
CREATE TABLE projects (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name           TEXT        NOT NULL,
    key            TEXT        NOT NULL,
    description    TEXT        NOT NULL DEFAULT '',
    ticket_counter INT         NOT NULL DEFAULT 0,
    created_by     UUID        NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, key)
);

CREATE INDEX idx_projects_org ON projects (org_id);

ALTER TABLE boards
    ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

CREATE INDEX idx_boards_project ON boards (project_id);

ALTER TABLE tickets
    ADD COLUMN project_id UUID REFERENCES projects(id),
    ADD COLUMN number     INT  NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX idx_tickets_project_number ON tickets (project_id, number)
    WHERE project_id IS NOT NULL;

CREATE INDEX idx_tickets_project ON tickets (project_id);
