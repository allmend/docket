-- Allow empty string as "no priority" and change default from 'medium' to ''.
ALTER TABLE tickets
    DROP CONSTRAINT IF EXISTS tickets_priority_check;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_priority_check
        CHECK (priority IN ('', 'low', 'medium', 'high', 'critical'));

ALTER TABLE tickets
    ALTER COLUMN priority SET DEFAULT '';

-- Migrate existing 'medium' defaults that came from the old default — leave
-- explicitly-set priorities untouched. Only reset rows where priority is
-- 'medium' AND created_at = updated_at (i.e. never manually changed).
-- This is intentionally conservative: if a user actually set medium, keep it.
-- Comment out the UPDATE below if you want to keep all existing priorities as-is.
-- UPDATE tickets SET priority = '' WHERE priority = 'medium' AND created_at = updated_at;
