-- Board mode: kanban | scrum | blank
ALTER TABLE boards ADD COLUMN mode TEXT NOT NULL DEFAULT 'kanban'
    CHECK (mode IN ('kanban', 'scrum', 'blank'));

-- Sprints belong to a board (scrum boards only)
CREATE TABLE sprints (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    board_id    UUID        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'planning'
                CHECK (status IN ('planning', 'active', 'completed')),
    start_date  DATE,
    end_date    DATE,
    created_by  UUID        NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sprints_board ON sprints (board_id);
CREATE INDEX idx_sprints_org   ON sprints (org_id);

-- Only one active sprint per board at a time
CREATE UNIQUE INDEX idx_sprints_one_active
    ON sprints (board_id) WHERE status = 'active';

-- tickets.sprint_id: NULL = backlog (for scrum boards)
ALTER TABLE tickets ADD COLUMN sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL;

CREATE INDEX idx_tickets_sprint ON tickets (sprint_id) WHERE sprint_id IS NOT NULL;
