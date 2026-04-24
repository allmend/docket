-- Snapshot sprint stats at close time so they remain accurate after tickets move.
ALTER TABLE sprints ADD COLUMN committed_tickets INT NOT NULL DEFAULT 0;
ALTER TABLE sprints ADD COLUMN completed_tickets INT NOT NULL DEFAULT 0;
ALTER TABLE sprints ADD COLUMN committed_points  FLOAT NOT NULL DEFAULT 0;
ALTER TABLE sprints ADD COLUMN completed_points  FLOAT NOT NULL DEFAULT 0;

-- Retro lifecycle: open while team is discussing, closed when done.
ALTER TABLE retro_boards ADD COLUMN status     TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed'));
ALTER TABLE retro_boards ADD COLUMN closed_at  TIMESTAMPTZ;
