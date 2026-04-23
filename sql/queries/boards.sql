-- name: ListBoards :many
SELECT * FROM boards WHERE org_id = $1 ORDER BY name;

-- name: GetBoard :one
SELECT * FROM boards WHERE org_id = $1 AND id = $2;

-- name: CreateBoard :one
INSERT INTO boards (org_id, name, description, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateBoard :one
UPDATE boards SET name = $3, description = $4, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteBoard :exec
DELETE FROM boards WHERE org_id = $1 AND id = $2;

-- name: ListColumns :many
SELECT * FROM columns WHERE org_id = $1 AND board_id = $2 ORDER BY position;

-- name: GetColumn :one
SELECT * FROM columns WHERE org_id = $1 AND id = $2;

-- name: CreateColumn :one
INSERT INTO columns (org_id, board_id, name, position)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateColumn :one
UPDATE columns SET name = $3, position = $4, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteColumn :exec
DELETE FROM columns WHERE org_id = $1 AND id = $2;

-- name: MaxColumnPosition :one
SELECT COALESCE(MAX(position), 0)::FLOAT FROM columns WHERE board_id = $1;

-- name: ListTags :many
SELECT * FROM tags WHERE org_id = $1 AND board_id = $2 ORDER BY name;

-- name: CreateTag :one
INSERT INTO tags (org_id, board_id, name, color)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags WHERE org_id = $1 AND id = $2;
