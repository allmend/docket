package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// TestMetricsSprintStats_ActiveSprintIsLive verifies the Prometheus sprint series
// reports live progress for an in-flight sprint. The committed/completed columns on
// the sprints row are only written by SnapshotSprintStats at close time, so reading
// them directly made every active sprint report 0 — a flat burndown until the sprint
// ended. Completed sprints must still report their snapshot, because non-done tickets
// leave the sprint at close and a live query would undercount what was committed.
func TestMetricsSprintStats_ActiveSprintIsLive(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	board, err := s.CreateBoard(ctx, org.ID, user.ID, nil, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	todo, err := s.CreateColumn(ctx, org.ID, board.ID, "To Do", 1000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}
	done, err := s.CreateColumn(ctx, org.ID, board.ID, "Done", 2000)
	if err != nil {
		t.Fatalf("create done column: %v", err)
	}

	sprint, err := s.CreateSprint(ctx, org.ID, board.ID, user.ID, "Sprint 1", "", nil, nil)
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	if _, err := s.SetSprintStatus(ctx, org.ID, sprint.ID, model.SprintStatusActive); err != nil {
		t.Fatalf("activate sprint: %v", err)
	}

	// Two tickets in the sprint: 3 points still To Do, 5 points Done.
	three, five := 3.0, 5.0
	todoTicket, err := s.CreateTicket(ctx, org.ID, board.ID, todo.ID, user.ID, nil, 1, "todo work", "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatalf("create todo ticket: %v", err)
	}
	doneTicket, err := s.CreateTicket(ctx, org.ID, board.ID, done.ID, user.ID, nil, 2, "done work", "", model.PriorityMedium, 2000)
	if err != nil {
		t.Fatalf("create done ticket: %v", err)
	}
	if _, err := s.UpdateTicketPoints(ctx, org.ID, todoTicket.ID, &three); err != nil {
		t.Fatalf("set todo points: %v", err)
	}
	if _, err := s.UpdateTicketPoints(ctx, org.ID, doneTicket.ID, &five); err != nil {
		t.Fatalf("set done points: %v", err)
	}
	for _, id := range []uuid.UUID{todoTicket.ID, doneTicket.ID} {
		if err := s.AssignTicketToSprint(ctx, org.ID, id, &sprint.ID); err != nil {
			t.Fatalf("assign to sprint: %v", err)
		}
	}
	// Sprint assignment parks every ticket in the board's first column, so the
	// move into Done has to happen after it — same order as the real flow.
	if err := s.MoveTicket(ctx, org.ID, doneTicket.ID, done.ID, 1000); err != nil {
		t.Fatalf("move to done: %v", err)
	}

	rows, err := s.MetricsSprintStats(ctx, org.ID)
	if err != nil {
		t.Fatalf("sprint stats: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	got := rows[0]
	if got.CommittedTickets != 2 || got.CompletedTickets != 1 {
		t.Errorf("tickets: committed=%d completed=%d, want 2/1", got.CommittedTickets, got.CompletedTickets)
	}
	if got.CommittedPoints != 8 || got.CompletedPoints != 5 {
		t.Errorf("points: committed=%v completed=%v, want 8/5", got.CommittedPoints, got.CompletedPoints)
	}
}

// TestMetricsSprintStats_CompletedKeepsSnapshot verifies a closed sprint still
// reports its snapshot rather than a live recount, which would undercount the
// commitment once non-done tickets have returned to the backlog.
func TestMetricsSprintStats_CompletedKeepsSnapshot(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	board, err := s.CreateBoard(ctx, org.ID, user.ID, nil, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	if _, err := s.CreateColumn(ctx, org.ID, board.ID, "To Do", 1000); err != nil {
		t.Fatalf("create todo column: %v", err)
	}
	done, err := s.CreateColumn(ctx, org.ID, board.ID, "Done", 2000)
	if err != nil {
		t.Fatalf("create done column: %v", err)
	}

	sprint, err := s.CreateSprint(ctx, org.ID, board.ID, user.ID, "Sprint 1", "", nil, nil)
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	if _, err := s.SetSprintStatus(ctx, org.ID, sprint.ID, model.SprintStatusActive); err != nil {
		t.Fatalf("activate sprint: %v", err)
	}

	ticket, err := s.CreateTicket(ctx, org.ID, board.ID, done.ID, user.ID, nil, 1, "shipped", "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	pts := 5.0
	if _, err := s.UpdateTicketPoints(ctx, org.ID, ticket.ID, &pts); err != nil {
		t.Fatalf("set points: %v", err)
	}
	if err := s.AssignTicketToSprint(ctx, org.ID, ticket.ID, &sprint.ID); err != nil {
		t.Fatalf("assign to sprint: %v", err)
	}
	if err := s.MoveTicket(ctx, org.ID, ticket.ID, done.ID, 1000); err != nil {
		t.Fatalf("move to done: %v", err)
	}

	// Close the sprint the way CloseSprint does: snapshot, then release tickets.
	if err := s.SnapshotSprintStats(ctx, org.ID, sprint.ID); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := s.AssignTicketToSprint(ctx, org.ID, ticket.ID, nil); err != nil {
		t.Fatalf("release ticket: %v", err)
	}
	if _, err := s.SetSprintStatus(ctx, org.ID, sprint.ID, model.SprintStatusCompleted); err != nil {
		t.Fatalf("complete sprint: %v", err)
	}

	rows, err := s.MetricsSprintStats(ctx, org.ID)
	if err != nil {
		t.Fatalf("sprint stats: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	// The ticket has left the sprint — a live recount would report 0/0.
	if rows[0].CommittedPoints != 5 || rows[0].CompletedPoints != 5 {
		t.Errorf("completed sprint lost its snapshot: committed=%v completed=%v, want 5/5",
			rows[0].CommittedPoints, rows[0].CompletedPoints)
	}
}
