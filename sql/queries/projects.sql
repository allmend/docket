-- name: ListProjects :many
SELECT * FROM projects WHERE org_id = $1 ORDER BY name;

-- name: GetProject :one
SELECT * FROM projects WHERE org_id = $1 AND id = $2;

-- name: CreateProject :one
INSERT INTO projects (org_id, name, key, description, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects SET name = $3, description = $4, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE org_id = $1 AND id = $2;

-- name: NextTicketNumber :one
UPDATE projects SET ticket_counter = ticket_counter + 1
WHERE id = $1
RETURNING ticket_counter;
