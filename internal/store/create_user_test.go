package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// TestCreateUserDuplicateUsernameDoesNotOverwrite is the important one. CreateUser
// carried ON CONFLICT (org_id, username) DO UPDATE, so adding a member whose
// username already existed quietly rewrote that account's name and email — and
// because CreateLocalUser follows up with UpsertPassword, reset their password
// too. An admin adding a colleague could take over an existing account by
// mistyping a username, with no error shown.
func TestCreateUserDuplicateUsernameDoesNotOverwrite(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")

	original, err := s.CreateUser(ctx, org.ID, "alice", "Alice Original", "alice@example.com", "admin")
	if err != nil {
		t.Fatalf("create original: %v", err)
	}

	_, err = s.CreateUser(ctx, org.ID, "alice", "Impostor", "impostor@example.com", "member")
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate username: got %v, want ErrDuplicate", err)
	}

	got, err := s.GetUserByID(ctx, org.ID, original.ID)
	if err != nil {
		t.Fatalf("reload original: %v", err)
	}
	if got.Name != "Alice Original" || got.Email != "alice@example.com" || got.Role != "admin" {
		t.Fatalf("existing account was modified: name=%q email=%q role=%q",
			got.Name, got.Email, got.Role)
	}
}

// TestCreateUserDuplicateEmail covers the case from the ticket: a distinct
// username with an email already in use. It hit (org_id, email) and surfaced as
// a raw 500 "failed to create user" with the Postgres detail in the response.
func TestCreateUserDuplicateEmail(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")

	if _, err := s.CreateUser(ctx, org.ID, "alice", "Alice", "shared@example.com", "member"); err != nil {
		t.Fatalf("create first: %v", err)
	}
	_, err := s.CreateUser(ctx, org.ID, "bob", "Bob", "shared@example.com", "member")
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate email: got %v, want ErrDuplicate", err)
	}
}

// TestCreateUserSameUsernameDifferentOrg pins that the uniqueness is per-org, so
// removing the upsert did not make usernames globally unique.
func TestCreateUserSameUsernameDifferentOrg(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")

	if _, err := s.CreateUser(ctx, orgA.ID, "alice", "Alice A", "alice@a.example", "member"); err != nil {
		t.Fatalf("create in org A: %v", err)
	}
	u, err := s.CreateUser(ctx, orgB.ID, "alice", "Alice B", "alice@b.example", "member")
	if err != nil {
		t.Fatalf("same username in another org must be allowed: %v", err)
	}
	if u.OrgID != orgB.ID {
		t.Fatalf("user landed in org %v, want %v", u.OrgID, orgB.ID)
	}
	if u.ID == uuid.Nil {
		t.Fatal("no id returned")
	}
}
