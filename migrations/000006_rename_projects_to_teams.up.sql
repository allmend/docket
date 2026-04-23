-- Rename projects → teams.
-- "teams" are the core organisational unit: one board per team, nav shows team list.

ALTER TABLE projects RENAME TO teams;
ALTER INDEX idx_projects_org RENAME TO idx_teams_org;

-- Rename project_id → team_id on boards
ALTER TABLE boards RENAME COLUMN project_id TO team_id;
ALTER INDEX idx_boards_project RENAME TO idx_boards_team;

-- Rename project_id → team_id on tickets
ALTER TABLE tickets RENAME COLUMN project_id TO team_id;
ALTER INDEX idx_tickets_project RENAME TO idx_tickets_team;
ALTER INDEX idx_tickets_project_number RENAME TO idx_tickets_team_number;

-- Enforce one board per team (team_id IS NULL boards are standalone, still allowed)
CREATE UNIQUE INDEX idx_boards_team_unique ON boards (team_id) WHERE team_id IS NOT NULL;
