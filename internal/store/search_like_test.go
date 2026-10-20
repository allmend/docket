package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestTicketSearch_LiteralWildcards: "_" and "%" typed into the link and
// mention pickers match themselves, not any character / everything.
func TestTicketSearch_LiteralWildcards(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-like")
	user := seedUser(t, org.ID, "alice")
	seedTicket(t, org.ID, user.ID, "snake_case parser")
	seedTicket(t, org.ID, user.ID, "snakeXcase parser")

	titles := func(q string) []string {
		t.Helper()
		got, err := s.SearchTicketsForMention(ctx, org.ID, q)
		if err != nil {
			t.Fatalf("mention search %q: %v", q, err)
		}
		link, err := s.SearchTicketsForLink(ctx, org.ID, uuid.New(), q)
		if err != nil {
			t.Fatalf("link search %q: %v", q, err)
		}
		if len(link) != len(got) {
			t.Fatalf("link and mention search disagree for %q: %d vs %d", q, len(link), len(got))
		}
		var out []string
		for _, tk := range got {
			out = append(out, tk.Title)
		}
		return out
	}

	if got := titles("e_c"); len(got) != 1 || got[0] != "snake_case parser" {
		t.Errorf(`"e_c" should match only the underscore title, got %v`, got)
	}
	if got := titles("%"); len(got) != 0 {
		t.Errorf(`"%%" should match nothing, got %v`, got)
	}
	if got := titles("parser"); len(got) != 2 {
		t.Errorf(`"parser" should match both, got %v`, got)
	}
}

// TestSearchUsers_LiteralWildcards covers the @mention and member pickers.
func TestSearchUsers_LiteralWildcards(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-like-users")
	seedUser(t, org.ID, "ann_lee")
	seedUser(t, org.ID, "annxlee")

	got, err := s.SearchUsers(ctx, org.ID, "n_l")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Username != "ann_lee" {
		t.Errorf(`"n_l" should match only ann_lee, got %d users`, len(got))
	}
	got, err = s.SearchUsers(ctx, org.ID, "%")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf(`"%%" should match no users, got %d`, len(got))
	}
}
