DROP INDEX IF EXISTS idx_boards_team_unique;

ALTER TABLE tickets RENAME COLUMN team_id TO project_id;
ALTER INDEX idx_tickets_team RENAME TO idx_tickets_project;
ALTER INDEX idx_tickets_team_number RENAME TO idx_tickets_project_number;

ALTER TABLE boards RENAME COLUMN team_id TO project_id;
ALTER INDEX idx_boards_team RENAME TO idx_boards_project;

ALTER INDEX idx_teams_org RENAME TO idx_projects_org;
ALTER TABLE teams RENAME TO projects;
