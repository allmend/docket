ALTER TABLE tickets ALTER COLUMN story_points TYPE NUMERIC(5,1) USING story_points::NUMERIC(5,1);
