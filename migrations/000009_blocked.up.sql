CREATE TABLE ticket_links (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID        NOT NULL REFERENCES orgs(id),
    from_ticket_id  UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    to_ticket_id    UUID        NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    relation_type   TEXT        NOT NULL CHECK (relation_type IN ('blocks', 'depends_on', 'duplicates', 'relates_to')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (from_ticket_id, to_ticket_id, relation_type)
);

CREATE INDEX idx_ticket_links_from ON ticket_links (org_id, from_ticket_id);
CREATE INDEX idx_ticket_links_to   ON ticket_links (org_id, to_ticket_id);
