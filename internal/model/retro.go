package model

import (
	"time"

	"github.com/google/uuid"
)

type RetroColumn string

const (
	RetroWentWell    RetroColumn = "went_well"
	RetroDidntGoWell RetroColumn = "didnt_go_well"
	RetroActionItem  RetroColumn = "action_item"
)

func (c RetroColumn) Label() string {
	switch c {
	case RetroWentWell:
		return "What Went Well"
	case RetroDidntGoWell:
		return "What Didn't Go Well"
	case RetroActionItem:
		return "Action Items"
	}
	return string(c)
}

type RetroBoard struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	BoardID   uuid.UUID  `json:"board_id"`
	SprintID  *uuid.UUID `json:"sprint_id"`
	Status    string     `json:"status"` // "open" | "closed"
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (r RetroBoard) IsClosed() bool { return r.Status == "closed" }

type RetroCard struct {
	ID            uuid.UUID   `json:"id"`
	OrgID         uuid.UUID   `json:"org_id"`
	RetroBoardID  uuid.UUID   `json:"retro_board_id"`
	Column        RetroColumn `json:"column_name"`
	Body          string      `json:"body"`
	AuthorID      uuid.UUID   `json:"author_id"` // never exposed in UI to peers
	OwnerID       *uuid.UUID  `json:"owner_id"`
	OwnerName     string      `json:"owner_name"`
	TicketID      *uuid.UUID  `json:"ticket_id"`
	TicketDisplay string      `json:"ticket_display"` // e.g. "ENG-42", populated on read
	CreatedAt     time.Time   `json:"created_at"`
}

type RetroView struct {
	RetroBoard  RetroBoard
	Board       Board
	Sprint      *Sprint
	WentWell    []RetroCard
	DidntGoWell []RetroCard
	ActionItems []RetroCard
}

type SprintReviewData struct {
	Sprint    Sprint
	Board     Board
	Completed []Ticket // tickets in Done columns
	Returned  []Ticket // tickets returning to backlog
}

type RetroListItem struct {
	RetroBoard       RetroBoard
	SprintName       string
	Sprint           *Sprint
	ClosedAt         *time.Time
	WentWellCount    int
	DidntGoWellCount int
	ActionItemCount  int
}
