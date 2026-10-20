package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestRefreshTokenLogoutRevokesFamily: logging out with the newest token must
// also kill its rotated predecessor, which would otherwise keep working until
// its grace window closed — and must leave other sessions (families) alone.
func TestRefreshTokenLogoutRevokesFamily(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	exp := time.Now().Add(24 * time.Hour)
	fam, other := uuid.New(), uuid.New()

	for _, h := range []string{"old", "new"} {
		if err := s.CreateRefreshToken(ctx, user.ID, h, exp, fam); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateRefreshToken(ctx, user.ID, "laptop", exp, other); err != nil {
		t.Fatal(err)
	}
	if err := s.SupersedeRefreshToken(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	// Inside the grace window the predecessor still validates...
	if _, err := s.ValidateRefreshToken(ctx, "old", time.Hour); err != nil {
		t.Fatalf("predecessor rejected inside grace: %v", err)
	}

	if err := s.RevokeRefreshToken(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	// ...but not after logout, even with a generous grace.
	for _, h := range []string{"old", "new"} {
		if _, err := s.ValidateRefreshToken(ctx, h, time.Hour); err == nil {
			t.Errorf("token %q survived logout of its family", h)
		}
	}
	if _, err := s.ValidateRefreshToken(ctx, "laptop", time.Hour); err != nil {
		t.Errorf("another session was logged out too: %v", err)
	}
}

// TestRefreshTokenReuseRevokesFamily: a rotated token presented after its grace
// window is a replay. The whole family dies, including the newest token.
func TestRefreshTokenReuseRevokesFamily(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	exp := time.Now().Add(24 * time.Hour)
	fam := uuid.New()

	for _, h := range []string{"stolen", "current"} {
		if err := s.CreateRefreshToken(ctx, user.ID, h, exp, fam); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SupersedeRefreshToken(ctx, "stolen"); err != nil {
		t.Fatal(err)
	}

	// Inside the grace window it is not reuse: nothing is revoked.
	if _, reused, err := s.RevokeReusedRefreshToken(ctx, "stolen", time.Hour); err != nil || reused {
		t.Fatalf("inside grace: reused=%v err=%v, want false", reused, err)
	}
	if _, err := s.ValidateRefreshToken(ctx, "current", time.Hour); err != nil {
		t.Fatalf("current token revoked by an in-grace check: %v", err)
	}

	// A fresh, never-rotated token is not reuse either.
	if _, reused, _ := s.RevokeReusedRefreshToken(ctx, "current", 0); reused {
		t.Fatal("an unrotated token was treated as reuse")
	}

	// After the window (grace 0): reuse, and the family is revoked.
	uid, reused, err := s.RevokeReusedRefreshToken(ctx, "stolen", 0)
	if err != nil || !reused || uid != user.ID {
		t.Fatalf("after grace: uid=%v reused=%v err=%v", uid, reused, err)
	}
	if _, err := s.ValidateRefreshToken(ctx, "current", time.Hour); err == nil {
		t.Fatal("newest token still valid after its family was replayed")
	}

	// Unknown tokens are simply not reuse.
	if _, reused, err := s.RevokeReusedRefreshToken(ctx, "nope", 0); err != nil || reused {
		t.Fatalf("unknown token: reused=%v err=%v", reused, err)
	}
}
