-- Restore original constraint and default.
UPDATE tickets SET priority = 'medium' WHERE priority = '';

ALTER TABLE tickets
    DROP CONSTRAINT IF EXISTS tickets_priority_check;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_priority_check
        CHECK (priority IN ('low', 'medium', 'high', 'critical'));

ALTER TABLE tickets
    ALTER COLUMN priority SET DEFAULT 'medium';
