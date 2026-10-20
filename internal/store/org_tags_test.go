package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestListOrgTags_OrgIsolation checks the org-wide tag list behind the New
// Ticket dialog's track picker returns every board's tags in the caller's org
// and nothing from another org.
func TestListOrgTags_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	ticketA1 := seedTicket(t, orgA.ID, userA.ID, "first board")
	ticketA2 := seedTicket(t, orgA.ID, userA.ID, "second board")
	ticketB := seedTicket(t, orgB.ID, userB.ID, "other org")

	for _, tc := range []struct {
		org, board uuid.UUID
		name       string
	}{
		{orgA.ID, ticketA1.BoardID, "api"},
		{orgA.ID, ticketA2.BoardID, "ui"},
		{orgB.ID, ticketB.BoardID, "secret"},
	} {
		if _, err := s.CreateTag(ctx, tc.org, tc.board, tc.name, "#8957e5", "", nil); err != nil {
			t.Fatalf("create tag %q: %v", tc.name, err)
		}
	}

	tags, err := s.ListOrgTags(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("list org tags: %v", err)
	}
	names := map[string]bool{}
	for _, tag := range tags {
		if tag.OrgID != orgA.ID {
			t.Fatalf("tag %q from another org leaked", tag.Name)
		}
		names[tag.Name] = true
	}
	if len(tags) != 2 || !names["api"] || !names["ui"] {
		t.Fatalf("got %v, want both of org A's boards' tags", names)
	}
}
