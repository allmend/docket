-- name: GetUserByID :one
SELECT * FROM users WHERE org_id = $1 AND id = $2;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE org_id = $1 AND email = $2;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE org_id = $1 AND username = $2;

-- name: ListUsers :many
SELECT * FROM users WHERE org_id = $1 ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (org_id, username, name, email, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET name = $3, email = $4, role = $5, updated_at = NOW()
WHERE org_id = $1 AND id = $2
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE org_id = $1 AND id = $2;

-- name: GetUserPassword :one
SELECT password_hash FROM user_passwords WHERE user_id = $1;

-- name: UpsertUserPassword :exec
INSERT INTO user_passwords (user_id, password_hash)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET password_hash = $2, updated_at = NOW();

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetRefreshToken :one
SELECT rt.*, u.org_id FROM refresh_tokens rt
JOIN users u ON u.id = rt.user_id
WHERE rt.token_hash = $1
  AND rt.revoked_at IS NULL
  AND rt.expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1;

-- name: RevokeAllUserTokens :exec
UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: GetOrgBySlug :one
SELECT * FROM orgs WHERE slug = $1;

-- name: GetOrgByID :one
SELECT * FROM orgs WHERE id = $1;

-- name: CreateOrg :one
INSERT INTO orgs (name, slug) VALUES ($1, $2) RETURNING *;

-- name: GetIdentityProvider :one
SELECT * FROM identity_providers WHERE org_id = $1 AND enabled = TRUE LIMIT 1;

-- name: UpsertLocalIdentityProvider :one
INSERT INTO identity_providers (org_id, provider, enabled)
VALUES ($1, 'local', TRUE)
ON CONFLICT DO NOTHING
RETURNING *;
