package store

import (
	"context"
	"testing"
	"time"
)

// TestRefreshTokenGraceWindow locks in the fix for the sporadic logouts: the
// board polls every 30s and a page fires several HTMX requests at once, so when
// the access token expires they all try to refresh with the same token. Under
// the old behaviour the first rotated it and the rest presented a token revoked
// microseconds earlier and were bounced to /login.
//
// A rotated (superseded) token must keep validating for the grace window, and
// must stop the moment the window closes.
func TestRefreshTokenGraceWindow(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	const hash = "hash-grace"
	if err := s.CreateRefreshToken(ctx, user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Baseline: a fresh token validates.
	if _, err := s.ValidateRefreshToken(ctx, hash, time.Minute); err != nil {
		t.Fatalf("fresh token rejected: %v", err)
	}

	// Rotate it, as AuthService.Refresh does.
	if err := s.SupersedeRefreshToken(ctx, hash); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	// The request that lost the race arrives moments later and must still work.
	if _, err := s.ValidateRefreshToken(ctx, hash, time.Minute); err != nil {
		t.Fatalf("superseded token rejected inside grace window: %v", err)
	}

	// Once the window has passed it must not: a grace period that never expires
	// is just a refresh token that is never rotated.
	if _, err := s.ValidateRefreshToken(ctx, hash, 0); err == nil {
		t.Fatal("superseded token accepted after the grace window closed")
	}
}

// TestRefreshTokenGraceDoesNotSlide guards the superseded_at IS NULL condition.
// Every racing request calls SupersedeRefreshToken; if each one rewrote the
// timestamp, the window would keep moving forward and the token would stay alive
// for as long as anything kept presenting it.
func TestRefreshTokenGraceDoesNotSlide(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	const hash = "hash-slide"
	if err := s.CreateRefreshToken(ctx, user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.SupersedeRefreshToken(ctx, hash); err != nil {
		t.Fatalf("first supersede: %v", err)
	}
	var first time.Time
	if err := s.primary.QueryRow(ctx,
		`SELECT superseded_at FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&first); err != nil {
		t.Fatalf("read superseded_at: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := s.SupersedeRefreshToken(ctx, hash); err != nil {
		t.Fatalf("second supersede: %v", err)
	}

	var second time.Time
	if err := s.primary.QueryRow(ctx,
		`SELECT superseded_at FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&second); err != nil {
		t.Fatalf("read superseded_at: %v", err)
	}
	if !second.Equal(first) {
		t.Fatalf("second supersede moved the window: %v -> %v", first, second)
	}
}

// TestRefreshTokenRevokeIgnoresGrace pins that logout is still immediate. The
// grace window exists for rotation only — if it applied to revocation too,
// signing out would leave a working token behind.
func TestRefreshTokenRevokeIgnoresGrace(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	const hash = "hash-revoked"
	if err := s.CreateRefreshToken(ctx, user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.RevokeRefreshToken(ctx, hash); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Generous grace, so only the revocation can be doing the rejecting.
	if _, err := s.ValidateRefreshToken(ctx, hash, time.Hour); err == nil {
		t.Fatal("revoked token accepted; logout does not end the session")
	}
}
