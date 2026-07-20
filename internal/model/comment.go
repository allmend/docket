package model

import (
	"html/template"
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

// InboxEntry is a history event on a ticket the current user is assigned to.
// Used to render the Inbox activity feed.
type InboxEntry struct {
	HistoryEntry
	TicketDisplayID string `json:"ticket_display_id"`
	TicketTitle     string `json:"ticket_title"`
}
