ALTER TABLE tickets
    DROP CONSTRAINT tickets_team_id_fkey,
    ADD CONSTRAINT tickets_team_id_fkey
        FOREIGN KEY (team_id) REFERENCES teams(id);
