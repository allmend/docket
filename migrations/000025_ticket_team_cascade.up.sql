-- Deleting a workspace must delete its tickets. tickets.team_id kept the
-- NO ACTION foreign key from the projects era (the constraint still carries
-- its old name, tickets_project_id_fkey), and Postgres checks it before the
-- boards → tickets cascade removes the rows, so every workspace delete with
-- tickets failed.
--
-- The constraint is found by what it references rather than by name: this runs
-- at startup, and a name that differs on some database would crash-loop the pod.
DO $$
DECLARE
    fk text;
BEGIN
    FOR fk IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
        WHERE c.contype = 'f'
          AND c.conrelid = 'tickets'::regclass
          AND c.confrelid = 'teams'::regclass
          AND a.attname = 'team_id'
    LOOP
        EXECUTE format('ALTER TABLE tickets DROP CONSTRAINT %I', fk);
    END LOOP;
END $$;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_team_id_fkey
        FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;
