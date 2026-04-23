CREATE TABLE notifications (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID        NOT NULL REFERENCES orgs(id),
    user_id     UUID        NOT NULL REFERENCES users(id),
    ticket_id   UUID        REFERENCES tickets(id) ON DELETE CASCADE,
    actor_id    UUID        REFERENCES users(id),
    actor_name  TEXT        NOT NULL DEFAULT '',
    type        TEXT        NOT NULL CHECK (type IN ('assigned', 'mentioned', 'comment')),
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user    ON notifications (org_id, user_id, created_at DESC);
CREATE INDEX idx_notifications_unread  ON notifications (org_id, user_id) WHERE read_at IS NULL;
