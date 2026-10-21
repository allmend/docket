package model

import "testing"

func TestFlowSummary(t *testing.T) {
	f := FlowSummary{Columns: []FlowColumn{
		{Name: "Backlog", Open: 5, OpenPoints: 8},
		{Name: "In Progress", Open: 3, OpenPoints: 5},
		{Name: "In Review", Open: 2},
		{Name: "Done", Open: 0},
	}}
	if got := f.Open(); got != 10 {
		t.Errorf("Open = %d, want 10", got)
	}
	// The first column is where new work lands; it isn't started yet.
	if got := f.InProgress(); got != 5 {
		t.Errorf("InProgress = %d, want 5", got)
	}
	if got := f.OpenPoints(); got != 13 {
		t.Errorf("OpenPoints = %v, want 13", got)
	}
	if got := f.Pct(f.Columns[0]); got != 50 {
		t.Errorf("Pct(Backlog) = %d, want 50", got)
	}
	if got := (FlowSummary{}).Pct(FlowColumn{}); got != 0 {
		t.Errorf("empty board Pct = %d, want 0", got)
	}
}
