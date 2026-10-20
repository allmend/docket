package service

import (
	"errors"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

func dailyTicket(title, priority string, assignees []model.User, tags []model.Tag) model.Ticket {
	return model.Ticket{
		Title:     title,
		Priority:  model.Priority(priority),
		Assignees: assignees,
		Tags:      tags,
	}
}

func TestFilterDailyScrumTicketsNoFilters(t *testing.T) {
	tickets := []model.Ticket{dailyTicket("a", "low", nil, nil), dailyTicket("b", "high", nil, nil)}
	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{})
	if len(got) != 2 {
		t.Errorf("no filters should pass everything, got %d", len(got))
	}
}

func TestFilterDailyScrumTicketsQuery(t *testing.T) {
	tickets := []model.Ticket{
		dailyTicket("Fix login bug", "high", nil, nil),
		dailyTicket("Write docs", "low", nil, nil),
	}
	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{Q: "LOGIN"})
	if len(got) != 1 || got[0].Title != "Fix login bug" {
		t.Errorf("query filter failed: %+v", got)
	}
}

func TestFilterDailyScrumTicketsPriority(t *testing.T) {
	tickets := []model.Ticket{
		dailyTicket("a", "high", nil, nil),
		dailyTicket("b", "low", nil, nil),
		dailyTicket("c", "high", nil, nil),
	}
	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{Priorities: []string{"high"}})
	if len(got) != 2 {
		t.Errorf("priority filter returned %d, want 2", len(got))
	}
}

func TestFilterDailyScrumTicketsAssignee(t *testing.T) {
	alice := model.User{ID: uuid.New(), Name: "Alice"}
	bob := model.User{ID: uuid.New(), Name: "Bob"}
	tickets := []model.Ticket{
		dailyTicket("alice's", "low", []model.User{alice}, nil),
		dailyTicket("bob's", "low", []model.User{bob}, nil),
		dailyTicket("nobody's", "low", nil, nil),
	}

	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{AssigneeIDs: []string{alice.ID.String()}})
	if len(got) != 1 || got[0].Title != "alice's" {
		t.Errorf("assignee filter failed: %+v", got)
	}

	got = filterDailyScrumTickets(tickets, model.DailyScrumFilters{FilterUnassigned: true})
	if len(got) != 1 || got[0].Title != "nobody's" {
		t.Errorf("unassigned filter failed: %+v", got)
	}

	// Unassigned OR a named assignee combine as a union.
	got = filterDailyScrumTickets(tickets, model.DailyScrumFilters{
		AssigneeIDs:      []string{bob.ID.String()},
		FilterUnassigned: true,
	})
	if len(got) != 2 {
		t.Errorf("unassigned+assignee union returned %d, want 2", len(got))
	}
}

func TestFilterDailyScrumTicketsTags(t *testing.T) {
	bug := model.Tag{ID: uuid.New(), Name: "bug"}
	tickets := []model.Ticket{
		dailyTicket("tagged", "low", nil, []model.Tag{bug}),
		dailyTicket("untagged", "low", nil, nil),
	}
	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{TagIDs: []string{bug.ID.String()}})
	if len(got) != 1 || got[0].Title != "tagged" {
		t.Errorf("tag filter failed: %+v", got)
	}
}

func TestFilterDailyScrumTicketsCombined(t *testing.T) {
	alice := model.User{ID: uuid.New(), Name: "Alice"}
	tickets := []model.Ticket{
		dailyTicket("Fix login bug", "high", []model.User{alice}, nil),
		dailyTicket("Fix logout bug", "low", []model.User{alice}, nil),
		dailyTicket("Fix login flow", "high", nil, nil),
	}
	got := filterDailyScrumTickets(tickets, model.DailyScrumFilters{
		Q:           "login",
		Priorities:  []string{"high"},
		AssigneeIDs: []string{alice.ID.String()},
	})
	if len(got) != 1 || got[0].Title != "Fix login bug" {
		t.Errorf("combined filters failed: %+v", got)
	}
}

func TestCheckDoneColumnName(t *testing.T) {
	withDone := []model.Column{{Name: "To Do"}, {Name: "Done"}}
	withoutDone := []model.Column{{Name: "Ideas"}}

	tests := []struct {
		name     string
		existing []model.Column
		col      string
		renaming bool
		wantErr  bool
	}{
		{"ordinary rename", withDone, "In Review", true, false},
		{"rename to Done", withoutDone, "Done", true, true},
		{"rename to done, any case", withoutDone, "dOnE", true, true},
		{"add second Done", withDone, "done", false, true},
		{"add first Done to a blank board", withoutDone, "Done", false, false},
		{"add ordinary column", withDone, "QA", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkDoneColumnName(tc.existing, tc.col, tc.renaming)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput (maps to 400)", err)
			}
		})
	}
}

func TestDailyParticipants(t *testing.T) {
	alice := model.User{ID: uuid.New(), Name: "Alice"}
	bob := model.User{ID: uuid.New(), Name: "Bob"}
	carol := model.User{ID: uuid.New(), Name: "Carol"} // has work, not a team member

	groups := []model.DailyScrumGroup{{User: bob}, {User: carol}}
	got := dailyParticipants([]model.User{alice, bob}, groups)

	want := []uuid.UUID{alice.ID, bob.ID, carol.ID}
	if len(got) != len(want) {
		t.Fatalf("got %d participants, want %d (%v)", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("participant %d = %s, want members first, then non-member assignees", i, got[i].Name)
		}
	}
	if len(dailyParticipants(nil, nil)) != 0 {
		t.Error("empty team with no work should have no participants")
	}
}

func TestDescribeColumnTickets(t *testing.T) {
	got := describeColumnTickets(store.ColumnTicketCounts{ActiveSprint: 2, PlanningSprint: 1, Hidden: 9})
	if want := "2 in the active sprint, 1 in sprint planning"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := describeColumnTickets(store.ColumnTicketCounts{OnBoard: 3}); got != "3 on the board" {
		t.Errorf("kanban: got %q", got)
	}
}
