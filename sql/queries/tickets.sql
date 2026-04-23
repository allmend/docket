-- name: ListTicketsByColumn :many
SELECT t.*, u.name AS assignee_name
FROM tickets t
LEFT JOIN users u ON u.id = t.assignee_id
WHERE t.org_id = $1 AND t.column_id = $2
ORDER BY t.position;

-- name: ListTicketsByBoard :many
SELECT t.*, u.name AS assignee_name
FROM tickets t
LEFT JOIN users u ON u.id = t.assignee_id
WHERE t.org_id = $1 AND t.board_id = $2
ORDER BY t.column_id, t.position;

-- name: GetTicket :one
SELECT t.*, u.name AS assignee_name
FROM tickets t
LEFT JOIN users u ON u.id = t.assignee_id
WHERE t.org_id = $1 AND t.id = $2;

-- name: CreateTicket :one
INSERT INTO tickets (org_id, board_id, column_id, created_by, title, body, priority, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateTicket :one
UPDATE tickets
SET title = $3, body = $4, priority = $5, assignee_id = $6, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: MoveTicket :one
UPDATE tickets
SET column_id = $3, position = $4, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteTicket :exec
DELETE FROM tickets WHERE org_id = $1 AND id = $2;

-- name: MaxTicketPositionInColumn :one
SELECT COALESCE(MAX(position), 0)::FLOAT FROM tickets WHERE column_id = $1;

-- name: SearchTickets :many
SELECT t.*, u.name AS assignee_name,
       ts_rank(t.search_vector, websearch_to_tsquery('english', $2)) AS rank
FROM tickets t
LEFT JOIN users u ON u.id = t.assignee_id
WHERE t.org_id = $1
  AND t.search_vector @@ websearch_to_tsquery('english', $2)
ORDER BY rank DESC
LIMIT 50;

-- name: ListTicketTags :many
SELECT tg.* FROM tags tg
JOIN ticket_tags tt ON tt.tag_id = tg.id
WHERE tt.ticket_id = $1;

-- name: AddTicketTag :exec
INSERT INTO ticket_tags (ticket_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveTicketTag :exec
DELETE FROM ticket_tags WHERE ticket_id = $1 AND tag_id = $2;
