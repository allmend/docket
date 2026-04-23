CREATE TABLE team_members (
    org_id    UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    team_id   UUID        NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

CREATE INDEX idx_team_members_org  ON team_members (org_id);
CREATE INDEX idx_team_members_user ON team_members (user_id);
