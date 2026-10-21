package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/allmend/docket/internal/model"
)

// "Completed" means in Done and closed as done. Only done closes move a ticket
// to Done, but a ticket can still end up there with another reason (closed
// for one reason, then dragged), and it was not delivered: it must not count
// towards completed tickets or points (velocity).
func TestCompletedCountsOnlyClosedAsDone(t *testing.T) {
	st := requireStore(t)
	ctx := context.Background()
	org := seedOrg(t, "completed-reason")
	user := seedUser(t, org.ID, "completed-reason-user")
	board, err := st.CreateBoard(ctx, org.ID, user.ID, nil, "B", "", model.BoardModeScrum)
	if err != nil {
		t.Fatal(err)
	}
	todo, _ := st.CreateColumn(ctx, org.ID, board.ID, "To Do", 1000)
	done, _ := st.CreateColumn(ctx, org.ID, board.ID, "Done", 2000)
	start, end := time.Now().Add(-time.Hour), time.Now().Add(240*time.Hour)
	sp, err := st.CreateSprint(ctx, org.ID, board.ID, user.ID, "S", "", &start, &end)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetSprintStatus(ctx, org.ID, sp.ID, model.SprintStatusActive); err != nil {
		t.Fatal(err)
	}
	mk := func(title string, pts float64, reason string) {
		t.Helper()
		tk, err := st.CreateTicket(ctx, org.ID, board.ID, todo.ID, user.ID, nil, 0, title, "", model.PriorityMedium, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.AssignTicketToSprint(ctx, org.ID, tk.ID, &sp.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := st.UpdateTicketPoints(ctx, org.ID, tk.ID, &pts); err != nil {
			t.Fatal(err)
		}
		if reason == "" {
			return
		}
		// After the sprint assignment, which parks tickets in the first column.
		if err := st.MoveTicket(ctx, org.ID, tk.ID, done.ID, 1000); err != nil {
			t.Fatal(err)
		}
		if reason != "legacy" {
			if _, err := st.CloseTicket(ctx, org.ID, tk.ID, reason); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("delivered", 3, model.CloseReasonDone)
	mk("in done before reasons existed", 2, "legacy")
	mk("dropped", 5, model.CloseReasonWontDo)
	mk("open", 8, "")

	got, err := st.GetSprint(ctx, org.ID, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedTickets != 2 || got.CompletedPoints != 5 {
		t.Errorf("live: completed %d tickets / %v points, want 2 / 5", got.CompletedTickets, got.CompletedPoints)
	}
	if got.CommittedPoints != 18 {
		t.Errorf("live: committed %v points, want 18", got.CommittedPoints)
	}

	// The snapshot taken at sprint close counts the same way.
	if err := st.SnapshotSprintStats(ctx, org.ID, sp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetSprintStatus(ctx, org.ID, sp.ID, model.SprintStatusCompleted); err != nil {
		t.Fatal(err)
	}
	snap, err := st.GetSprint(ctx, org.ID, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CompletedTickets != 2 || snap.CompletedPoints != 5 {
		t.Errorf("snapshot: completed %d tickets / %v points, want 2 / 5", snap.CompletedTickets, snap.CompletedPoints)
	}

	completed, returning, err := st.ListSprintTicketsForReview(ctx, org.ID, sp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 2 || len(returning) != 1 {
		t.Errorf("review: %d completed, %d returning; want 2, 1", len(completed), len(returning))
	}
}

// Migration 000028 ran on the test database before any test data existed, so
// replay its statement here against closed tickets outside Done.
func TestMigration028MovesClosedTicketsIntoDone(t *testing.T) {
	st := requireStore(t)
	ctx := context.Background()
	org := seedOrg(t, "mig028")
	user := seedUser(t, org.ID, "mig028-user")
	board, _ := st.CreateBoard(ctx, org.ID, user.ID, nil, "B", "", model.BoardModeKanban)
	backlog, _ := st.CreateColumn(ctx, org.ID, board.ID, "Backlog", 1000)
	done, _ := st.CreateColumn(ctx, org.ID, board.ID, "Done", 2000)
	open, _ := st.CreateTicket(ctx, org.ID, board.ID, backlog.ID, user.ID, nil, 0, "open", "", model.PriorityMedium, 1000)
	shut, _ := st.CreateTicket(ctx, org.ID, board.ID, backlog.ID, user.ID, nil, 0, "closed", "", model.PriorityMedium, 2000)
	if _, err := st.CloseTicket(ctx, org.ID, shut.ID, model.CloseReasonDone); err != nil {
		t.Fatal(err)
	}
	dropped, _ := st.CreateTicket(ctx, org.ID, board.ID, backlog.ID, user.ID, nil, 0, "won't do", "", model.PriorityMedium, 3000)
	if _, err := st.CloseTicket(ctx, org.ID, dropped.ID, model.CloseReasonWontDo); err != nil {
		t.Fatal(err)
	}
	blank, _ := st.CreateBoard(ctx, org.ID, user.ID, nil, "Blank", "", model.BoardModeBlank)
	only, _ := st.CreateColumn(ctx, org.ID, blank.ID, "Ideas", 1000)
	stays, _ := st.CreateTicket(ctx, org.ID, blank.ID, only.ID, user.ID, nil, 0, "no done column", "", model.PriorityMedium, 1000)
	if _, err := st.CloseTicket(ctx, org.ID, stays.ID, model.CloseReasonDone); err != nil {
		t.Fatal(err)
	}

	sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000028_closed_tickets_in_done.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migration: %v", err)
	}
	for _, c := range []struct {
		id   uuid.UUID
		want uuid.UUID
		name string
	}{{open.ID, backlog.ID, "open ticket"}, {shut.ID, done.ID, "closed as done"}, {dropped.ID, backlog.ID, "closed as won't do"}, {stays.ID, only.ID, "closed ticket on a board without Done"}} {
		got, err := st.GetTicket(ctx, org.ID, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if got.ColumnID != c.want {
			t.Errorf("%s: in column %s, want %s", c.name, got.ColumnID, c.want)
		}
	}
}
