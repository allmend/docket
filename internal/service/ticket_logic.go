package service

import (
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
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

// rebalanceDropSlot returns the position that keeps a dropped ticket directly
// in front of its former next-neighbour once the list has been renumbered to
// multiples of 1000. tickets is the list in position order *before* the
// renumber. The neighbour is identified by nextID when the client sent it, and
// otherwise by its pre-renumber position nextPos. The dragged ticket itself is
// skipped: its stale position may collide in the degenerate gaps that trigger
// a rebalance. When the neighbour can't be found (stale client data) the
// ticket goes to the end of the list.
func rebalanceDropSlot(tickets []model.Ticket, ticketID, nextID uuid.UUID, nextPos float64) float64 {
	for i, t := range tickets {
		if t.ID == ticketID {
			continue
		}
		if (nextID != uuid.Nil && t.ID == nextID) || (nextID == uuid.Nil && t.Position == nextPos) {
			return float64((i+1)*1000) - 500
		}
	}
	return float64((len(tickets) + 1) * 1000)
}

// ticketIDs lists the tickets' IDs in order — the order a rebalance assigns
// them 1000, 2000, …, which is the order rebalanceDropSlot counts in.
func ticketIDs(tickets []model.Ticket) []uuid.UUID {
	ids := make([]uuid.UUID, len(tickets))
	for i, t := range tickets {
		ids[i] = t.ID
	}
	return ids
}

// columnTransition is what a column move does to a ticket's open/closed
// state. Drag and the status dropdown share it, so they can't disagree:
// entering Done closes an open ticket, leaving Done reopens a closed one, and
// a ticket closed outside Done (with a reason such as "won't do") stays closed
// while it moves between other columns.
type columnTransition int

const (
	transitionNone columnTransition = iota
	transitionClose
	transitionReopen
)

func decideColumnTransition(fromName, toName string, closed bool) columnTransition {
	switch {
	case isDone(toName) && !closed:
		return transitionClose
	case !isDone(toName) && isDone(fromName) && closed:
		return transitionReopen
	}
	return transitionNone
}

// toggleNthCheckbox flips the Nth acceptance-criteria item, counting from 0 the
// same way model.Ticket.ACItems does. Out-of-range indices leave src unchanged.
func toggleNthCheckbox(src string, n int) string {
	lines := strings.Split(src, "\n")
	itemIdx := 0
	for i, line := range lines {
		item, ok := model.ParseACLine(line)
		if !ok {
			continue
		}
		if itemIdx == n {
			// The marker sits at "- [" after any indentation; flip the one
			// character inside the brackets.
			at := len(line) - len(strings.TrimLeft(line, " \t")) + 3
			mark := "x"
			if item.Checked {
				mark = " "
			}
			lines[i] = line[:at] + mark + line[at+1:]
			return strings.Join(lines, "\n")
		}
		itemIdx++
	}
	return src
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
		if _, isItem := model.ParseACLine(line); isItem {
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

// columnMoveChange returns the history entry for a move from the column named
// fromName into newCol, or false when there is nothing to record: a nil newCol
// (backlog reorder) or a drop back into the same column (reorder in place).
func columnMoveChange(fromColumnID uuid.UUID, fromName string, newCol *model.Column) (fieldChange, bool) {
	if newCol == nil || newCol.ID == fromColumnID {
		return fieldChange{}, false
	}
	return fieldChange{"column", fromName, newCol.Name}, true
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

// targetColumnForState picks the column a ticket should move to when it is
// closed (the Done column) or reopened (the first column that isn't Done, by
// position). Nil means stay: it is already in a fitting column, or the board
// has none.
func targetColumnForState(cols []model.Column, current uuid.UUID, closed bool) *model.Column {
	isDone := func(c model.Column) bool { return strings.EqualFold(c.Name, "done") }
	for _, c := range cols {
		if c.ID == current && isDone(c) == closed {
			return nil
		}
	}
	var pick *model.Column
	for i := range cols {
		c := cols[i]
		if isDone(c) != closed {
			continue
		}
		if pick == nil || c.Position < pick.Position {
			pick = &cols[i]
		}
	}
	return pick
}
