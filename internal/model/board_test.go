package model

import (
	"strings"
	"testing"
	"time"
)

func TestSprintCompletionPct(t *testing.T) {
	tests := []struct {
		name      string
		committed int
		completed int
		want      int
	}{
		{"no tickets committed", 0, 0, 0},
		{"half done", 10, 5, 50},
		{"all done", 4, 4, 100},
		{"none done", 8, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Sprint{CommittedTickets: tt.committed, CompletedTickets: tt.completed}
			if got := s.CompletionPct(); got != tt.want {
				t.Errorf("CompletionPct() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSprintPointsPct(t *testing.T) {
	tests := []struct {
		name      string
		committed float64
		completed float64
		want      int
	}{
		{"no points committed", 0, 0, 0},
		{"half done", 20, 10, 50},
		{"rounds down", 3, 1, 33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Sprint{CommittedPoints: tt.committed, CompletedPoints: tt.completed}
			if got := s.PointsPct(); got != tt.want {
				t.Errorf("PointsPct() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSprintRemainingPoints(t *testing.T) {
	s := Sprint{CommittedPoints: 10, CompletedPoints: 3}
	if got := s.RemainingPoints(); got != 7 {
		t.Errorf("RemainingPoints() = %v, want 7", got)
	}
	// Completing more than committed (unplanned work) must not go negative.
	s = Sprint{CommittedPoints: 5, CompletedPoints: 8}
	if got := s.RemainingPoints(); got != 0 {
		t.Errorf("RemainingPoints() = %v, want 0", got)
	}
}

func TestSprintTotalDays(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	s := Sprint{StartDate: &start, EndDate: &end}
	if got := s.TotalDays(); got != 14 {
		t.Errorf("TotalDays() = %d, want 14", got)
	}
	if got := (Sprint{}).TotalDays(); got != 0 {
		t.Errorf("TotalDays() without dates = %d, want 0", got)
	}
	if got := (Sprint{StartDate: &start}).TotalDays(); got != 0 {
		t.Errorf("TotalDays() without end date = %d, want 0", got)
	}
}

func TestSprintDayNumber(t *testing.T) {
	if got := (Sprint{}).DayNumber(); got != 0 {
		t.Errorf("DayNumber() without start date = %d, want 0", got)
	}
	// Day 1 is the start day itself.
	today := time.Now()
	if got := (Sprint{StartDate: &today}).DayNumber(); got != 1 {
		t.Errorf("DayNumber() on start day = %d, want 1", got)
	}
	// Three days in → day 4 (1-based, inclusive of start day).
	threeDaysAgo := today.Add(-3 * 24 * time.Hour)
	if got := (Sprint{StartDate: &threeDaysAgo}).DayNumber(); got != 4 {
		t.Errorf("DayNumber() three days in = %d, want 4", got)
	}
	// A future start date must clamp to day 1, never 0 or negative.
	future := today.Add(48 * time.Hour)
	if got := (Sprint{StartDate: &future}).DayNumber(); got != 1 {
		t.Errorf("DayNumber() with future start = %d, want 1", got)
	}
}

func TestSprintStatus(t *testing.T) {
	if !SprintStatusActive.IsActive() || SprintStatusActive.IsPlanning() || SprintStatusActive.IsCompleted() {
		t.Error("active status misreported")
	}
	if !SprintStatusPlanning.IsPlanning() || SprintStatusPlanning.IsActive() {
		t.Error("planning status misreported")
	}
	if !SprintStatusCompleted.IsCompleted() || SprintStatusCompleted.IsActive() {
		t.Error("completed status misreported")
	}
}

func TestBoardModeIsScrum(t *testing.T) {
	if !BoardModeScrum.IsScrum() {
		t.Error("scrum mode should report IsScrum")
	}
	if BoardModeKanban.IsScrum() {
		t.Error("kanban mode should not report IsScrum")
	}
}

func TestColumnViewTotalPoints(t *testing.T) {
	pts := func(v float64) *float64 { return &v }

	cv := ColumnView{Tickets: []Ticket{
		{StoryPoints: pts(3)},
		{StoryPoints: nil},
		{StoryPoints: pts(5)},
	}}
	if got := cv.TotalPoints(); got != "8 pt" {
		t.Errorf("TotalPoints() = %q, want %q", got, "8 pt")
	}

	// Zero total renders as empty so the badge can be hidden.
	empty := ColumnView{Tickets: []Ticket{{StoryPoints: nil}}}
	if got := empty.TotalPoints(); got != "" {
		t.Errorf("TotalPoints() with no points = %q, want empty", got)
	}
}

func TestSprintCapacityTeamDays(t *testing.T) {
	c := SprintCapacity{Members: []SprintCapacityMember{
		{FocusPct: 100},
		{FocusPct: 50},
	}}
	if got := c.TeamDays(10); got != 15 {
		t.Errorf("TeamDays(10) = %v, want 15", got)
	}
	if got := c.TeamDays(0); got != -1 {
		t.Errorf("TeamDays(0) = %v, want -1", got)
	}
	if got := (SprintCapacity{}).TeamDays(10); got != -1 {
		t.Errorf("TeamDays with no members = %v, want -1", got)
	}
}

func TestSprintScheduleLabel(t *testing.T) {
	// DayNumber is derived from time.Since(StartDate), so anchor the dates
	// relative to now: a sprint that started `startedAgo` days ago and runs for
	// `length` days total.
	sprint := func(startedAgo, length int) Sprint {
		start := time.Now().AddDate(0, 0, -startedAgo)
		end := start.AddDate(0, 0, length-1)
		return Sprint{StartDate: &start, EndDate: &end}
	}

	tests := []struct {
		name string
		s    Sprint
		want string
	}{
		{"mid sprint", sprint(2, 10), "7 days left"},
		{"one day to go", sprint(8, 10), "1 day left"},
		{"final day", sprint(9, 10), "last day"},
		{"one day over", sprint(10, 10), "1 day overdue"},
		{"well overdue", sprint(26, 10), "17 days overdue"},
		{"no dates", Sprint{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.ScheduleLabel(); got != tc.want {
				t.Errorf("ScheduleLabel() = %q, want %q (day %d of %d)",
					got, tc.want, tc.s.DayNumber(), tc.s.TotalDays())
			}
		})
	}
}

// TestSprintScheduleLabelNeverNegative pins the reported bug: an overrunning
// sprint rendered "-17 days left" because the template did TotalDays−DayNumber
// inline and always said "left".
func TestSprintScheduleLabelNeverNegative(t *testing.T) {
	for over := 1; over <= 30; over++ {
		start := time.Now().AddDate(0, 0, -(10 + over - 1))
		end := start.AddDate(0, 0, 9)
		got := Sprint{StartDate: &start, EndDate: &end}.ScheduleLabel()
		if strings.Contains(got, "-") {
			t.Fatalf("%d days over: got %q, which contains a negative", over, got)
		}
		if !strings.Contains(got, "overdue") {
			t.Fatalf("%d days over: got %q, want an overdue phrasing", over, got)
		}
	}
}
