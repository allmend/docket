package model

import (
	"html/template"
	"sort"
	"time"

	"github.com/allmend/docket/internal/markdown"
	"github.com/google/uuid"
)

type Comment struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	TicketID   uuid.UUID `json:"ticket_id"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	Edited     bool      `json:"edited"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// Editable is populated at render time — true when the viewer authored the
	// comment or is an org admin. Not persisted. Gates the edit/delete buttons.
	Editable bool `json:"-"`
}

func (c *Comment) BodyHTML() template.HTML {
	return markdown.Render(c.Body)
}

type HistoryEntry struct {
	ID        uuid.UUID `json:"id"`
	TicketID  uuid.UUID `json:"ticket_id"`
	ActorID   uuid.UUID `json:"actor_id"`
	ActorName string    `json:"actor_name"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	CreatedAt time.Time `json:"created_at"`
}

// ActivityItem is one entry in a ticket's activity stream: either a history
// record or a comment, never both. Exactly one of History/Comment is non-nil,
// which is how the template picks its renderer.
type ActivityItem struct {
	CreatedAt time.Time
	History   *HistoryEntry
	Comment   *Comment
}

// MergeActivity interleaves history entries and comments into a single stream
// ordered oldest-first, so the ticket view reads chronologically instead of
// showing every field change above every comment.
//
// Oldest-first matters beyond readability: the comment composer posts with
// hx-swap="beforeend", so a new comment must belong at the end of the list.
//
// History rows with field "comment" are dropped — the comment itself carries the
// same actor and timestamp plus the body, so keeping both duplicates the entry.
// The stored row is untouched; feeds that don't render comment bodies (the
// dashboard and stand-up activity lists) still use it.
func MergeActivity(history []HistoryEntry, comments []Comment) []ActivityItem {
	items := make([]ActivityItem, 0, len(history)+len(comments))
	for i := range history {
		if history[i].Field == "comment" {
			continue
		}
		items = append(items, ActivityItem{CreatedAt: history[i].CreatedAt, History: &history[i]})
	}
	for i := range comments {
		items = append(items, ActivityItem{CreatedAt: comments[i].CreatedAt, Comment: &comments[i]})
	}
	// Stable so that entries sharing a timestamp keep history-before-comment
	// order rather than shuffling between renders.
	sort.SliceStable(items, func(a, b int) bool {
		return items[a].CreatedAt.Before(items[b].CreatedAt)
	})
	return items
}

// InboxEntry is a history event on a ticket the current user is assigned to.
// Used to render the Inbox activity feed.
type InboxEntry struct {
	HistoryEntry
	TicketDisplayID string `json:"ticket_display_id"`
	TicketTitle     string `json:"ticket_title"`
}
