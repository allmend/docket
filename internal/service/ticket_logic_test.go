package service

import (
	"testing"

	"github.com/allmend/docket/internal/model"
)

func TestNextPosition(t *testing.T) {
	tests := []struct {
		name          string
		prev, next    float64
		wantPos       float64
		wantRebalance bool
	}{
		{"empty list", 0, 0, 1000, false},
		{"insert at top", 0, 2000, 1000, false},
		{"insert at bottom", 3000, 0, 4000, false},
		{"insert between", 1000, 2000, 1500, false},
		{"tiny gap forces rebalance", 1000, 1000.001, 1000.0005, true},
		{"tiny gap at top forces rebalance", 0, 0.001, 0.0005, true},
		{"bottom insert never rebalances", 0.0001, 0, 1000.0001, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, rebalance := nextPosition(tt.prev, tt.next)
			if pos != tt.wantPos {
				t.Errorf("nextPosition(%v, %v) pos = %v, want %v", tt.prev, tt.next, pos, tt.wantPos)
			}
			if rebalance != tt.wantRebalance {
				t.Errorf("nextPosition(%v, %v) rebalance = %v, want %v", tt.prev, tt.next, rebalance, tt.wantRebalance)
			}
		})
	}
}

func TestToggleNthCheckbox(t *testing.T) {
	src := "- [ ] one\n- [x] two\n- [X] three"

	if got := toggleNthCheckbox(src, 0); got != "- [x] one\n- [x] two\n- [X] three" {
		t.Errorf("toggle first: %q", got)
	}
	if got := toggleNthCheckbox(src, 1); got != "- [ ] one\n- [ ] two\n- [X] three" {
		t.Errorf("toggle second: %q", got)
	}
	// Uppercase X unchecks too.
	if got := toggleNthCheckbox(src, 2); got != "- [ ] one\n- [x] two\n- [ ] three" {
		t.Errorf("toggle third: %q", got)
	}
	// Out of range leaves the source untouched.
	if got := toggleNthCheckbox(src, 9); got != src {
		t.Errorf("out-of-range index modified source: %q", got)
	}
}

func TestACAppendItem(t *testing.T) {
	if got := acAppendItem("", "first"); got != "- [ ] first" {
		t.Errorf("append to empty = %q", got)
	}
	if got := acAppendItem("- [x] done", "  new  "); got != "- [x] done\n- [ ] new" {
		t.Errorf("append trims and joins = %q", got)
	}
	// Trailing newlines must not produce blank lines between items.
	if got := acAppendItem("- [ ] a\n\n", "b"); got != "- [ ] a\n- [ ] b" {
		t.Errorf("append after trailing newline = %q", got)
	}
}

func TestACDeleteItem(t *testing.T) {
	src := "- [ ] a\n- [x] b\n- [X] c"

	if got := acDeleteItem(src, 1); got != "- [ ] a\n- [X] c" {
		t.Errorf("delete middle = %q", got)
	}
	if got := acDeleteItem(src, 0); got != "- [x] b\n- [X] c" {
		t.Errorf("delete first = %q", got)
	}
	// Out of range deletes nothing.
	if got := acDeleteItem(src, 5); got != src {
		t.Errorf("out-of-range delete changed source = %q", got)
	}
	// Non-checklist lines survive, blank lines are dropped.
	if got := acDeleteItem("intro\n\n- [ ] a", 0); got != "intro" {
		t.Errorf("delete keeps prose = %q", got)
	}
}

func TestTicketFieldChanges(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	old := &model.Ticket{Title: "old", Body: "body", Priority: model.Priority("medium"), AssigneeName: strPtr("alice")}
	upd := &model.Ticket{Title: "new", Body: "body2", Priority: model.Priority("high"), AssigneeName: strPtr("bob")}

	changes := ticketFieldChanges(old, upd)
	if len(changes) != 4 {
		t.Fatalf("got %d changes, want 4: %+v", len(changes), changes)
	}
	want := map[string][2]string{
		"title":       {"old", "new"},
		"priority":    {"medium", "high"},
		"assignee":    {"alice", "bob"},
		"description": {"(previous)", "(updated)"},
	}
	for _, c := range changes {
		w, ok := want[c.Field]
		if !ok {
			t.Errorf("unexpected change field %q", c.Field)
			continue
		}
		if c.Old != w[0] || c.New != w[1] {
			t.Errorf("%s: got %q→%q, want %q→%q", c.Field, c.Old, c.New, w[0], w[1])
		}
	}

	// Identical tickets produce no history entries.
	if changes := ticketFieldChanges(old, old); len(changes) != 0 {
		t.Errorf("no-op update produced %d changes", len(changes))
	}

	// Nil assignee on both sides is not a change.
	a := &model.Ticket{}
	b := &model.Ticket{}
	if changes := ticketFieldChanges(a, b); len(changes) != 0 {
		t.Errorf("nil assignees diffed as change: %+v", changes)
	}
}

func TestIsDone(t *testing.T) {
	for _, name := range []string{"Done", "done", "DONE", "dOnE"} {
		if !isDone(name) {
			t.Errorf("isDone(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"Done!", "In Progress", "", "Donezo"} {
		if isDone(name) {
			t.Errorf("isDone(%q) = true, want false", name)
		}
	}
}
