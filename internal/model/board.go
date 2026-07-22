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
	Key           string    `json:"key"`  // e.g. "ENG", "BE" — ticket prefix
	Slug          string    `json:"slug"` // URL-safe identifier, e.g. "backend-engineering"
	Description   string    `json:"description"`
	TicketCounter  int      `json:"ticket_counter"`
	SprintCapacity int      `json:"sprint_capacity"` // story points per sprint, denominator of the planning committed bar
	CreatedBy     uuid.UUID `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TeamWithBoard is used for navigation — loads the team, its board, active sprint, and tags.
type TeamWithBoard struct {
	Team         Team
	Board        *Board  // nil if the team has no board yet
	ActiveSprint *Sprint // nil if no active sprint
	Tags         []Tag   // board labels/tracks, empty if none
	Members      []User  // first few members for avatar display
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

func (m BoardMode) IsScrum() bool { return m == BoardModeScrum }
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
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	BoardID     uuid.UUID  `json:"board_id"`
	Name        string     `json:"name"`
	Color       string     `json:"color"`
	Description string     `json:"description"`
	LeadUserID  *uuid.UUID `json:"lead_user_id"` // optional track lead
}

// TrackStat is a tag with its lead and open-work counters, for the
// workspace settings Tracks panel.
type TrackStat struct {
	Tag
	LeadName   string
	OpenCount  int
	OpenPoints float64
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

// DayNumber returns the 1-based current day within the sprint (0 if no start date).
func (s Sprint) DayNumber() int {
	if s.StartDate == nil {
		return 0
	}
	d := int(time.Since(*s.StartDate).Hours()/24) + 1
	if d < 1 {
		return 1
	}
	return d
}

// TotalDays returns the planned length of the sprint in days (0 if dates missing).
func (s Sprint) TotalDays() int {
	if s.StartDate == nil || s.EndDate == nil {
		return 0
	}
	return int(s.EndDate.Sub(*s.StartDate).Hours()/24) + 1
}

// ScheduleLabel describes where the sprint is against its end date: "5 days left",
// "last day", or "3 days overdue". Returns "" when the sprint has no dates.
//
// It exists because the templates computed TotalDays−DayNumber inline and always
// suffixed "days left", so an overrunning sprint read "-17 days left".
func (s Sprint) ScheduleLabel() string {
	if s.StartDate == nil || s.EndDate == nil {
		return ""
	}
	switch d := s.TotalDays() - s.DayNumber(); {
	case d > 1:
		return fmt.Sprintf("%d days left", d)
	case d == 1:
		return "1 day left"
	case d == 0:
		return "last day"
	case d == -1:
		return "1 day overdue"
	default:
		return fmt.Sprintf("%d days overdue", -d)
	}
}

// RemainingPoints returns committed − completed story points.
func (s Sprint) RemainingPoints() float64 {
	r := s.CommittedPoints - s.CompletedPoints
	if r < 0 {
		return 0
	}
	return r
}

// BoardView is the full board with columns and their tickets, used for rendering.
type BoardView struct {
	Board               Board
	Team                *Team // nil if board has no team
	Columns             []ColumnView
	ActiveSprint        *Sprint              // nil for kanban/blank boards or scrum boards with no active sprint
	Sprints             []Sprint             // all sprints for this board (scrum only)
	SprintViews         []SprintView         // planning sprints with their tickets (backlog page)
	BacklogCount        int                  // tickets with sprint_id IS NULL (scrum only)
	BacklogPoints       int                  // sum of story points for backlog tickets
	UnestimatedCount    int                  // backlog tickets with no story points
	FirstColumnID       uuid.UUID            // first column's ID — used by New Ticket button
	ActiveSprintSection *ActiveSprintSection // backlog page: active sprint tickets grouped by column
	BoardTags           []Tag                // all board labels/tracks — used by the board filter
}

// ActiveSprintSection is the active sprint shown above the backlog list.
type ActiveSprintSection struct {
	Sprint  Sprint
	Columns []SprintColumnGroup
	Total   int
	Done    int
}

// SprintColumnGroup is one column's tickets within the active sprint section.
type SprintColumnGroup struct {
	Name    string
	IsDone  bool
	Tickets []Ticket
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

// TotalPoints sums story points across all tickets in this column. Returns "" when zero.
func (cv ColumnView) TotalPoints() string {
	var total float64
	for _, t := range cv.Tickets {
		if t.StoryPoints != nil {
			total += *t.StoryPoints
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f pt", total)
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

// DailyScrumFilters holds active filter state for the Daily Scrum view.
type DailyScrumFilters struct {
	Q                string
	AssigneeIDs      []string
	TagIDs           []string
	Priorities       []string
	FilterUnassigned bool
}

func (f DailyScrumFilters) HasFilters() bool {
	return f.Q != "" || len(f.AssigneeIDs) > 0 || len(f.TagIDs) > 0 || len(f.Priorities) > 0 || f.FilterUnassigned
}

// DailyScrumTicket is a ticket annotated with its column name for the Daily Scrum view.
type DailyScrumTicket struct {
	Ticket     Ticket
	ColumnName string
}

// DailyScrumGroup is one assignee and their sprint tickets.
type DailyScrumGroup struct {
	User    User
	Tickets []DailyScrumTicket
}

// DailyScrumView is the full data for the Daily Scrum page.
type DailyScrumView struct {
	Board        Board
	Team         *Team
	ActiveSprint *Sprint
	Groups       []DailyScrumGroup
	Unassigned   []DailyScrumTicket
	AllAssignees []User
	AllTags      []Tag
	Filters      DailyScrumFilters
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
