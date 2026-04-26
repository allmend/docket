package store

import (
	"context"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func scanToken(row interface{ Scan(...any) error }, t *model.APIToken) error {
	return row.Scan(
		&t.ID, &t.OrgID, &t.CreatedBy, &t.Name, &t.Scope,
		&t.CreatedAt, &t.LastUsedAt, &t.RevokedAt,
	)
}

const tokenCols = `id, org_id, created_by, name, scope, created_at, last_used_at, revoked_at`

func (s *Store) CreateAPIToken(ctx context.Context, orgID, createdBy uuid.UUID, name string, tokenHash string, scope model.TokenScope) (*model.APIToken, error) {
	var t model.APIToken
	err := scanToken(s.primary.QueryRow(ctx,
		`INSERT INTO api_tokens (org_id, created_by, name, token_hash, scope)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+tokenCols,
		orgID, createdBy, name, tokenHash, scope,
	), &t)
	return &t, err
}

// GetAPITokenByHash looks up a token by its SHA-256 hash.
// No org_id filter — this is used during authentication before org is known.
func (s *Store) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	var t model.APIToken
	err := scanToken(s.replica.QueryRow(ctx,
		`SELECT `+tokenCols+` FROM api_tokens WHERE token_hash = $1`,
		hash,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) ListAPITokens(ctx context.Context, orgID uuid.UUID) ([]model.APIToken, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+tokenCols+` FROM api_tokens
		 WHERE org_id = $1
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.APIToken
	for rows.Next() {
		var t model.APIToken
		if err := scanToken(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) RevokeAPIToken(ctx context.Context, orgID, tokenID uuid.UUID) error {
	now := time.Now()
	_, err := s.primary.Exec(ctx,
		`UPDATE api_tokens SET revoked_at = $1 WHERE org_id = $2 AND id = $3 AND revoked_at IS NULL`,
		now, orgID, tokenID,
	)
	return err
}

// TouchAPIToken updates last_used_at to now. Best-effort — errors are ignored by callers.
func (s *Store) TouchAPIToken(ctx context.Context, tokenID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1`,
		tokenID,
	)
	return err
}

func (s *Store) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, username, name, email, role, created_at, updated_at
		 FROM users WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE users SET role = $1, updated_at = NOW() WHERE org_id = $2 AND id = $3`,
		role, orgID, userID,
	)
	return err
}
