package service

import (
	"regexp"
	"strings"

	"github.com/allmend/docket/internal/model"
)

// nextPosition computes the fractional-index position for a card dropped
// between neighbours at prevPos and nextPos, where 0 means the list boundary.
// needsRebalance reports that the gap is too small to split further and the
// column must be renumbered before the move can be applied.
func nextPosition(prevPos, nextPos float64) (pos float64, needsRebalance bool) {
	switch {
	case prevPos == 0 && nextPos == 0:
		pos = 1000
	case prevPos == 0:
		pos = nextPos / 2
	case nextPos == 0:
		pos = prevPos + 1000
	default:
		pos = (prevPos + nextPos) / 2
	}
	return pos, nextPos != 0 && (pos-prevPos) < 0.001
}

// checkboxRe matches GFM task-list markers: [ ] or [x] (case-insensitive).
var checkboxRe = regexp.MustCompile(`\[([ xX])\]`)

// toggleNthCheckbox flips the Nth task-list marker in src, counting from 0.
// Out-of-range indices leave src unchanged.
func toggleNthCheckbox(src string, n int) string {
	count := 0
	return checkboxRe.ReplaceAllStringFunc(src, func(match string) string {
		if count == n {
			count++
			if match == "[ ]" {
				return "[x]"
			}
			return "[ ]"
		}
		count++
		return match
	})
}

// acAppendItem appends a new unchecked checklist line to the acceptance criteria.
func acAppendItem(ac, text string) string {
	line := "- [ ] " + strings.TrimSpace(text)
	if ac == "" {
		return line
	}
	return strings.TrimRight(ac, "\n") + "\n" + line
}

// acDeleteItem removes the Nth checklist item, counting from 0. Non-checklist
// lines are kept (except blank ones), and out-of-range indices delete nothing.
func acDeleteItem(ac string, index int) string {
	var kept []string
	itemIdx := 0
	for _, line := range strings.Split(ac, "\n") {
		trimmed := strings.TrimSpace(line)
		isItem := strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ")
		if isItem {
			if itemIdx != index {
				kept = append(kept, line)
			}
			itemIdx++
		} else if trimmed != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// fieldChange is one ticket field diff destined for the history log.
type fieldChange struct {
	Field, Old, New string
}

// ticketFieldChanges diffs the mutable fields between the ticket before and
// after an update, returning one entry per changed field. Description changes
// are recorded as markers only — no full diff is stored.
func ticketFieldChanges(old, updated *model.Ticket) []fieldChange {
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}

	var changes []fieldChange
	if old.Title != updated.Title {
		changes = append(changes, fieldChange{"title", old.Title, updated.Title})
	}
	if old.Priority != updated.Priority {
		changes = append(changes, fieldChange{"priority", string(old.Priority), string(updated.Priority)})
	}
	if deref(old.AssigneeName) != deref(updated.AssigneeName) {
		changes = append(changes, fieldChange{"assignee", deref(old.AssigneeName), deref(updated.AssigneeName)})
	}
	if old.Body != updated.Body {
		changes = append(changes, fieldChange{"description", "(previous)", "(updated)"})
	}
	return changes
}
