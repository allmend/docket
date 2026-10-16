package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Org struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TokenScope represents the access level granted to an API token.
type TokenScope string

const (
	ScopeMetricsRead TokenScope = "metrics:read"
	ScopeAPIRead     TokenScope = "api:read"
	ScopeAPIWrite    TokenScope = "api:write"
)

// scopeRank orders the scopes: metrics:read < api:read < api:write.
// An unknown or empty scope ranks 0 and allows nothing.
var scopeRank = map[TokenScope]int{
	ScopeMetricsRead: 1,
	ScopeAPIRead:     2,
	ScopeAPIWrite:    3,
}

// Allows reports whether a principal holding s may use something that
// requires the given scope.
func (s TokenScope) Allows(required TokenScope) bool {
	return scopeRank[s] > 0 && scopeRank[s] >= scopeRank[required]
}

// APIToken is a long-lived bearer token for machine-to-machine access.
type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	CreatedBy  uuid.UUID  `json:"created_by"`
	Name       string     `json:"name"`
	Scope      TokenScope `json:"scope"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`

	// Revocable is stamped per viewer for rendering: whether this viewer may
	// revoke the token (see service.CanRevokeToken). Never persisted.
	Revocable bool `json:"-"`
}

func (t *APIToken) IsRevoked() bool { return t.RevokedAt != nil }

// Claims are the JWT payload fields.
type Claims struct {
	UserID string `json:"sub"`
	OrgID  string `json:"org"`
	Role   string `json:"role"`
}
