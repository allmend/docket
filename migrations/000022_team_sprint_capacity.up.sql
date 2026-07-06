-- Team-settable sprint capacity in story points. Used as the denominator of
-- the "committed" bar on the sprint planning view.
ALTER TABLE teams ADD COLUMN sprint_capacity INT NOT NULL DEFAULT 20;
