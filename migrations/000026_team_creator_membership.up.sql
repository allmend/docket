-- Workspaces were created without adding their creator to team_members, so
-- some exist with no members at all. Backfill the creator as a member where
-- missing. Only when the creator is still a user of the same org.
-- Idempotent: re-running inserts nothing.
INSERT INTO team_members (org_id, team_id, user_id)
SELECT t.org_id, t.id, t.created_by
FROM teams t
JOIN users u ON u.id = t.created_by AND u.org_id = t.org_id
ON CONFLICT (team_id, user_id) DO NOTHING;
