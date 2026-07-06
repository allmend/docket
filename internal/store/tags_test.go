package store

import (
	"context"
	"testing"
)

// TestTrackStats_OrgIsolationAndCounters covers the settings Tracks panel
// queries: CreateTag/UpdateTag with description+lead, and ListTrackStats
// counting only open tickets and only within the caller's org.
func TestTrackStats_OrgIsolationAndCounters(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticket := seedTicket(t, orgA.ID, userA.ID, "open work")

	tag, err := s.CreateTag(ctx, orgA.ID, ticket.BoardID, "auth", "#8957e5", "Sessions & SSO", &userA.ID)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.Description != "Sessions & SSO" || tag.LeadUserID == nil || *tag.LeadUserID != userA.ID {
		t.Fatalf("create tag lost details: %+v", tag)
	}

	if err := s.AddTagToTicket(ctx, orgA.ID, ticket.ID, tag.ID); err != nil {
		t.Fatalf("tag ticket: %v", err)
	}
	pts := 5.0
	if _, err := s.UpdateTicketPoints(ctx, orgA.ID, ticket.ID, &pts); err != nil {
		t.Fatalf("set points: %v", err)
	}

	stats, err := s.ListTrackStats(ctx, orgA.ID, ticket.BoardID)
	if err != nil {
		t.Fatalf("list stats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("got %d tracks, want 1", len(stats))
	}
	st := stats[0]
	if st.LeadName != "alice" || st.OpenCount != 1 || st.OpenPoints != 5 {
		t.Fatalf("bad counters: lead=%q open=%d points=%g", st.LeadName, st.OpenCount, st.OpenPoints)
	}

	// Closed tickets drop out of the open counters.
	if _, err := s.CloseTicket(ctx, orgA.ID, ticket.ID, "done"); err != nil {
		t.Fatalf("close: %v", err)
	}
	stats, _ = s.ListTrackStats(ctx, orgA.ID, ticket.BoardID)
	if stats[0].OpenCount != 0 || stats[0].OpenPoints != 0 {
		t.Fatalf("closed ticket still counted: open=%d points=%g", stats[0].OpenCount, stats[0].OpenPoints)
	}

	// Cross-org: another org sees nothing and cannot update the tag.
	if cross, _ := s.ListTrackStats(ctx, orgB.ID, ticket.BoardID); len(cross) != 0 {
		t.Fatalf("cross-org stats leaked %d tracks", len(cross))
	}
	if _, err := s.UpdateTag(ctx, orgB.ID, tag.ID, "stolen", "#ff0000", "", nil); err == nil {
		t.Fatal("cross-org UpdateTag succeeded — org isolation breach")
	}

	// In-org update persists all fields.
	upd, err := s.UpdateTag(ctx, orgA.ID, tag.ID, "identity", "#39c5cf", "Auth & identity", nil)
	if err != nil {
		t.Fatalf("update tag: %v", err)
	}
	if upd.Name != "identity" || upd.Color != "#39c5cf" || upd.Description != "Auth & identity" || upd.LeadUserID != nil {
		t.Fatalf("update lost fields: %+v", upd)
	}
}
