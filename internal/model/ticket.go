package model

import (
	"fmt"
	"html/template"
	"time"

	"github.com/allmend/docket/internal/markdown"
	"github.com/google/uuid"
)

type Priority string

const (
	PriorityNone     Priority = ""
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type Ticket struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	BoardID     uuid.UUID  `json:"board_id"`
	ColumnID    uuid.UUID  `json:"column_id"`
	TeamID      *uuid.UUID `json:"team_id,omitempty"`
	AssigneeID  *uuid.UUID `json:"assignee_id,omitempty"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	Number      int        `json:"number"`     // sequential per team
	TeamKey     string     `json:"team_key"`   // denormalised for display
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Priority     Priority   `json:"priority"`
	StoryPoints  *float64   `json:"story_points,omitempty"`
	Position     float64    `json:"position"`
	SprintID     *uuid.UUID `json:"sprint_id,omitempty"`
	ExternalRef  *string    `json:"external_ref,omitempty"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	CloseReason  *string    `json:"close_reason,omitempty"`
	Tags         []Tag      `json:"tags,omitempty"`

	// Denormalised for display — populated by joins
	AssigneeName *string     `json:"assignee_name,omitempty"`
	Assignees    []User      `json:"assignees,omitempty"` // populated by board view bulk-load
	IsBlocked    bool        `json:"is_blocked"`          // populated by board view bulk-load
	BlockedBy    string      `json:"blocked_by"`          // display ID of the blocking ticket

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DisplayID returns the human-readable ticket identifier, e.g. "PROJ-42".
// Falls back to a short UUID fragment if no team is set.
func (t *Ticket) DisplayID() string {
	if t.TeamKey != "" && t.Number > 0 {
		return fmt.Sprintf("%s-%d", t.TeamKey, t.Number)
	}
	return t.ID.String()[:8]
}

// BodyHTML renders the ticket body as sanitised HTML.
func (t *Ticket) BodyHTML() template.HTML {
	return markdown.Render(t.Body)
}

func (p Priority) String() string { return string(p) }

// BadgeClass maps priority to a DaisyUI badge class.
func (p Priority) BadgeClass() string {
	switch p {
	case PriorityLow:
		return "badge-neutral"
	case PriorityMedium:
		return "badge-info"
	case PriorityHigh:
		return "badge-warning"
	case PriorityCritical:
		return "badge-error"
	default:
		return "badge-neutral"
	}
}
