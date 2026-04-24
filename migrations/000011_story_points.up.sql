ALTER TABLE tickets ADD COLUMN story_points INT NULL CHECK (story_points IS NULL OR story_points >= 0);
