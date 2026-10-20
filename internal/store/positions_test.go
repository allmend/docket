package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func TestRenumberTicketPositions(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")

	first := seedTicket(t, orgA.ID, userA.ID, "first")
	more := func(title string, number int, pos float64) *model.Ticket {
		tk, err := s.CreateTicket(ctx, orgA.ID, first.BoardID, first.ColumnID, userA.ID, nil, number, title, "", model.PriorityMedium, pos)
		if err != nil {
			t.Fatalf("seed %s: %v", title, err)
		}
		return tk
	}
	second := more("second", 2, 1000.0004)
	third := more("third", 3, 1000.0008)
	foreign := seedTicket(t, orgB.ID, userB.ID, "foreign")

	// The foreign ID must be skipped, not renumbered.
	order := []uuid.UUID{third.ID, first.ID, second.ID, foreign.ID}
	if err := s.RenumberTicketPositions(ctx, orgA.ID, order); err != nil {
		t.Fatalf("renumber: %v", err)
	}

	want := map[uuid.UUID]float64{third.ID: 1000, first.ID: 2000, second.ID: 3000}
	for id, pos := range want {
		got, err := s.GetTicket(ctx, orgA.ID, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Position != pos {
			t.Errorf("%s: position %v, want %v", got.Title, got.Position, pos)
		}
	}
	if got, _ := s.GetTicket(ctx, orgB.ID, foreign.ID); got.Position != foreign.Position {
		t.Errorf("another org's ticket was renumbered to %v", got.Position)
	}
	if err := s.RenumberTicketPositions(ctx, orgA.ID, nil); err != nil {
		t.Errorf("empty renumber: %v", err)
	}
}
