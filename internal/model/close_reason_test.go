package model

import (
	"strings"
	"testing"
)

func TestParseCloseReason(t *testing.T) {
	cases := []struct {
		reason, note string
		want         string
		ok           bool
	}{
		{"done", "", "done", true},
		{"duplicate", "ignored", "duplicate", true},
		{"not_relevant", "", "not_relevant", true},
		{"wont_do", "", "wont_do", true},
		{"other", "  superseded by the new design  ", "superseded by the new design", true},
		{"other", "   ", "", false},
		{"other", strings.Repeat("é", MaxCloseNoteLen), strings.Repeat("é", MaxCloseNoteLen), true},
		{"other", strings.Repeat("x", MaxCloseNoteLen+1), "", false},
		{"", "", "", false},
		{"Closed", "", "", false},
		{"fixed", "", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseCloseReason(tc.reason, tc.note)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseCloseReason(%q, %q) = %q, %v; want %q, %v", tc.reason, tc.note, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCloseReasonLabel(t *testing.T) {
	cases := map[string]string{
		"done":                          "Done",
		"duplicate":                     "Duplicate",
		"not_relevant":                  "Not relevant",
		"wont_do":                       "Won't do",
		"Closed":                        "",
		"":                              "",
		"merged into the parent ticket": "merged into the parent ticket",
	}
	for stored, want := range cases {
		if got := CloseReasonLabel(stored); got != want {
			t.Errorf("CloseReasonLabel(%q) = %q, want %q", stored, got, want)
		}
	}
}

// Every option except "other" must round-trip through ParseCloseReason, so the
// picker can never offer a reason the server rejects.
func TestCloseReasonOptionsAreAccepted(t *testing.T) {
	opts := CloseReasonOptions()
	if opts[len(opts)-1].Value != CloseReasonOther {
		t.Fatal("other must be the last option")
	}
	for _, o := range opts[:len(opts)-1] {
		if _, ok := ParseCloseReason(o.Value, ""); !ok {
			t.Errorf("option %q is rejected by ParseCloseReason", o.Value)
		}
	}
}
