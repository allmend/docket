package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrOrgNotFound        = errors.New("org not found")
	// ErrUserExists is a duplicate username or email within the org. Which one
	// is deliberately not distinguished: telling an admin is harmless, but this
	// same path would confirm account existence if it ever serves self-signup.
	ErrUserExists = errors.New("a user with that username or email already exists")
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 5 * 24 * time.Hour

	// refreshGrace is how long a rotated refresh token keeps working. It only has
	// to cover requests that were already in flight when the rotation happened,
	// so it is deliberately short — long enough for a page's worth of concurrent
	// HTMX requests, far too short to be useful to someone replaying a stolen
	// token. Logout revokes outright and ignores this entirely.
	refreshGrace = 30 * time.Second
)

type AuthService struct {
	store     *store.Store
	jwtSecret []byte
}

func NewAuthService(st *store.Store, jwtSecret string) *AuthService {
	return &AuthService{store: st, jwtSecret: []byte(jwtSecret)}
}

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// LoginSingleOrg validates username/password against the only org in the database.
// Used in single-tenant deployments where org selection is not exposed in the UI.
func (s *AuthService) LoginSingleOrg(ctx context.Context, username, password string) (*model.User, *TokenPair, error) {
	org, err := s.store.GetFirstOrg(ctx)
	if err != nil {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte(password)) //nolint
		return nil, nil, ErrInvalidCredentials
	}
	return s.loginWithOrg(ctx, org, username, password)
}

func (s *AuthService) loginWithOrg(ctx context.Context, org *model.Org, username, password string) (*model.User, *TokenPair, error) {
	user, err := s.store.GetUserByUsername(ctx, org.ID, username)
	if err != nil {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []byte(password)) //nolint
		return nil, nil, ErrInvalidCredentials
	}

	hash, err := s.store.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(ctx, user, uuid.Nil)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// Refresh exchanges a valid refresh token for a new token pair.
func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*model.User, *TokenPair, error) {
	h := hashToken(rawRefreshToken)
	row, err := s.store.ValidateRefreshToken(ctx, h, refreshGrace)
	if err != nil {
		if userID, reused, rerr := s.store.RevokeReusedRefreshToken(ctx, h, refreshGrace); rerr != nil {
			slog.Error("refresh token reuse check", "err", rerr)
		} else if reused {
			slog.Warn("rotated refresh token replayed after its grace window; revoked the session",
				"user_id", userID)
		}
		return nil, nil, ErrInvalidCredentials
	}

	user, err := s.store.GetUserByID(ctx, row.OrgID, row.UserID)
	if err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	// Rotate: supersede old, issue new. Superseding rather than revoking leaves
	// the old token usable for refreshGrace, so the sibling requests that raced
	// this one get a session instead of a redirect to /login.
	if err := s.store.SupersedeRefreshToken(ctx, h); err != nil {
		return nil, nil, fmt.Errorf("supersede: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, user, row.FamilyID)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// RevokeRefreshToken invalidates a raw refresh token so it can no longer be
// exchanged for new access tokens. Called on logout. Best-effort — errors are
// not fatal since the cookie has already been cleared on the client.
func (s *AuthService) RevokeRefreshToken(ctx context.Context, rawToken string) {
	_ = s.store.RevokeRefreshToken(ctx, hashToken(rawToken))
}

// CreateOrgWithAdmin bootstraps a new org with a local admin user.
func (s *AuthService) CreateOrgWithAdmin(ctx context.Context, orgName, orgSlug, username, name, email, password string) (*model.User, error) {
	org, err := s.store.CreateOrg(ctx, orgName, orgSlug)
	if err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	if err := s.store.EnsureLocalProvider(ctx, org.ID); err != nil {
		return nil, fmt.Errorf("ensure provider: %w", err)
	}

	user, err := s.store.CreateUser(ctx, org.ID, username, name, email, "admin")
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if err := s.store.UpsertPassword(ctx, user.ID, string(hash)); err != nil {
		return nil, fmt.Errorf("upsert password: %w", err)
	}

	return user, nil
}

// CreateLocalUser adds a new user to an existing org with a hashed password.
// Only admins should be able to call the handler that invokes this.
func (s *AuthService) CreateLocalUser(ctx context.Context, orgID uuid.UUID, username, name, email, password, role string) (*model.User, error) {
	user, err := s.store.CreateUser(ctx, orgID, username, name, email, role)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	if err := s.store.UpsertPassword(ctx, user.ID, string(hash)); err != nil {
		return nil, fmt.Errorf("upsert password: %w", err)
	}

	return user, nil
}

// ValidateAccessToken parses and validates a JWT, returning the claims.
func (s *AuthService) ValidateAccessToken(tokenStr string) (*model.Claims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	sub, ok1 := claims["sub"].(string)
	org, ok2 := claims["org"].(string)
	role, ok3 := claims["role"].(string)
	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("malformed token claims")
	}
	return &model.Claims{
		UserID: sub,
		OrgID:  org,
		Role:   role,
	}, nil
}

// issueTokenPair signs an access token and stores a new refresh token in
// familyID (uuid.Nil starts a new family, as on login).
func (s *AuthService) issueTokenPair(ctx context.Context, user *model.User, familyID uuid.UUID) (*TokenPair, error) {
	if familyID == uuid.Nil {
		familyID = uuid.New()
	}
	accessToken, err := s.signAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	raw, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	if err := s.store.CreateRefreshToken(ctx, user.ID, hashToken(raw), time.Now().Add(refreshTokenTTL), familyID); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{AccessToken: accessToken, RefreshToken: raw}, nil
}

func (s *AuthService) signAccessToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID.String(),
		"org":  user.OrgID.String(),
		"role": user.Role,
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
		"iat":  time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// --- Context helpers (used by middleware and handlers) ---

type ctxKeyOrgID struct{}
type ctxKeyUserID struct{}
type ctxKeyRole struct{}
type ctxKeyScope struct{}

// WithIdentity stores identity information on context (called by middleware).
func WithIdentity(ctx context.Context, orgID, userID uuid.UUID, role string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyOrgID{}, orgID)
	ctx = context.WithValue(ctx, ctxKeyUserID{}, userID)
	ctx = context.WithValue(ctx, ctxKeyRole{}, role)
	return ctx
}

// OrgIDFromContext returns the org UUID from context.
func OrgIDFromContext(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ctxKeyOrgID{}).(uuid.UUID)
	return v
}

