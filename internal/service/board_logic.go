package service

import (
	"fmt"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

// inFlightTickets is a Kanban or Blank board's stand-up work set: open tickets
// that have been started, i.e. past the first column (where new work lands)
// and not in Done. cols is the board's columns in board order.
func inFlightTickets(tickets []model.Ticket, cols []model.Column) []model.Ticket {
	var first uuid.UUID
	if len(cols) > 0 {
		first = cols[0].ID
	}
	done := make(map[uuid.UUID]bool)
	for _, c := range cols {
		if isDone(c.Name) {
			done[c.ID] = true
		}
	}
	var out []model.Ticket
	for _, t := range tickets {
		if t.ClosedAt != nil || t.ColumnID == first || done[t.ColumnID] {
			continue
		}
		out = append(out, t)
	}
	return out
}

// describeColumnTickets says where the visible tickets blocking a column
// delete are, e.g. "2 in the active sprint, 1 in sprint planning".
func describeColumnTickets(c store.ColumnTicketCounts) string {
	var parts []string
	add := func(n int, where string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, where))
		}
	}
	add(c.ActiveSprint, "in the active sprint")
	add(c.PlanningSprint, "in sprint planning")
	add(c.OnBoard, "on the board")
	return strings.Join(parts, ", ")
}
