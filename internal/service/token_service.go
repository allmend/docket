package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TokenService struct {
	store *store.Store
}

func NewTokenService(st *store.Store) *TokenService {
	return &TokenService{store: st}
}

// Create generates a new API token, stores its hash, and returns the plaintext once.
// The plaintext is never stored — callers must show it to the user immediately.
func (s *TokenService) Create(ctx context.Context, orgID, createdBy uuid.UUID, name string, scope model.TokenScope) (plaintext string, token *model.APIToken, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	plaintext = "dkt_" + secret

	hash := sha256sum(plaintext)
	token, err = s.store.CreateAPIToken(ctx, orgID, createdBy, name, hash, scope)
	return plaintext, token, err
}

func (s *TokenService) List(ctx context.Context, orgID uuid.UUID) ([]model.APIToken, error) {
	return s.store.ListAPITokens(ctx, orgID)
}

// CanRevokeToken is the single rule for revoking an API token: org admins may
// revoke any token, anyone else only the tokens they created — otherwise any
// member could cut off an admin's or an integration's credentials. Revoke
// enforces it; the settings page uses it to show the button only where it works.
func CanRevokeToken(t *model.APIToken, actorID uuid.UUID, actorRole string) bool {
	return actorRole == "admin" || t.CreatedBy == actorID
}

// Revoke revokes an API token, subject to CanRevokeToken.
func (s *TokenService) Revoke(ctx context.Context, orgID, tokenID, actorID uuid.UUID, actorRole string) error {
	t, err := s.store.GetAPIToken(ctx, orgID, tokenID)
	if err != nil {
		return err
	}
	if !CanRevokeToken(t, actorID, actorRole) {
		return ErrForbidden
	}
	return s.store.RevokeAPIToken(ctx, orgID, tokenID)
}

// Validate looks up an API token by its plaintext value.
// Returns an error if the token is not found or has been revoked.
// Updates last_used_at as a best-effort side effect.
func (s *TokenService) Validate(ctx context.Context, plaintext string) (*model.APIToken, error) {
	hash := sha256sum(plaintext)
	t, err := s.store.GetAPITokenByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("token not found")
	}
	if t.IsRevoked() {
		return nil, fmt.Errorf("token revoked")
	}
	go s.store.TouchAPIToken(context.Background(), t.ID) //nolint:errcheck
	return t, nil
}

func (s *TokenService) ListMembers(ctx context.Context, orgID uuid.UUID) ([]model.User, error) {
	return s.store.ListOrgMembers(ctx, orgID)
}

// UpdateRole sets a member's role. Demoting the last admin is refused — the org
// would be left with nobody able to manage members or roles.
func (s *TokenService) UpdateRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "member" {
		return fmt.Errorf("%w: role must be admin or member", ErrInvalidInput)
	}
	if _, err := s.store.GetUserByID(ctx, orgID, userID); err != nil {
		return err
	}
	if err := s.store.UpdateUserRole(ctx, orgID, userID, role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: the org must keep at least one admin", ErrInvalidInput)
		}
		return err
	}
	return nil
}

func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
