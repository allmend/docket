package service

import (
	"fmt"
	"strings"

	"github.com/allmend/docket/internal/store"
)

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
