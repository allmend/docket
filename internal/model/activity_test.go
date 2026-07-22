package model

import (
	"testing"
	"time"
)

func TestMergeActivity(t *testing.T) {
	base := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	at := func(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

	history := []HistoryEntry{
		{Field: "title", CreatedAt: at(0)},
		{Field: "comment", CreatedAt: at(10)}, // shadow of the comment below
		{Field: "column", NewValue: "In Progress", CreatedAt: at(20)},
	}
	comments := []Comment{
		{Body: "first", CreatedAt: at(10)},
		{Body: "second", CreatedAt: at(30)},
	}

	got := MergeActivity(history, comments)

	// The "comment" history row is dropped; everything else interleaves by time.
	if len(got) != 4 {
		t.Fatalf("got %d items, want 4", len(got))
	}

	want := []struct {
		isComment bool
		label     string
		at        time.Time
	}{
		{false, "title", at(0)},
		{true, "first", at(10)},
		{false, "column", at(20)},
		{true, "second", at(30)},
	}
	for i, w := range want {
		item := got[i]
		if !item.CreatedAt.Equal(w.at) {
			t.Errorf("item %d at %v, want %v", i, item.CreatedAt, w.at)
		}
		switch {
		case w.isComment:
			if item.Comment == nil {
				t.Fatalf("item %d should be a comment", i)
			}
			if item.History != nil {
				t.Errorf("item %d has both a comment and history", i)
			}
			if item.Comment.Body != w.label {
				t.Errorf("item %d body = %q, want %q", i, item.Comment.Body, w.label)
			}
		default:
			if item.History == nil {
				t.Fatalf("item %d should be history", i)
			}
			if item.Comment != nil {
				t.Errorf("item %d has both a comment and history", i)
			}
			if item.History.Field != w.label {
				t.Errorf("item %d field = %q, want %q", i, item.History.Field, w.label)
			}
		}
	}
}

// TestMergeActivityDropsCommentHistory pins the de-duplication: a comment must
// appear once, not as both a history row and the comment itself.
func TestMergeActivityDropsCommentHistory(t *testing.T) {
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	got := MergeActivity(
		[]HistoryEntry{{Field: "comment", ActorName: "Admin", CreatedAt: now}},
		[]Comment{{Body: "hello", AuthorName: "Admin", CreatedAt: now}},
	)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1 (the comment only)", len(got))
	}
	if got[0].Comment == nil {
		t.Fatal("the surviving item should be the comment, not the history row")
	}
}

// TestMergeActivityPointsAtDistinctEntries guards against the classic loop-variable
// aliasing bug: every item must point at its own record, not a shared one.
func TestMergeActivityPointsAtDistinctEntries(t *testing.T) {
	base := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	got := MergeActivity([]HistoryEntry{
		{Field: "title", CreatedAt: base},
		{Field: "priority", CreatedAt: base.Add(time.Minute)},
	}, nil)

	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	if got[0].History.Field == got[1].History.Field {
		t.Fatalf("both items alias the same entry (%q)", got[0].History.Field)
	}
}

// TestMergeActivityEmpty covers a freshly created ticket.
func TestMergeActivityEmpty(t *testing.T) {
	if got := MergeActivity(nil, nil); len(got) != 0 {
		t.Fatalf("got %d items, want 0", len(got))
	}
}
