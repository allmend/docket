package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
)

// TestBlockingLinksFromTicket_OrgIsolation covers the close-time auto-unblock
// path: listing and deleting a closing ticket's outbound "blocks" links must
// be scoped to the caller's org.
func TestBlockingLinksFromTicket_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	blocker := seedTicket(t, orgA.ID, userA.ID, "blocker")
	blocked := seedTicket(t, orgA.ID, userA.ID, "blocked")

	if _, err := s.CreateLink(ctx, orgA.ID, blocker.ID, blocked.ID, model.RelationBlocks); err != nil {
		t.Fatalf("setup link: %v", err)
	}

	// Wrong org sees nothing.
	links, err := s.ListBlockingLinksFromTicket(ctx, orgB.ID, blocker.ID)
	if err != nil {
		t.Fatalf("list (wrong org): %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("cross-org list leaked %d links, want 0", len(links))
	}

	// Wrong org deletes nothing.
	if err := s.DeleteBlockingLinksFromTicket(ctx, orgB.ID, blocker.ID); err != nil {
		t.Fatalf("delete (wrong org): %v", err)
	}
	links, _ = s.ListBlockingLinksFromTicket(ctx, orgA.ID, blocker.ID)
	if len(links) != 1 {
		t.Fatalf("cross-org delete removed the link: got %d, want 1", len(links))
	}
	if links[0].ToTicketID != blocked.ID {
		t.Fatalf("listed link points at %s, want %s", links[0].ToTicketID, blocked.ID)
	}

	// Owner delete clears the link — the blocked ticket is unblocked.
	if err := s.DeleteBlockingLinksFromTicket(ctx, orgA.ID, blocker.ID); err != nil {
		t.Fatalf("delete (in org): %v", err)
	}
	links, _ = s.ListBlockingLinksFromTicket(ctx, orgA.ID, blocker.ID)
	if len(links) != 0 {
		t.Fatalf("in-org delete failed: %d links remain", len(links))
	}
}

// TestDeleteBlockingLinksFromTicket_OnlyBlocks asserts the close-time cleanup
// touches only "blocks" links — relates_to/depends_on links must survive.
func TestDeleteBlockingLinksFromTicket_OnlyBlocks(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	userA := seedUser(t, orgA.ID, "alice")
	closing := seedTicket(t, orgA.ID, userA.ID, "closing")
	other := seedTicket(t, orgA.ID, userA.ID, "other")

	if _, err := s.CreateLink(ctx, orgA.ID, closing.ID, other.ID, model.RelationBlocks); err != nil {
		t.Fatalf("setup blocks link: %v", err)
	}
	if _, err := s.CreateLink(ctx, orgA.ID, closing.ID, other.ID, model.RelationRelatesTo); err != nil {
		t.Fatalf("setup relates_to link: %v", err)
	}
	// An inbound "blocks" link (someone else blocks the closing ticket) must survive too.
	if _, err := s.CreateLink(ctx, orgA.ID, other.ID, closing.ID, model.RelationBlocks); err != nil {
		t.Fatalf("setup inbound blocks link: %v", err)
	}

	if err := s.DeleteBlockingLinksFromTicket(ctx, orgA.ID, closing.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	links, err := s.ListLinks(ctx, orgA.ID, closing.ID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("got %d links after delete, want 2 (relates_to + inbound blocks)", len(links))
	}
	for _, l := range links {
		if l.Relation == model.RelationBlocks && l.FromTicketID == closing.ID {
			t.Fatal("outbound blocks link survived the delete")
		}
	}
}
