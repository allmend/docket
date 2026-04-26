package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
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

func (s *TokenService) Revoke(ctx context.Context, orgID, tokenID uuid.UUID) error {
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

func (s *TokenService) UpdateRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	if role != "admin" && role != "member" {
		return fmt.Errorf("invalid role: %s", role)
	}
	return s.store.UpdateUserRole(ctx, orgID, userID, role)
}

func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
