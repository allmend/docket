package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestTicketDisplayID(t *testing.T) {
	tk := Ticket{TeamKey: "BE", Number: 42}
	if got := tk.DisplayID(); got != "BE-42" {
		t.Errorf("DisplayID() = %q, want %q", got, "BE-42")
	}

	// Without a team, the display ID falls back to a short UUID fragment.
	id := uuid.MustParse("12345678-0000-0000-0000-000000000000")
	tk = Ticket{ID: id}
	if got := tk.DisplayID(); got != "12345678" {
		t.Errorf("DisplayID() fallback = %q, want %q", got, "12345678")
	}

	// A team key with no assigned number is not enough.
	tk = Ticket{ID: id, TeamKey: "BE", Number: 0}
	if got := tk.DisplayID(); got != "12345678" {
		t.Errorf("DisplayID() with number 0 = %q, want UUID fallback", got)
	}
}

func TestTicketACItems(t *testing.T) {
	tk := Ticket{AcceptanceCriteria: "- [ ] write docs\n- [x] add tests\n- [X] uppercase works\n\nnot a checklist line"}
	items := tk.ACItems()
	if len(items) != 3 {
		t.Fatalf("ACItems() returned %d items, want 3", len(items))
	}
	if items[0].Checked || items[0].Text != "write docs" {
		t.Errorf("first item = %+v, want unchecked 'write docs'", items[0])
	}
	if !items[1].Checked || items[1].Text != "add tests" {
		t.Errorf("second item = %+v, want checked 'add tests'", items[1])
	}
	if !items[2].Checked {
		t.Errorf("uppercase [X] should count as checked")
	}

	if items := (Ticket{}).ACItems(); len(items) != 0 {
		t.Errorf("empty AC should produce no items, got %d", len(items))
	}
}
