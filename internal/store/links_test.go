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
// touches only "blocks" links — links of any other relation must survive.
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

// TestListLinks_RelationVocabulary runs the whole relation set through the
// database and back, checking each one flips to its inverse from the far side.
func TestListLinks_RelationVocabulary(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	from := seedTicket(t, org.ID, user.ID, "source")
	to := seedTicket(t, org.ID, user.ID, "target")

	cases := []struct {
		relation model.RelationType
		inverse  model.RelationType
	}{
		{model.RelationBlocks, model.RelationBlockedBy},
		{model.RelationDependsOn, model.RelationRequiredBy},
		{model.RelationDuplicates, model.RelationDuplicatedBy},
		{model.RelationRelatesTo, model.RelationRelatesTo}, // symmetric
	}

	for _, c := range cases {
		if _, err := s.CreateLink(ctx, org.ID, from.ID, to.ID, c.relation); err != nil {
			t.Fatalf("create %s link: %v", c.relation, err)
		}
	}

	// Source side: relations read forward, pointing at the target.
	forward, err := s.ListLinks(ctx, org.ID, from.ID)
	if err != nil {
		t.Fatalf("list links (source): %v", err)
	}
	if len(forward) != len(cases) {
		t.Fatalf("source sees %d links, want %d", len(forward), len(cases))
	}
	seen := make(map[model.RelationType]bool, len(forward))
	for _, l := range forward {
		seen[l.Relation] = true
		if l.ToTicketID != to.ID {
			t.Errorf("%s points at %s, want the target ticket %s", l.Relation, l.ToTicketID, to.ID)
		}
	}
	for _, c := range cases {
		if !seen[c.relation] {
			t.Errorf("source is missing a %s link", c.relation)
		}
	}

	// Target side: each relation is rewritten to its inverse, pointing back.
	inverse, err := s.ListLinks(ctx, org.ID, to.ID)
	if err != nil {
		t.Fatalf("list links (target): %v", err)
	}
	seenInverse := make(map[model.RelationType]bool, len(inverse))
	for _, l := range inverse {
		seenInverse[l.Relation] = true
		// The rewrite must swap both ends, not just the relation name.
		if l.ToTicketID != from.ID {
			t.Errorf("%s points at %s, want the source ticket %s", l.Relation, l.ToTicketID, from.ID)
		}
	}
	for _, c := range cases {
		if !seenInverse[c.inverse] {
			t.Errorf("target is missing a %s link (inverse of %s)", c.inverse, c.relation)
		}
	}
}

// TestCreateLink_RejectsUnknownRelation covers the CHECK constraint refusing a
// relation outside the vocabulary. "clones" was considered and turned down.
func TestCreateLink_RejectsUnknownRelation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	from := seedTicket(t, org.ID, user.ID, "source")
	to := seedTicket(t, org.ID, user.ID, "target")

	if _, err := s.CreateLink(ctx, org.ID, from.ID, to.ID, model.RelationType("clones")); err == nil {
		t.Fatal("clones link was accepted; the CHECK constraint should reject it")
	}
}

// TestBulkGetWaitingOn covers the amber dependency marker: shown while the
// depended-on ticket is open, gone once it closes, and the link survives either
// way. Blocking links behave differently and are covered above.
func TestBulkGetWaitingOn(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	user := seedUser(t, orgA.ID, "alice")
	dependent := seedTicket(t, orgA.ID, user.ID, "dependent")
	dependency := seedTicket(t, orgA.ID, user.ID, "dependency")

	// depends_on stores the dependent as the source, the opposite way round to blocks.
	if _, err := s.CreateLink(ctx, orgA.ID, dependent.ID, dependency.ID, model.RelationDependsOn); err != nil {
		t.Fatalf("create depends_on link: %v", err)
	}

	waiting, err := s.BulkGetWaitingOn(ctx, orgA.ID, dependent.BoardID)
	if err != nil {
		t.Fatalf("bulk waiting on: %v", err)
	}
	if _, ok := waiting[dependent.ID]; !ok {
		t.Fatal("dependent is not marked as waiting while its dependency is open")
	}
	if _, ok := waiting[dependency.ID]; ok {
		t.Error("the dependency itself was marked as waiting; only the dependent waits")
	}

	// Cross-org callers see nothing.
	crossOrg, err := s.BulkGetWaitingOn(ctx, orgB.ID, dependent.BoardID)
	if err != nil {
		t.Fatalf("bulk waiting on (cross-org): %v", err)
	}
	if len(crossOrg) != 0 {
		t.Errorf("cross-org waiting map leaked %d entries, want 0", len(crossOrg))
	}

	// Closing the dependency clears the marker but leaves the link alone.
	if _, err := s.CloseTicket(ctx, orgA.ID, dependency.ID, "done"); err != nil {
		t.Fatalf("close dependency: %v", err)
	}
	waiting, err = s.BulkGetWaitingOn(ctx, orgA.ID, dependent.BoardID)
	if err != nil {
		t.Fatalf("bulk waiting on (after close): %v", err)
	}
	if _, ok := waiting[dependent.ID]; ok {
		t.Error("dependent still marked as waiting after its dependency closed")
	}
	links, err := s.ListLinks(ctx, orgA.ID, dependent.ID)
	if err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("dependency link count = %d, want 1 (links are never auto-deleted)", len(links))
	}
}

