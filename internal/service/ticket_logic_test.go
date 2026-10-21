package service

import (
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func TestRebalanceDropSlot(t *testing.T) {
	id := func(n byte) uuid.UUID { return uuid.UUID{n} }
	col := []model.Ticket{ // pre-renumber column, position order
		{ID: id(1), Position: 1000},
		{ID: id(2), Position: 1000.0004},
		{ID: id(3), Position: 1000.0008},
	}
	dragged := id(9)

	// Dropping in front of the middle ticket: it renumbers to 2000, slot is 1500.
	if got := rebalanceDropSlot(col, dragged, uuid.Nil, 1000.0004); got != 1500 {
		t.Errorf("slot before middle = %v, want 1500", got)
	}
	// Dropping in front of the first ticket: it renumbers to 1000, slot is 500.
	if got := rebalanceDropSlot(col, dragged, uuid.Nil, 1000); got != 500 {
		t.Errorf("slot before first = %v, want 500", got)
	}
	// The dragged ticket's own stale position must not match as the neighbour.
	inCol := []model.Ticket{col[0], {ID: dragged, Position: 1000.0004}, col[1], col[2]}
	if got := rebalanceDropSlot(inCol, dragged, uuid.Nil, 1000.0004); got != 2500 {
		t.Errorf("slot skipping self = %v, want 2500", got)
	}
	// Unknown neighbour (stale client data) falls back to the end of the column.
	if got := rebalanceDropSlot(col, dragged, uuid.Nil, 42); got != 4000 {
		t.Errorf("stale-neighbour fallback = %v, want 4000", got)
	}
	if got := rebalanceDropSlot(nil, dragged, uuid.Nil, 42); got != 1000 {
		t.Errorf("empty-column fallback = %v, want 1000", got)
	}
	// Positions tie across columns in a backlog, so the neighbour's ID wins
	// over a position match when the client sends it.
	tied := []model.Ticket{{ID: id(1), Position: 1000}, {ID: id(2), Position: 1000}, {ID: id(3), Position: 3000}}
	if got := rebalanceDropSlot(tied, dragged, id(2), 1000); got != 1500 {
		t.Errorf("slot by neighbour ID = %v, want 1500", got)
	}
	if got := rebalanceDropSlot(tied, dragged, id(7), 1000); got != 4000 {
		t.Errorf("unknown neighbour ID fallback = %v, want 4000", got)
	}
}

func TestDecideColumnTransition(t *testing.T) {
	cases := []struct {
		from, to string
		closed   bool
		want     columnTransition
	}{
		{"In Progress", "Done", false, transitionClose},
		{"In Progress", "done", false, transitionClose},
		{"Done", "In Progress", true, transitionReopen},
		{"Done", "Done", true, transitionNone},         // reorder inside Done
		{"To Do", "In Progress", true, transitionNone}, // closed with a reason outside Done stays closed
		{"To Do", "Done", true, transitionNone},        // already closed: keeps its reason
		{"Done", "To Do", false, transitionNone},       // open ticket sitting in Done: nothing to reopen
		{"To Do", "In Progress", false, transitionNone},
	}
	for _, c := range cases {
		if got := decideColumnTransition(c.from, c.to, c.closed); got != c.want {
			t.Errorf("%q→%q closed=%v: got %v, want %v", c.from, c.to, c.closed, got, c.want)
		}
	}
}

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

// The index comes from the rendered list (model.Ticket.ACItems), so toggling
// must count items exactly as that does: brackets inside an item's text, or on
// a line that isn't an item, are not checkboxes.
func TestToggleNthCheckboxCountsOnlyItems(t *testing.T) {
	src := "Notes about [x] and [ ]\n- [ ] support a[x] syntax\n  - [ ] indented second"

	got := toggleNthCheckbox(src, 1)
	want := "Notes about [x] and [ ]\n- [ ] support a[x] syntax\n  - [x] indented second"
	if got != want {
		t.Errorf("toggle second item:\n got %q\nwant %q", got, want)
	}
	if n := len((model.Ticket{AcceptanceCriteria: src}).ACItems()); n != 2 {
		t.Fatalf("ACItems() sees %d items, want 2 — test premise broken", n)
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

func TestColumnMoveChange(t *testing.T) {
	todo := uuid.New()
	done := &model.Column{ID: uuid.New(), Name: "Done"}
	same := &model.Column{ID: todo, Name: "To Do"}

	if _, ok := columnMoveChange(todo, "To Do", nil); ok {
		t.Error("backlog reorder (nil column) recorded a change")
	}
	if _, ok := columnMoveChange(todo, "To Do", same); ok {
		t.Error("reorder within the same column recorded a change")
	}
	c, ok := columnMoveChange(todo, "To Do", done)
	if !ok {
		t.Fatal("cross-column move recorded nothing")
	}
	if c != (fieldChange{"column", "To Do", "Done"}) {
		t.Errorf("change = %+v, want column To Do → Done", c)
	}
}

func TestSamePoints(t *testing.T) {
	one, two := 1.0, 2.0
	for _, c := range []struct {
		a, b *float64
		want bool
	}{{nil, nil, true}, {&one, nil, false}, {nil, &one, false}, {&one, &one, true}, {&one, &two, false}} {
		if got := samePoints(c.a, c.b); got != c.want {
			t.Errorf("samePoints(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseTicketRef(t *testing.T) {
	cases := []struct {
		in   string
		key  string
		num  int
		isOK bool
	}{
		{"BE-42", "BE", 42, true},
		{"be-7", "BE", 7, true},
		{"DKT2-1", "DKT2", 1, true},
		{"BE-0", "", 0, false},
		{"BE 42", "", 0, false},
		{"42", "", 0, false},
		{"-42", "", 0, false},
		{"BE-42x", "", 0, false},
	}
	for _, c := range cases {
		key, num, ok := parseTicketRef(c.in)
		if key != c.key || num != c.num || ok != c.isOK {
			t.Errorf("parseTicketRef(%q) = %q, %d, %v; want %q, %d, %v", c.in, key, num, ok, c.key, c.num, c.isOK)
		}
	}
}
