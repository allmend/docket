package store

import (
	"context"
	"errors"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) GetFirstOrg(ctx context.Context) (*model.Org, error) {
	var o model.Org
	err := s.replica.QueryRow(ctx,
		// Ordered so login and auto-seed agree on which org is "the" org even
		// if a second one ever appears; a bare LIMIT 1 is arbitrary.
		`SELECT id, name, slug, created_at, updated_at FROM orgs ORDER BY created_at, id LIMIT 1`,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) GetOrgByID(ctx context.Context, id uuid.UUID) (*model.Org, error) {
	var o model.Org
	err := s.replica.QueryRow(ctx,
		`SELECT id, name, slug, created_at, updated_at FROM orgs WHERE id = $1`,
		id,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) CreateOrg(ctx context.Context, name, slug string) (*model.Org, error) {
	var o model.Org
	err := s.primary.QueryRow(ctx,
		`INSERT INTO orgs (name, slug) VALUES ($1, $2)
		 ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, name, slug, created_at, updated_at`,
		name, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, orgID uuid.UUID, username string) (*model.User, error) {
	var u model.User
	err := s.replica.QueryRow(ctx,
		`SELECT id, org_id, username, name, email, role, created_at, updated_at
		 FROM users WHERE org_id = $1 AND username = $2`,
		orgID, username,
	).Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, orgID, userID uuid.UUID) (*model.User, error) {
	var u model.User
	err := s.replica.QueryRow(ctx,
		`SELECT id, org_id, username, name, email, role, created_at, updated_at
		 FROM users WHERE org_id = $1 AND id = $2`,
		orgID, userID,
	).Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ErrDuplicate reports a unique-constraint violation, so callers can tell "this
// already exists" apart from a real failure without knowing any Postgres.
var ErrDuplicate = errors.New("already exists")

// isUniqueViolation matches Postgres 23505.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateUser inserts a user. It deliberately does NOT upsert: it used to carry
// ON CONFLICT (org_id, username) DO UPDATE, which meant adding a member whose
// username already existed silently overwrote that account's name and email —
// and since the caller follows up with UpsertPassword, reset their password too.
// Seeding does not need it either: CreateOrgWithAdmin inserts the org first and
// orgs.slug is UNIQUE, so a second startup never reaches this.
//
// Both (org_id, username) and (org_id, email) are unique, so either collision
// surfaces as ErrDuplicate.
func (s *Store) CreateUser(ctx context.Context, orgID uuid.UUID, username, name, email, role string) (*model.User, error) {
	var u model.User
	err := s.primary.QueryRow(ctx,
		`INSERT INTO users (org_id, username, name, email, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, org_id, username, name, email, role, created_at, updated_at`,
		orgID, username, name, email, role,
	).Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpsertPassword(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO user_passwords (user_id, password_hash)
		 VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET password_hash = $2, updated_at = NOW()`,
		userID, hash,
	)
	return err
}

func (s *Store) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	err := s.replica.QueryRow(ctx,
		`SELECT password_hash FROM user_passwords WHERE user_id = $1`,
		userID,
	).Scan(&hash)
	return hash, err
}

func (s *Store) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

type RefreshTokenRow struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
}

// ValidateRefreshToken accepts a token that is unrevoked and unexpired, and
// additionally one that was superseded by a rotation less than grace ago — see
// SupersedeRefreshToken for why. Revocation is unaffected by grace and always
// takes effect immediately.
func (s *Store) ValidateRefreshToken(ctx context.Context, tokenHash string, grace time.Duration) (*RefreshTokenRow, error) {
	var r RefreshTokenRow
	// NOTE: must use primary when a real read replica is added — replication lag
	// could make a just-revoked token appear valid on the replica.
	err := s.replica.QueryRow(ctx,
		`SELECT rt.user_id, u.org_id FROM refresh_tokens rt
		 JOIN users u ON u.id = rt.user_id
		 WHERE rt.token_hash = $1
		   AND rt.revoked_at IS NULL
		   AND rt.expires_at > NOW()
		   AND (rt.superseded_at IS NULL
		        OR rt.superseded_at > NOW() - make_interval(secs => $2))`,
		tokenHash, grace.Seconds(),
	).Scan(&r.UserID, &r.OrgID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// SupersedeRefreshToken marks a token as rotated. It stays usable for the grace
// window ValidateRefreshToken applies, so concurrent requests that were already
// in flight with this token do not get logged out by the rotation.
//
// The superseded_at IS NULL guard keeps the *first* rotation's timestamp. Without
// it every racing request would push the timestamp forward and the window would
// slide indefinitely, which is the opposite of a grace period.
func (s *Store) SupersedeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE refresh_tokens SET superseded_at = NOW()
		 WHERE token_hash = $1 AND superseded_at IS NULL`,
		tokenHash,
	)
	return err
}

// RevokeRefreshToken kills a token immediately, with no grace — this is logout
// and explicit revocation, where the whole point is that the token stops working.
func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`,
		tokenHash,
	)
	return err
}

func (s *Store) ListUsers(ctx context.Context, orgID uuid.UUID) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, username, name, email, role, created_at, updated_at
		 FROM users WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) SearchUsers(ctx context.Context, orgID uuid.UUID, q string) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, username, name, email, role, created_at, updated_at
		 FROM users WHERE org_id = $1 AND (name ILIKE $2 ESCAPE '\' OR username ILIKE $2 ESCAPE '\')
		 ORDER BY name LIMIT 10`,
		orgID, containsPattern(q),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) EnsureLocalProvider(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO identity_providers (org_id, provider, enabled)
		 VALUES ($1, 'local', TRUE)
		 ON CONFLICT DO NOTHING`,
		orgID,
	)
	return err
}
