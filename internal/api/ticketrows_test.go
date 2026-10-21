package api

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// Every surface that lists tickets marks a blocked ticket the same way, so it
// keeps its marker when dragged from one list to another. And the Backlog
// page's sprint sections render the same draggable row as the backlog itself:
// they used to have their own markup without a drag handle, so a ticket
// dropped into a sprint couldn't be dragged back out after a reload.
func TestTicketRowsShareBlockerMarkersAndDragHandle(t *testing.T) {
	tmpls, err := parseTemplates("../../templates")
	if err != nil {
		t.Fatalf("parseTemplates: %v", err)
	}
	set := tmpls["backlog.html"]
	if set == nil {
		t.Fatal("backlog.html missing from parsed set")
	}

	blocked := model.Ticket{ID: uuid.New(), TeamKey: "BE", Number: 7, Title: "stuck", IsBlocked: true, BlockedBy: "BE-1"}
	waiting := model.Ticket{ID: uuid.New(), TeamKey: "BE", Number: 8, Title: "later", IsWaiting: true, WaitingOn: "BE-2"}
	closedAt := time.Now()
	done := model.Ticket{ID: uuid.New(), TeamKey: "BE", Number: 9, Title: "done", ClosedAt: &closedAt, IsBlocked: true, BlockedBy: "BE-3"}

	render := func(name string, data any) string {
		t.Helper()
		var buf bytes.Buffer
		if err := set.ExecuteTemplate(&buf, name, data); err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
		return buf.String()
	}

	surfaces := map[string]func(model.Ticket) string{
		"backlog-row": func(tk model.Ticket) string {
			return render("backlog-row", map[string]any{"Ticket": tk, "Rank": 1, "Draggable": true})
		},
		"backlog-planning-card": func(tk model.Ticket) string { return render("backlog-planning-card", tk) },
		"board-card": func(tk model.Ticket) string {
			return render("board-card", map[string]any{"Ticket": tk, "IsDone": false})
		},
	}
	for name, draw := range surfaces {
		if html := draw(blocked); !strings.Contains(html, "Blocked by BE-1") || !strings.Contains(html, "text-danger") {
			t.Errorf("%s: blocked ticket has no red Blocked-by marker", name)
		}
		if html := draw(waiting); !strings.Contains(html, "Depends on BE-2") || !strings.Contains(html, "text-warn") {
			t.Errorf("%s: waiting ticket has no amber Depends-on marker", name)
		}
	}

	if html := render("backlog-row", map[string]any{"Ticket": blocked, "Rank": 0, "Draggable": true}); !strings.Contains(html, "drag-handle") {
		t.Error("sprint row (rank 0) lost its drag handle")
	} else if strings.Contains(html, "rank-label") {
		t.Error("sprint row carries a backlog rank label; renumberRanks would count it")
	}
	if html := render("backlog-row", map[string]any{"Ticket": done, "Rank": 0, "Draggable": false}); strings.Contains(html, "drag-handle") {
		t.Error("a done row should not be draggable")
	} else if strings.Contains(html, "Blocked by") {
		t.Error("a closed ticket should not show a blocker marker")
	}
}
