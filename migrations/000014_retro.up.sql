CREATE TABLE retro_boards (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     UUID        NOT NULL,
    board_id   UUID        NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    sprint_id  UUID        REFERENCES sprints(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_retro_boards_org   ON retro_boards(org_id);
CREATE INDEX idx_retro_boards_board ON retro_boards(board_id);

CREATE TABLE retro_cards (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID        NOT NULL,
    retro_board_id UUID        NOT NULL REFERENCES retro_boards(id) ON DELETE CASCADE,
    column_name    TEXT        NOT NULL CHECK (column_name IN ('went_well', 'didnt_go_well', 'action_item')),
    body           TEXT        NOT NULL,
    author_id      UUID        NOT NULL REFERENCES users(id),
    owner_id       UUID        REFERENCES users(id),
    owner_name     TEXT,
    ticket_id      UUID        REFERENCES tickets(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_retro_cards_org   ON retro_cards(org_id);
CREATE INDEX idx_retro_cards_board ON retro_cards(retro_board_id);
