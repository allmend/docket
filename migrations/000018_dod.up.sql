-- Definition of Done checklist items per board
CREATE TABLE dod_items (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL REFERENCES orgs(id)    ON DELETE CASCADE,
    board_id   UUID        NOT NULL REFERENCES boards(id)  ON DELETE CASCADE,
    text       TEXT        NOT NULL,
    position   FLOAT       NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dod_items_board ON dod_items (board_id);
CREATE INDEX idx_dod_items_org   ON dod_items (org_id);

-- Per-ticket check state for each DoD item
CREATE TABLE dod_checks (
    dod_item_id UUID        NOT NULL REFERENCES dod_items(id) ON DELETE CASCADE,
    ticket_id   UUID        NOT NULL REFERENCES tickets(id)   ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id)      ON DELETE CASCADE,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (dod_item_id, ticket_id)
);

CREATE INDEX idx_dod_checks_ticket ON dod_checks (ticket_id);
CREATE INDEX idx_dod_checks_org    ON dod_checks (org_id);
