package model

import (
	"time"

	"github.com/google/uuid"
)

type Team struct {
	ID            uuid.UUID `json:"id"`
	OrgID         uuid.UUID `json:"org_id"`
	Name          string    `json:"name"`
	Key           string    `json:"key"` // e.g. "ENG", "BE" — ticket prefix
	Description   string    `json:"description"`
	TicketCounter int       `json:"ticket_counter"`
	CreatedBy     uuid.UUID `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TeamWithBoard is used for navigation — loads the team and its single board together.
type TeamWithBoard struct {
	Team  Team
	Board *Board // nil if the team has no board yet
}

type BoardMode string

const (
	BoardModeKanban BoardMode = "kanban"
	BoardModeScrum  BoardMode = "scrum"
	BoardModeBlank  BoardMode = "blank"
)

type Board struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	TeamID      *uuid.UUID `json:"team_id,omitempty"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Mode        BoardMode  `json:"mode"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (m BoardMode) IsScrum() bool  { return m == BoardModeScrum }
func (m BoardMode) Label() string {
	switch m {
	case BoardModeScrum:
		return "Scrum"
	case BoardModeKanban:
		return "Kanban"
	default:
		return "Blank"
	}
}

type Column struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	BoardID   uuid.UUID `json:"board_id"`
	Name      string    `json:"name"`
	Position  float64   `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Tag struct {
	ID      uuid.UUID `json:"id"`
	OrgID   uuid.UUID `json:"org_id"`
	BoardID uuid.UUID `json:"board_id"`
	Name    string    `json:"name"`
	Color   string    `json:"color"`
}

type SprintStatus string

const (
	SprintStatusPlanning  SprintStatus = "planning"
	SprintStatusActive    SprintStatus = "active"
	SprintStatusCompleted SprintStatus = "completed"
)

type Sprint struct {
	ID        uuid.UUID    `json:"id"`
	OrgID     uuid.UUID    `json:"org_id"`
	BoardID   uuid.UUID    `json:"board_id"`
	Name      string       `json:"name"`
	Status    SprintStatus `json:"status"`
	StartDate *time.Time   `json:"start_date,omitempty"`
	EndDate   *time.Time   `json:"end_date,omitempty"`
	CreatedBy uuid.UUID    `json:"created_by"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (s SprintStatus) IsActive() bool    { return s == SprintStatusActive }
func (s SprintStatus) IsPlanning() bool  { return s == SprintStatusPlanning }
func (s SprintStatus) IsCompleted() bool { return s == SprintStatusCompleted }

// BoardView is the full board with columns and their tickets, used for rendering.
type BoardView struct {
	Board         Board
	Team          *Team        // nil if board has no team
	Columns       []ColumnView
	ActiveSprint  *Sprint      // nil for kanban/blank boards or scrum boards with no active sprint
	Sprints       []Sprint     // all sprints for this board (scrum only)
	SprintViews   []SprintView // planning sprints with their tickets (backlog page)
	BacklogCount  int          // tickets with sprint_id IS NULL (scrum only)
	FirstColumnID uuid.UUID    // first column's ID — used by New Ticket button
}

// SprintView is a planning sprint with its tickets, used in the backlog sectioned view.
type SprintView struct {
	Sprint  Sprint
	Tickets []Ticket
}

type ColumnView struct {
	Column    Column
	Tickets   []Ticket
	IsBacklog bool // true for the virtual backlog column shown on scrum sprint boards
	IsDone    bool // true if column name matches "done" (case-insensitive)
}
