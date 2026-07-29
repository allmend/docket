package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
)

// TestBulkListTicketAssignees_OrgIsolation verifies the board-scoped bulk
// assignee load also filters by org — a caller passing another org's ID must
// see nothing for the same board ID.
func TestBulkListTicketAssignees_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticket := seedTicket(t, orgA.ID, userA.ID, "work")

	if err := s.AddTicketAssignee(ctx, orgA.ID, ticket.ID, userA.ID); err != nil {
		t.Fatalf("add assignee: %v", err)
	}

	inOrg, err := s.BulkListTicketAssignees(ctx, orgA.ID, ticket.BoardID)
	if err != nil {
		t.Fatalf("bulk list (in-org): %v", err)
	}
	if got := len(inOrg[ticket.ID]); got != 1 {
		t.Fatalf("in-org assignees = %d, want 1", got)
	}

	crossOrg, err := s.BulkListTicketAssignees(ctx, orgB.ID, ticket.BoardID)
	if err != nil {
		t.Fatalf("bulk list (cross-org): %v", err)
	}
	if got := len(crossOrg[ticket.ID]); got != 0 {
		t.Fatalf("cross-org assignees leaked: got %d, want 0", got)
	}
}

// TestBulkListTicketTags_OrgIsolation verifies the board-scoped bulk tag load
// also filters by org.
func TestBulkListTicketTags_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticket := seedTicket(t, orgA.ID, userA.ID, "work")

	tag, err := s.CreateTag(ctx, orgA.ID, ticket.BoardID, "auth", "#8957e5", "", nil)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := s.AddTagToTicket(ctx, orgA.ID, ticket.ID, tag.ID); err != nil {
		t.Fatalf("tag ticket: %v", err)
	}

	inOrg, err := s.BulkListTicketTags(ctx, orgA.ID, ticket.BoardID)
	if err != nil {
		t.Fatalf("bulk tags (in-org): %v", err)
	}
	if got := len(inOrg[ticket.ID]); got != 1 {
		t.Fatalf("in-org tags = %d, want 1", got)
	}

	crossOrg, err := s.BulkListTicketTags(ctx, orgB.ID, ticket.BoardID)
	if err != nil {
		t.Fatalf("bulk tags (cross-org): %v", err)
	}
	if got := len(crossOrg[ticket.ID]); got != 0 {
		t.Fatalf("cross-org tags leaked: got %d, want 0", got)
	}
}

// TestBulkListTicketTagsByTeam_OrgIsolation checks the team-scoped bulk tag load
// behind the API ticket list filters by org as well as team.
func TestBulkListTicketTagsByTeam_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")

	team, err := s.CreateTeam(ctx, orgA.ID, userA.ID, "Work", "WRK", "work", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	board, err := s.CreateBoard(ctx, orgA.ID, userA.ID, &team.ID, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	col, err := s.CreateColumn(ctx, orgA.ID, board.ID, "To Do", 1000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}
	ticket, err := s.CreateTicket(ctx, orgA.ID, board.ID, col.ID, userA.ID, &team.ID, 1, "work", "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	tag, err := s.CreateTag(ctx, orgA.ID, board.ID, "docket", "#a3e635", "", nil)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := s.AddTagToTicket(ctx, orgA.ID, ticket.ID, tag.ID); err != nil {
		t.Fatalf("tag ticket: %v", err)
	}

	inOrg, err := s.BulkListTicketTagsByTeam(ctx, orgA.ID, team.ID)
	if err != nil {
		t.Fatalf("bulk tags by team (in-org): %v", err)
	}
	if got := len(inOrg[ticket.ID]); got != 1 {
		t.Fatalf("in-org tags = %d, want 1", got)
	}
	if inOrg[ticket.ID][0].Name != "docket" {
		t.Fatalf("in-org tag name = %q, want %q", inOrg[ticket.ID][0].Name, "docket")
	}

	crossOrg, err := s.BulkListTicketTagsByTeam(ctx, orgB.ID, team.ID)
	if err != nil {
		t.Fatalf("bulk tags by team (cross-org): %v", err)
	}
	if got := len(crossOrg[ticket.ID]); got != 0 {
		t.Fatalf("cross-org tags leaked: got %d, want 0", got)
	}
}

// TestCreateTag_CrossOrgLeadDropped verifies a lead_user_id belonging to
// another org is stored as NULL rather than referencing a foreign tenant's user.
func TestCreateTag_CrossOrgLeadDropped(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	foreign := seedUser(t, orgB.ID, "mallory")
	ticket := seedTicket(t, orgA.ID, userA.ID, "work")

	tag, err := s.CreateTag(ctx, orgA.ID, ticket.BoardID, "auth", "#8957e5", "", &foreign.ID)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.LeadUserID != nil {
		t.Fatalf("cross-org lead stored: %v, want nil", *tag.LeadUserID)
	}

	// In-org lead is retained.
	if _, err := s.UpdateTag(ctx, orgA.ID, tag.ID, "auth", "#8957e5", "", &userA.ID); err != nil {
		t.Fatalf("update tag: %v", err)
	}
	upd, err := s.UpdateTag(ctx, orgA.ID, tag.ID, "auth", "#8957e5", "", &foreign.ID)
	if err != nil {
		t.Fatalf("update tag: %v", err)
	}
	if upd.LeadUserID != nil {
		t.Fatalf("cross-org lead stored on update: %v, want nil", *upd.LeadUserID)
	}
}
