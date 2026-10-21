package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
)

// A column delete only cares about tickets the board shows. Tickets that left
// a sprint keep their column_id, so they are counted separately and can be
// reassigned out of the way.
func TestCountColumnTicketsAndReassign(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	other := seedOrg(t, "org-b")
	user := seedUser(t, org.ID, "alice")
	base := seedTicket(t, org.ID, user.ID, "base")
	review, err := s.CreateColumn(ctx, org.ID, base.BoardID, "Review", 2000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}
	inReview := func(title string, number int) *model.Ticket {
		tk, err := s.CreateTicket(ctx, org.ID, base.BoardID, review.ID, user.ID, nil, number, title, "", model.PriorityMedium, float64(number*1000))
		if err != nil {
			t.Fatalf("seed %s: %v", title, err)
		}
		return tk
	}
	sprint := func(name string) *model.Sprint {
		sp, err := s.CreateSprint(ctx, org.ID, base.BoardID, user.ID, name, "", nil, nil)
		if err != nil {
			t.Fatalf("sprint %s: %v", name, err)
		}
		return sp
	}

	active := sprint("active")
	if _, err := s.SetSprintStatus(ctx, org.ID, active.ID, model.SprintStatusActive); err != nil {
		t.Fatal(err)
	}
	finished := sprint("finished")

	// AssignTicketToSprint parks a ticket in the board's first column, so each
	// one moves back to Review after the assignment.
	toSprint := func(tk *model.Ticket, sp *model.Sprint) {
		if err := s.AssignTicketToSprint(ctx, org.ID, tk.ID, &sp.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.MoveTicket(ctx, org.ID, tk.ID, review.ID, tk.Position); err != nil {
			t.Fatal(err)
		}
	}
	toSprint(inReview("in active sprint", 2), active)
	inReview("in backlog", 3)
	toSprint(inReview("in finished sprint", 4), finished)
	if _, err := s.SetSprintStatus(ctx, org.ID, finished.ID, model.SprintStatusCompleted); err != nil {
		t.Fatal(err)
	}

	got, err := s.CountColumnTickets(ctx, org.ID, review.ID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	want := ColumnTicketCounts{ActiveSprint: 1, Hidden: 2}
	if got != want {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
	if got, _ := s.CountColumnTickets(ctx, other.ID, review.ID); got != (ColumnTicketCounts{}) {
		t.Fatalf("another org sees counts %+v", got)
	}

	// Another org can't move this org's tickets.
	if err := s.ReassignColumnTickets(ctx, other.ID, review.ID, base.ColumnID); err != nil {
		t.Fatalf("foreign reassign: %v", err)
	}
	if got, _ := s.CountColumnTickets(ctx, org.ID, review.ID); got != want {
		t.Fatalf("foreign reassign moved tickets: %+v", got)
	}

	if err := s.ReassignColumnTickets(ctx, org.ID, review.ID, base.ColumnID); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if got, _ := s.CountColumnTickets(ctx, org.ID, review.ID); got != (ColumnTicketCounts{}) {
		t.Fatalf("after reassign counts = %+v, want none", got)
	}
	if err := s.DeleteColumn(ctx, org.ID, review.ID); err != nil {
		t.Fatalf("delete emptied column: %v", err)
	}
}

func TestFlowSummary(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	other := seedOrg(t, "org-b")
	user := seedUser(t, org.ID, "alice")
	open := seedTicket(t, org.ID, user.ID, "open") // "To Do", 1000
	done, err := s.CreateColumn(ctx, org.ID, open.BoardID, "Done", 2000)
	if err != nil {
		t.Fatal(err)
	}
	pts := 3.0
	if _, err := s.UpdateTicketPoints(ctx, org.ID, open.ID, &pts); err != nil {
		t.Fatal(err)
	}
	finished, err := s.CreateTicket(ctx, org.ID, open.BoardID, done.ID, user.ID, nil, 2, "finished", "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CloseTicket(ctx, org.ID, finished.ID, "done"); err != nil {
		t.Fatal(err)
	}

	f, err := s.FlowSummary(ctx, org.ID, open.BoardID)
	if err != nil {
		t.Fatalf("flow: %v", err)
	}
	want := []model.FlowColumn{{Name: "To Do", Open: 1, OpenPoints: 3}, {Name: "Done", Open: 0}}
	if len(f.Columns) != len(want) || f.Columns[0] != want[0] || f.Columns[1] != want[1] {
		t.Fatalf("columns = %+v, want %+v", f.Columns, want)
	}
	if f.DoneLast7d != 1 {
		t.Errorf("DoneLast7d = %d, want 1", f.DoneLast7d)
	}

	foreign, err := s.FlowSummary(ctx, other.ID, open.BoardID)
	if err != nil {
		t.Fatalf("foreign flow: %v", err)
	}
	if len(foreign.Columns) != 0 || foreign.DoneLast7d != 0 {
		t.Errorf("another org sees this board's flow: %+v", foreign)
	}
}
