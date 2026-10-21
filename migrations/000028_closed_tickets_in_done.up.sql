-- Tickets closed as done belong in their board's Done column: from 0.19.1 the
-- Close button moves them there, as dragging always did. Move the ones closed
-- as done before that, after anything already in Done. Other close reasons
-- (duplicate, won't do, …) stay where they are, as do boards without Done.
WITH done AS (
    SELECT DISTINCT ON (board_id) id, board_id, org_id
    FROM columns
    WHERE LOWER(name) = 'done'
    ORDER BY board_id, position
),
moving AS (
    SELECT t.id, d.id AS done_id,
           ROW_NUMBER() OVER (PARTITION BY d.id ORDER BY t.closed_at, t.id) AS n
    FROM tickets t
    JOIN columns cur ON cur.id = t.column_id
    JOIN done d ON d.board_id = t.board_id AND d.org_id = t.org_id
    WHERE t.closed_at IS NOT NULL
      AND COALESCE(t.close_reason, 'done') = 'done'
      AND LOWER(cur.name) <> 'done'
)
UPDATE tickets t
SET column_id = m.done_id,
    position  = COALESCE((SELECT MAX(x.position) FROM tickets x WHERE x.column_id = m.done_id), 0) + m.n * 1000,
    updated_at = NOW()
FROM moving m
WHERE t.id = m.id;