// UserIDFromContext returns the user UUID from context.
func UserIDFromContext(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ctxKeyUserID{}).(uuid.UUID)
	return v
}

// RoleFromContext returns the role string from context.
func RoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRole{}).(string)
	return v
}

// WithScope stores the token scope on context.
func WithScope(ctx context.Context, scope model.TokenScope) context.Context {
	return context.WithValue(ctx, ctxKeyScope{}, scope)
}

// ScopeFromContext returns the token scope from context.
func ScopeFromContext(ctx context.Context) model.TokenScope {
	v, _ := ctx.Value(ctxKeyScope{}).(model.TokenScope)
	return v
}

// GetCurrentUser returns the User record for the authenticated user on the context.
func (s *AuthService) GetCurrentUser(ctx context.Context) *model.User {
	orgID := OrgIDFromContext(ctx)
	userID := UserIDFromContext(ctx)
	if orgID == (uuid.UUID{}) || userID == (uuid.UUID{}) {
		return nil
	}
	u, _ := s.store.GetUserByID(ctx, orgID, userID)
	return u
}

// GetCurrentOrg returns the Org for the request context.
func (s *AuthService) GetCurrentOrg(ctx context.Context) *model.Org {
	orgID := OrgIDFromContext(ctx)
	if orgID == (uuid.UUID{}) {
		return nil
	}
	org, _ := s.store.GetOrgByID(ctx, orgID)
	return org
}

// OrgExists reports whether any org has been created. Auto-seeding runs only
// when this is false: seeding "whenever the configured admin is missing" would
// quietly mint a default-password admin on a restart after the operator had
// changed SEED_USERNAME or dropped the seed env vars.
func (s *AuthService) OrgExists(ctx context.Context) (bool, error) {
	_, err := s.store.GetFirstOrg(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// SingleOrgName returns the name of the sole org, for display on the login page
// before anyone is authenticated. Empty when it can't be resolved — the caller
// treats that as "don't show an org name".
func (s *AuthService) SingleOrgName(ctx context.Context) string {
	org, err := s.store.GetFirstOrg(ctx)
	if err != nil || org == nil {
		return ""
	}
	return org.Name
}
