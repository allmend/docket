package model

import (
	"strings"
	"unicode/utf8"
)

// Close reasons stored in tickets.close_reason. A known reason is stored by
// its key; "other" stores the user's own note instead, so the column holds
// either a key or free text.
const (
	CloseReasonDone        = "done"
	CloseReasonDuplicate   = "duplicate"
	CloseReasonNotRelevant = "not_relevant"
	CloseReasonWontDo      = "wont_do"
	CloseReasonOther       = "other"
)

// MaxCloseNoteLen caps a free-text ("other") close reason, in characters.
const MaxCloseNoteLen = 200

// CloseReasonOption is one entry in the close picker.
type CloseReasonOption struct {
	Value string
	Label string
}

var closeReasonLabels = map[string]string{
	CloseReasonDone:        "Done",
	CloseReasonDuplicate:   "Duplicate",
	CloseReasonNotRelevant: "Not relevant",
	CloseReasonWontDo:      "Won't do",
}

// CloseReasonOptions lists the picker entries in display order, ending with
// "other", which asks for a note.
func CloseReasonOptions() []CloseReasonOption {
	opts := make([]CloseReasonOption, 0, 5)
	for _, v := range []string{CloseReasonDone, CloseReasonDuplicate, CloseReasonNotRelevant, CloseReasonWontDo} {
		opts = append(opts, CloseReasonOption{Value: v, Label: closeReasonLabels[v]})
	}
	return append(opts, CloseReasonOption{Value: CloseReasonOther, Label: "Other…"})
}

// ParseCloseReason turns picker input into the value to store. A known reason
// is stored as its key; "other" stores the trimmed note, which must be
// non-empty and at most MaxCloseNoteLen characters. Anything else is invalid.
func ParseCloseReason(reason, note string) (string, bool) {
	if _, ok := closeReasonLabels[reason]; ok {
		return reason, true
	}
	if reason != CloseReasonOther {
		return "", false
	}
	note = strings.TrimSpace(note)
	if note == "" || utf8.RuneCountInString(note) > MaxCloseNoteLen {
		return "", false
	}
	return note, true
}

// CloseReasonLabel is the display text for a stored close reason: the label
// for a known key, the note itself for a free-text reason. "Closed" was the
// placeholder stored before reasons existed and says nothing, so it shows as
// no reason at all.
func CloseReasonLabel(stored string) string {
	if label, ok := closeReasonLabels[stored]; ok {
		return label
	}
	if stored == "Closed" {
		return ""
	}
	return stored
}
