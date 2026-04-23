-- Ticket comments
CREATE TABLE ticket_comments (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    ticket_id  UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_id  UUID        NOT NULL REFERENCES users(id),
    body       TEXT        NOT NULL DEFAULT '',
    edited     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_comments_ticket ON ticket_comments (ticket_id, created_at);
CREATE INDEX idx_ticket_comments_org    ON ticket_comments (org_id);

-- Ticket history (audit log)
CREATE TABLE ticket_history (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id  UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    actor_id   UUID        NOT NULL REFERENCES users(id),
    actor_name TEXT        NOT NULL,
    field      TEXT        NOT NULL, -- e.g. "priority", "title", "status", "comment"
    old_value  TEXT        NOT NULL DEFAULT '',
    new_value  TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_history_ticket ON ticket_history (ticket_id, created_at);
