ALTER TABLE sprints DROP COLUMN IF EXISTS committed_tickets;
ALTER TABLE sprints DROP COLUMN IF EXISTS completed_tickets;
ALTER TABLE sprints DROP COLUMN IF EXISTS committed_points;
ALTER TABLE sprints DROP COLUMN IF EXISTS completed_points;
ALTER TABLE retro_boards DROP COLUMN IF EXISTS status;
ALTER TABLE retro_boards DROP COLUMN IF EXISTS closed_at;
