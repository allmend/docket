package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestColumnMetaNumberHistory_OrgIsolation pins the org filter on three
// methods that used to trust their caller to have resolved the parent first.
func TestColumnMetaNumberHistory_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	ticket := seedTicket(t, orgA.ID, userA.ID, "ticket")

	// GetColumnMeta: another org cannot read a column's name.
	if _, _, err := s.GetColumnMeta(ctx, orgB.ID, ticket.ColumnID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("cross-org GetColumnMeta: got %v, want ErrNoRows", err)
	}
	if name, _, err := s.GetColumnMeta(ctx, orgA.ID, ticket.ColumnID); err != nil || name != "To Do" {
		t.Errorf("own GetColumnMeta: got %q, %v", name, err)
	}

	// NextTicketNumber: another org cannot advance a team's counter.
	team, err := s.CreateTeam(ctx, orgA.ID, userA.ID, "Team", "TM", "team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := s.NextTicketNumber(ctx, orgB.ID, team.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("cross-org NextTicketNumber: got %v, want ErrNoRows", err)
	}
	if n, err := s.NextTicketNumber(ctx, orgA.ID, team.ID); err != nil || n != 1 {
		t.Errorf("own NextTicketNumber: got %d, %v; want 1 (the cross-org call must not have counted)", n, err)
	}

	// AppendHistory: another org cannot write onto the ticket's history.
	if err := s.AppendHistory(ctx, orgB.ID, ticket.ID, userB.ID, "bob", "title", "a", "b"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("cross-org AppendHistory: got %v, want ErrNoRows", err)
	}
	if err := s.AppendHistory(ctx, orgA.ID, ticket.ID, userA.ID, "alice", "title", "a", "b"); err != nil {
		t.Errorf("own AppendHistory: %v", err)
	}
	if h, _ := s.ListHistory(ctx, orgA.ID, ticket.ID); len(h) != 1 {
		t.Errorf("history: got %d entries, want 1", len(h))
	}
}
