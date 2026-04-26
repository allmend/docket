-- Per-member availability for a sprint (0–100%, 100 = full sprint, 0 = out)
CREATE TABLE sprint_capacity (
    sprint_id   UUID        NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id)    ON DELETE CASCADE,
    focus_pct   SMALLINT    NOT NULL DEFAULT 100 CHECK (focus_pct BETWEEN 0 AND 100),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sprint_id, user_id)
);

CREATE INDEX idx_sprint_capacity_org    ON sprint_capacity (org_id);
CREATE INDEX idx_sprint_capacity_sprint ON sprint_capacity (sprint_id);
