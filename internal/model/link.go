package model

import (
	"time"

	"github.com/google/uuid"
)

type RelationType string

const (
	RelationBlocks     RelationType = "blocks"
	RelationDependsOn  RelationType = "depends_on"
	RelationDuplicates RelationType = "duplicates"
	RelationRelatesTo  RelationType = "relates_to"
)

// Label returns a human-readable phrase for the relation, from the perspective
// of the ticket that owns the link.
func (r RelationType) Label() string {
	switch r {
	case RelationBlocks:
		return "Blocks"
	case RelationDependsOn:
		return "Depends on"
	case RelationDuplicates:
		return "Duplicates"
	case RelationRelatesTo:
		return "Relates to"
	default:
		return string(r)
	}
}

type TicketLink struct {
	ID           uuid.UUID    `json:"id"`
	OrgID        uuid.UUID    `json:"org_id"`
	FromTicketID uuid.UUID    `json:"from_ticket_id"`
	ToTicketID   uuid.UUID    `json:"to_ticket_id"`
	Relation     RelationType `json:"relation_type"`
	CreatedAt    time.Time    `json:"created_at"`

	// Denormalised for display — populated by store joins.
	FromDisplayID  string     `json:"from_display_id"`
	FromTitle      string     `json:"from_title"`
	FromClosedAt   *time.Time `json:"from_closed_at,omitempty"`
	FromColumnName string     `json:"from_column_name"`
	ToDisplayID    string     `json:"to_display_id"`
	ToTitle        string     `json:"to_title"`
	ToClosedAt     *time.Time `json:"to_closed_at,omitempty"`
	ToColumnName   string     `json:"to_column_name"`
}
