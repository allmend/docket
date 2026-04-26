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

// APIToken is a long-lived bearer token for machine-to-machine access.
type APIToken struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	Name        string     `json:"name"`
	Scope       TokenScope `json:"scope"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

func (t *APIToken) IsRevoked() bool { return t.RevokedAt != nil }

// Claims are the JWT payload fields.
type Claims struct {
	UserID string `json:"sub"`
	OrgID  string `json:"org"`
	Role   string `json:"role"`
}
