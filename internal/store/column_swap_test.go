package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSwapColumnPositions checks the column reorder: two columns on one board
// trade positions, and the guard refuses a column from another org or another
// board without changing anything.
func TestSwapColumnPositions(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	ticketA := seedTicket(t, orgA.ID, userA.ID, "a")       // board A1, column "To Do" at 1000
	ticketA2 := seedTicket(t, orgA.ID, userA.ID, "a2")     // board A2 in the same org
	ticketB := seedTicket(t, orgB.ID, userB.ID, "foreign") // board in org B

	todo := ticketA.ColumnID
	done, err := s.CreateColumn(ctx, orgA.ID, ticketA.BoardID, "Done", 2000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}

	positions := func() (float64, float64) {
		t.Helper()
		a, err := s.GetColumn(ctx, orgA.ID, todo)
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.GetColumn(ctx, orgA.ID, done.ID)
		if err != nil {
			t.Fatal(err)
		}
		return a.Position, b.Position
	}

	if err := s.SwapColumnPositions(ctx, orgA.ID, ticketA.BoardID, todo, done.ID); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if a, b := positions(); a != 2000 || b != 1000 {
		t.Fatalf("after swap: todo=%v done=%v, want 2000/1000", a, b)
	}

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"column from another org", s.SwapColumnPositions(ctx, orgA.ID, ticketA.BoardID, todo, ticketB.ColumnID)},
		{"column from another board", s.SwapColumnPositions(ctx, orgA.ID, ticketA.BoardID, todo, ticketA2.ColumnID)},
		{"caller from another org", s.SwapColumnPositions(ctx, orgB.ID, ticketA.BoardID, todo, done.ID)},
	} {
		if !errors.Is(tc.err, pgx.ErrNoRows) {
			t.Errorf("%s: got %v, want pgx.ErrNoRows", tc.name, tc.err)
		}
	}
	if a, b := positions(); a != 2000 || b != 1000 {
		t.Fatalf("refused swaps changed positions: todo=%v done=%v", a, b)
	}
}
