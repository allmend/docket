package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// wantPriorityOrder is the order every priority-sorted list must return.
var wantPriorityOrder = []model.Priority{
	model.PriorityCritical,
	model.PriorityHigh,
	model.PriorityMedium,
	model.PriorityLow,
	model.PriorityNone,
}

// seedPrioritySpread creates one ticket per priority on a single board, assigns
// them all to the given user, and returns the board. Tickets are inserted in an
// order unrelated to their priority so a query that fails to sort is caught.
func seedPrioritySpread(t *testing.T, orgID, userID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	board, err := testStore.CreateBoard(ctx, orgID, userID, nil, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("seed board: %v", err)
	}
	col, err := testStore.CreateColumn(ctx, orgID, board.ID, "To Do", 1000)
	if err != nil {
		t.Fatalf("seed column: %v", err)
	}

	// Insertion order deliberately scrambled relative to wantPriorityOrder.
	insert := []model.Priority{
		model.PriorityMedium,
		model.PriorityNone,
		model.PriorityCritical,
		model.PriorityLow,
		model.PriorityHigh,
	}
	for i, p := range insert {
		ticket, err := testStore.CreateTicket(ctx, orgID, board.ID, col.ID, userID,
			nil, i+1, string(p)+" ticket", "", p, float64((i+1)*1000))
		if err != nil {
			t.Fatalf("seed ticket %q: %v", p, err)
		}
		if err := testStore.AddTicketAssignee(ctx, orgID, ticket.ID, userID); err != nil {
			t.Fatalf("assign ticket %q: %v", p, err)
		}
	}
	return board.ID
}

// TestListTicketsByAssignee_PriorityOrder locks in critical-first ordering.
// priority is a TEXT column, so the previous `ORDER BY priority DESC` sorted it
// alphabetically — medium, low, high, critical — putting the least urgent work
// on top. This test fails against that ordering.
func TestListTicketsByAssignee_PriorityOrder(t *testing.T) {
	requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	seedPrioritySpread(t, org.ID, user.ID)

	tickets, err := testStore.ListTicketsByAssignee(ctx, org.ID, user.ID)
	if err != nil {
		t.Fatalf("list by assignee: %v", err)
	}
	if len(tickets) != len(wantPriorityOrder) {
		t.Fatalf("got %d tickets, want %d", len(tickets), len(wantPriorityOrder))
	}
	for i, want := range wantPriorityOrder {
		if tickets[i].Priority != want {
			got := make([]model.Priority, len(tickets))
			for j, tk := range tickets {
				got[j] = tk.Priority
			}
			t.Fatalf("priority order = %v, want %v", got, wantPriorityOrder)
		}
	}
}

// TestListMyOpenTickets_PriorityOrder covers the dashboard query, which sorts by
// sprint grouping first and priority second.
func TestListMyOpenTickets_PriorityOrder(t *testing.T) {
	requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	seedPrioritySpread(t, org.ID, user.ID)

	rows, err := testStore.ListMyOpenTickets(ctx, org.ID, user.ID)
	if err != nil {
		t.Fatalf("list my open tickets: %v", err)
	}
	if len(rows) != len(wantPriorityOrder) {
		t.Fatalf("got %d rows, want %d", len(rows), len(wantPriorityOrder))
	}
	// DashboardMyTicket.Priority is a plain string, not model.Priority.
	for i, want := range wantPriorityOrder {
		if rows[i].Priority != string(want) {
			got := make([]string, len(rows))
			for j, r := range rows {
				got[j] = r.Priority
			}
			t.Fatalf("priority order = %v, want %v", got, wantPriorityOrder)
		}
	}
}
