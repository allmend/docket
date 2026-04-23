CREATE TABLE ticket_assignees (
    ticket_id   UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (ticket_id, user_id)
);

CREATE INDEX idx_ticket_assignees_ticket ON ticket_assignees(ticket_id);
