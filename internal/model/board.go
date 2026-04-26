package model

import (
	"fmt"
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
	ID               uuid.UUID    `json:"id"`
	OrgID            uuid.UUID    `json:"org_id"`
	BoardID          uuid.UUID    `json:"board_id"`
	Name             string       `json:"name"`
	Goal             string       `json:"goal"`
	Status           SprintStatus `json:"status"`
	StartDate        *time.Time   `json:"start_date,omitempty"`
	EndDate          *time.Time   `json:"end_date,omitempty"`
	CommittedTickets int          `json:"committed_tickets"`
	CompletedTickets int          `json:"completed_tickets"`
	CommittedPoints  float64      `json:"committed_points"`
	CompletedPoints  float64      `json:"completed_points"`
	CreatedBy        uuid.UUID    `json:"created_by"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// CompletionPct returns ticket completion percentage, 0 if no tickets committed.
func (s Sprint) CompletionPct() int {
	if s.CommittedTickets == 0 {
		return 0
	}
	return int(float64(s.CompletedTickets) / float64(s.CommittedTickets) * 100)
}

// PointsPct returns story point completion percentage, 0 if no points committed.
func (s Sprint) PointsPct() int {
	if s.CommittedPoints == 0 {
		return 0
	}
	return int(s.CompletedPoints / s.CommittedPoints * 100)
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

// RoadmapTicket is a lightweight ticket summary for the roadmap view.
type RoadmapTicket struct {
	ID       uuid.UUID
	Title    string
	Priority Priority
	TeamKey  string
	Number   int
	IsDone   bool
}

func (t RoadmapTicket) DisplayID() string {
	if t.TeamKey != "" && t.Number > 0 {
		return fmt.Sprintf("%s-%d", t.TeamKey, t.Number)
	}
	return t.ID.String()[:8]
}

// RoadmapSprintView is one sprint row in the roadmap, with its tickets.
type RoadmapSprintView struct {
	Sprint  Sprint
	Tickets []RoadmapTicket
}

// DodItem is one checklist item in a board's Definition of Done.
type DodItem struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	BoardID   uuid.UUID `json:"board_id"`
	Text      string    `json:"text"`
	Position  float64   `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// DodItemWithCheck is a DoD item annotated with its checked state for a specific ticket.
type DodItemWithCheck struct {
	DodItem
	Checked bool
}

// SprintCapacityMember is one member's availability entry for a sprint.
type SprintCapacityMember struct {
	UserID   uuid.UUID
	Name     string
	Username string
	FocusPct int // 0–100
}

// SprintCapacity holds all member availability rows for one sprint plus summary fields.
type SprintCapacity struct {
	SprintID        uuid.UUID
	Members         []SprintCapacityMember
	CommittedPoints float64
	TotalFocusPct   int // sum of all member focus_pct values
}

// TeamDays returns total available team-days given sprint length in calendar days.
// Returns -1 if sprint has no dates set.
func (c SprintCapacity) TeamDays(sprintDays int) float64 {
	if sprintDays <= 0 || len(c.Members) == 0 {
		return -1
	}
	var sum float64
	for _, m := range c.Members {
		sum += float64(m.FocusPct) / 100.0
	}
	return sum * float64(sprintDays)
}
