package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type BoardService struct {
	store *store.Store
}

func isDone(name string) bool {
	return strings.EqualFold(name, "done")
}

func NewBoardService(st *store.Store) *BoardService {
	return &BoardService{store: st}
}

func (s *BoardService) GetBoard(ctx context.Context, orgID, boardID uuid.UUID) (*model.Board, error) {
	return s.store.GetBoard(ctx, orgID, boardID)
}

func (s *BoardService) GetBoardByTeam(ctx context.Context, orgID, teamID uuid.UUID) (*model.Board, error) {
	return s.store.GetBoardByTeamID(ctx, orgID, teamID)
}

// defaultColumns returns the seed column names for a board mode.
var defaultColumns = map[model.BoardMode][]string{
	model.BoardModeKanban: {"Backlog", "In Progress", "In Review", "Done"},
	model.BoardModeScrum:  {"To Do", "In Progress", "In Review", "Done"},
	model.BoardModeBlank:  {},
}

func (s *BoardService) CreateBoard(ctx context.Context, orgID, userID uuid.UUID, teamID *uuid.UUID, name, description string, mode model.BoardMode) (*model.Board, error) {
	if mode == "" {
		mode = model.BoardModeKanban
	}
	board, err := s.store.CreateBoard(ctx, orgID, userID, teamID, name, description, mode)
	if err != nil {
		return nil, err
	}

	for i, colName := range defaultColumns[mode] {
		if _, err := s.store.CreateColumn(ctx, orgID, board.ID, colName, float64((i+1)*1000)); err != nil {
			return nil, fmt.Errorf("seed column %q: %w", colName, err)
		}
	}
	return board, nil
}

func (s *BoardService) UpdateBoard(ctx context.Context, orgID, boardID uuid.UUID, name, description string) (*model.Board, error) {
	return s.store.UpdateBoard(ctx, orgID, boardID, name, description)
}

func (s *BoardService) DeleteBoard(ctx context.Context, orgID, boardID uuid.UUID) error {
	return s.store.DeleteBoard(ctx, orgID, boardID)
}

// GetBoardView returns the full board with its team, columns, and tickets.
// For scrum boards it also fetches sprint data and backlog count.
// For scrum boards with an active sprint it shows sprint tickets in columns;
// otherwise it shows all board tickets (kanban/blank) or an empty board (scrum with no active sprint).
func (s *BoardService) GetBoardView(ctx context.Context, orgID, boardID uuid.UUID) (*model.BoardView, error) {
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}

	var team *model.Team
	if board.TeamID != nil {
		team, err = s.store.GetTeam(ctx, orgID, *board.TeamID)
		if err != nil {
			return nil, fmt.Errorf("get team: %w", err)
		}
	}

	cols, err := s.store.ListColumns(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}

	view := &model.BoardView{Board: *board, Team: team}
	if len(cols) > 0 {
		view.FirstColumnID = cols[0].ID
	}
	view.BoardTags, _ = s.store.ListTags(ctx, orgID, boardID)

	if board.Mode == model.BoardModeScrum {
		// Load all sprints for sidebar/header display.
		sprints, err := s.store.ListSprints(ctx, orgID, boardID)
		if err != nil {
			return nil, fmt.Errorf("list sprints: %w", err)
		}
		view.Sprints = sprints

		// Backlog count: tickets with no sprint assigned.
		view.BacklogCount, _ = s.store.CountBacklogTickets(ctx, orgID, boardID)

		// Load all ticket assignees, tags, and blocked status for the board once.
		assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, orgID, boardID)
		tagsByTicket, _ := s.store.BulkListTicketTags(ctx, orgID, boardID)
		blockedBy, _ := s.store.BulkGetBlockedBy(ctx, orgID, boardID)

		// Active sprint: show ONLY sprint columns — no virtual backlog.
		// The board becomes a pure sprint view; the backlog is accessible via the Backlog link.
		activeSprint, err := s.store.GetActiveSprint(ctx, orgID, boardID)
		if err == nil {
			view.ActiveSprint = activeSprint
			tickets, err := s.store.ListSprintTickets(ctx, orgID, activeSprint.ID)
			if err != nil {
				return nil, fmt.Errorf("list sprint tickets: %w", err)
			}
			for i := range tickets {
				tickets[i].Assignees = assigneesByTicket[tickets[i].ID]
				tickets[i].Tags = tagsByTicket[tickets[i].ID]
				if blocker, ok := blockedBy[tickets[i].ID]; ok {
					tickets[i].IsBlocked = true
					tickets[i].BlockedBy = blocker
				}
			}
			byCol := make(map[uuid.UUID][]model.Ticket)
			for _, t := range tickets {
				byCol[t.ColumnID] = append(byCol[t.ColumnID], t)
			}
			for _, col := range cols {
				view.Columns = append(view.Columns, model.ColumnView{Column: col, Tickets: byCol[col.ID], IsDone: isDone(col.Name)})
			}
			return view, nil
		}

		// No active sprint: show virtual backlog + sprint columns.
		// For planning sprints, load and bucket their tickets so planning work persists on refresh.
		backlogTickets, _ := s.store.ListBacklogTickets(ctx, orgID, boardID)
		for i := range backlogTickets {
			backlogTickets[i].Assignees = assigneesByTicket[backlogTickets[i].ID]
			backlogTickets[i].Tags = tagsByTicket[backlogTickets[i].ID]
			if blocker, ok := blockedBy[backlogTickets[i].ID]; ok {
				backlogTickets[i].IsBlocked = true
				backlogTickets[i].BlockedBy = blocker
			}
		}
		view.Columns = append(view.Columns, model.ColumnView{
			Column:    model.Column{Name: "Backlog"},
			Tickets:   backlogTickets,
			IsBacklog: true,
		})

		// Find a planning sprint and show its tickets in their assigned columns.
		var planningSprint *model.Sprint
		for i := range sprints {
			if sprints[i].Status == model.SprintStatusPlanning {
				planningSprint = &sprints[i]
				break
			}
		}
		if planningSprint != nil {
			planningTickets, _ := s.store.ListSprintTickets(ctx, orgID, planningSprint.ID)
			for i := range planningTickets {
				planningTickets[i].Assignees = assigneesByTicket[planningTickets[i].ID]
				planningTickets[i].Tags = tagsByTicket[planningTickets[i].ID]
			}
			byCol := make(map[uuid.UUID][]model.Ticket)
			for _, t := range planningTickets {
				byCol[t.ColumnID] = append(byCol[t.ColumnID], t)
			}
			for _, col := range cols {
				view.Columns = append(view.Columns, model.ColumnView{Column: col, Tickets: byCol[col.ID], IsDone: isDone(col.Name)})
			}
		} else {
			// No planning sprint — empty sprint columns.
			for _, col := range cols {
				view.Columns = append(view.Columns, model.ColumnView{Column: col, IsDone: isDone(col.Name)})
			}
		}
		return view, nil
	}

	// Kanban / blank: show all tickets on the board.
	tickets, err := s.store.ListTicketsByBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, orgID, boardID)
	tagsByTicket, _ := s.store.BulkListTicketTags(ctx, orgID, boardID)
	blockedBy, _ := s.store.BulkGetBlockedBy(ctx, orgID, boardID)
	for i := range tickets {
		tickets[i].Assignees = assigneesByTicket[tickets[i].ID]
		tickets[i].Tags = tagsByTicket[tickets[i].ID]
		if blocker, ok := blockedBy[tickets[i].ID]; ok {
			tickets[i].IsBlocked = true
			tickets[i].BlockedBy = blocker
		}
	}
	byCol := make(map[uuid.UUID][]model.Ticket)
	for _, t := range tickets {
		byCol[t.ColumnID] = append(byCol[t.ColumnID], t)
	}
	for _, col := range cols {
		view.Columns = append(view.Columns, model.ColumnView{Column: col, Tickets: byCol[col.ID], IsDone: isDone(col.Name)})
	}
	return view, nil
}

func (s *BoardService) ListColumns(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Column, error) {
	return s.store.ListColumns(ctx, orgID, boardID)
}

func (s *BoardService) AddColumn(ctx context.Context, orgID, boardID uuid.UUID, name string) (*model.Column, error) {
	maxPos, err := s.store.MaxColumnPosition(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("max position: %w", err)
	}
	return s.store.CreateColumn(ctx, orgID, boardID, name, maxPos+1000)
}

func (s *BoardService) RenameColumn(ctx context.Context, orgID, columnID uuid.UUID, name string) (*model.Column, error) {
	col, err := s.store.GetColumn(ctx, orgID, columnID)
	if err != nil {
		return nil, err
	}
	if isDone(col.Name) {
		return nil, fmt.Errorf("the Done column cannot be renamed")
	}
	return s.store.RenameColumn(ctx, orgID, columnID, name)
}

func (s *BoardService) DeleteColumn(ctx context.Context, orgID, columnID uuid.UUID) error {
	col, err := s.store.GetColumn(ctx, orgID, columnID)
	if err != nil {
		return err
	}
	if isDone(col.Name) {
		return fmt.Errorf("the Done column cannot be deleted")
	}
	return s.store.DeleteColumn(ctx, orgID, columnID)
}

// --- Sprint service methods ---

func (s *BoardService) ListSprints(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Sprint, error) {
	return s.store.ListSprints(ctx, orgID, boardID)
}

func (s *BoardService) GetActiveSprint(ctx context.Context, orgID, boardID uuid.UUID) (*model.Sprint, error) {
	return s.store.GetActiveSprint(ctx, orgID, boardID)
}

func (s *BoardService) GetSprint(ctx context.Context, orgID, sprintID uuid.UUID) (*model.Sprint, error) {
	return s.store.GetSprint(ctx, orgID, sprintID)
}

func (s *BoardService) CreateSprint(ctx context.Context, orgID, boardID, userID uuid.UUID, name, goal string, startDate, endDate *time.Time) (*model.Sprint, error) {
	return s.store.CreateSprint(ctx, orgID, boardID, userID, name, goal, startDate, endDate)
}

func (s *BoardService) UpdateSprint(ctx context.Context, orgID, sprintID uuid.UUID, name, goal string, startDate, endDate *time.Time) (*model.Sprint, error) {
	return s.store.UpdateSprint(ctx, orgID, sprintID, name, goal, startDate, endDate)
}

// StartSprint transitions a planning sprint to active.
// Only one sprint may be active at a time — the DB unique partial index enforces this.
// If the sprint has no dates yet, the start/end passed from the planning duration
// picker are applied so the board header can show real "day N of M" progress.
func (s *BoardService) StartSprint(ctx context.Context, orgID, sprintID uuid.UUID, startDate, endDate *time.Time) (*model.Sprint, error) {
	sp, err := s.store.GetSprint(ctx, orgID, sprintID)
	if err != nil {
		return nil, err
	}
	if sp.Status != model.SprintStatusPlanning {
		return nil, fmt.Errorf("sprint must be in planning status to start")
	}
	// Backfill dates from the planning picker only when none were explicitly set.
	if sp.StartDate == nil && startDate != nil && endDate != nil {
		if _, err := s.store.UpdateSprint(ctx, orgID, sprintID, sp.Name, sp.Goal, startDate, endDate); err != nil {
			return nil, fmt.Errorf("set sprint dates: %w", err)
		}
	}
	return s.store.SetSprintStatus(ctx, orgID, sprintID, model.SprintStatusActive)
}

// CloseSprint transitions an active sprint to completed and returns unfinished
// tickets to the backlog.
func (s *BoardService) CloseSprint(ctx context.Context, orgID, sprintID, actorID uuid.UUID) (*model.Sprint, error) {
	sp, err := s.store.GetSprint(ctx, orgID, sprintID)
	if err != nil {
		return nil, err
	}
	if sp.Status != model.SprintStatusActive {
		return nil, fmt.Errorf("sprint must be active to close")
	}
	// Snapshot stats before moving tickets so counts remain accurate.
	if err := s.store.SnapshotSprintStats(ctx, orgID, sprintID); err != nil {
		return nil, fmt.Errorf("snapshot sprint stats: %w", err)
	}
	// Return non-done tickets to backlog before marking completed.
	if err := s.store.ReturnSprintTicketsToBacklog(ctx, orgID, sprintID); err != nil {
		return nil, fmt.Errorf("return tickets to backlog: %w", err)
	}
	// Record history on each blocked ticket before clearing resolved blocking links.
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	clearedLinks, _ := s.store.ListBlockingLinksForDoneTickets(ctx, orgID, sprintID)
	for _, l := range clearedLinks {
		_ = s.store.AppendHistory(ctx, l.ToTicketID, actorID, actorName, "link_cleared", "blocked by "+l.FromDisplayID, "")
	}
	if err := s.store.ClearBlockingLinksForDoneTickets(ctx, orgID, sprintID); err != nil {
		return nil, fmt.Errorf("clear resolved blocking links: %w", err)
	}
	return s.store.SetSprintStatus(ctx, orgID, sprintID, model.SprintStatusCompleted)
}

func (s *BoardService) DeleteSprint(ctx context.Context, orgID, sprintID uuid.UUID) error {
	return s.store.DeleteSprint(ctx, orgID, sprintID)
}

func (s *BoardService) AssignTicketToSprint(ctx context.Context, orgID, ticketID uuid.UUID, sprintID *uuid.UUID) error {
	return s.store.AssignTicketToSprint(ctx, orgID, ticketID, sprintID)
}

// AutoAssignToSprint assigns a ticket to the active or planning sprint for the board.
// Called when a backlog ticket is moved to a sprint column via the status dropdown so
// the column change is reflected on the board rather than being silently lost.
// Returns nil if there is no sprint to assign to — caller may ignore this gracefully.
func (s *BoardService) AutoAssignToSprint(ctx context.Context, orgID, ticketID, boardID uuid.UUID) error {
	// Prefer the active sprint; fall back to the first planning sprint.
	sprint, err := s.store.GetActiveSprint(ctx, orgID, boardID)
	if err != nil {
		sprints, err := s.store.ListSprints(ctx, orgID, boardID)
		if err != nil {
			return err
		}
		for i := range sprints {
			if sprints[i].Status == model.SprintStatusPlanning {
				sprint = &sprints[i]
				break
			}
		}
	}
	if sprint == nil {
		return nil // no sprint; nothing to do
	}
	return s.store.AssignTicketToSprint(ctx, orgID, ticketID, &sprint.ID)
}

func (s *BoardService) GetBacklog(ctx context.Context, orgID, boardID uuid.UUID) (*model.BoardView, error) {
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}
	var team *model.Team
	if board.TeamID != nil {
		team, err = s.store.GetTeam(ctx, orgID, *board.TeamID)
		if err != nil {
			return nil, fmt.Errorf("get team: %w", err)
		}
	}
	tickets, err := s.store.ListBacklogTickets(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list backlog: %w", err)
	}
	sprints, err := s.store.ListSprints(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	cols, err := s.store.ListColumns(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}

	assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, orgID, boardID)
	blockedByBacklog, _ := s.store.BulkGetBlockedBy(ctx, orgID, boardID)
	for i := range tickets {
		tickets[i].Assignees = assigneesByTicket[tickets[i].ID]
		if blocker, ok := blockedByBacklog[tickets[i].ID]; ok {
			tickets[i].IsBlocked = true
			tickets[i].BlockedBy = blocker
		}
	}

	var totalPts int
	var unestimated int
	for _, t := range tickets {
		if t.StoryPoints != nil {
			totalPts += int(*t.StoryPoints)
		} else {
			unestimated++
		}
	}

	view := &model.BoardView{
		Board:            *board,
		Team:             team,
		Sprints:          sprints,
		Columns:          []model.ColumnView{{IsBacklog: true, Tickets: tickets}},
		BacklogCount:     len(tickets),
		BacklogPoints:    totalPts,
		UnestimatedCount: unestimated,
	}
	if len(cols) > 0 {
		view.FirstColumnID = cols[0].ID
	}

	for i := range sprints {
		if sprints[i].Status != model.SprintStatusPlanning {
			continue
		}
		st, _ := s.store.ListSprintTickets(ctx, orgID, sprints[i].ID)
		for j := range st {
			st[j].Assignees = assigneesByTicket[st[j].ID]
		}
		view.SprintViews = append(view.SprintViews, model.SprintView{
			Sprint:  sprints[i],
			Tickets: st,
		})
	}

	// Load active sprint and group its tickets by column for the backlog sprint section.
	if activeSprint, err := s.store.GetActiveSprint(ctx, orgID, boardID); err == nil {
		view.ActiveSprint = activeSprint
		sprintTickets, _ := s.store.ListSprintTickets(ctx, orgID, activeSprint.ID)
		for i := range sprintTickets {
			sprintTickets[i].Assignees = assigneesByTicket[sprintTickets[i].ID]
		}

		type colInfo struct {
			name   string
			isDone bool
		}
		colLookup := make(map[uuid.UUID]colInfo, len(cols))
		for _, c := range cols {
			colLookup[c.ID] = colInfo{name: c.Name, isDone: strings.EqualFold(c.Name, "done")}
		}
		groupMap := make(map[uuid.UUID][]model.Ticket, len(cols))
		for _, t := range sprintTickets {
			groupMap[t.ColumnID] = append(groupMap[t.ColumnID], t)
		}

		section := &model.ActiveSprintSection{Sprint: *activeSprint, Total: len(sprintTickets)}
		for _, c := range cols {
			grp := groupMap[c.ID]
			if len(grp) == 0 {
				continue
			}
			info := colLookup[c.ID]
			if info.isDone {
				section.Done += len(grp)
			}
			section.Columns = append(section.Columns, model.SprintColumnGroup{
				Name:    info.name,
				IsDone:  info.isDone,
				Tickets: grp,
			})
		}
		view.ActiveSprintSection = section
	}

	return view, nil
}

// --- Tag service methods ---

func (s *BoardService) ListBoardTags(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Tag, error) {
	return s.store.ListTags(ctx, orgID, boardID)
}

func (s *BoardService) GetTag(ctx context.Context, orgID, tagID uuid.UUID) (*model.Tag, error) {
	return s.store.GetTag(ctx, orgID, tagID)
}

func (s *BoardService) ListTicketsByTag(ctx context.Context, orgID, tagID uuid.UUID) ([]model.Ticket, error) {
	return s.store.ListTicketsByTag(ctx, orgID, tagID)
}

func (s *BoardService) CreateTag(ctx context.Context, orgID, boardID uuid.UUID, name, color, description string, leadUserID *uuid.UUID) (*model.Tag, error) {
	return s.store.CreateTag(ctx, orgID, boardID, name, color, description, leadUserID)
}

func (s *BoardService) UpdateTag(ctx context.Context, orgID, tagID uuid.UUID, name, color, description string, leadUserID *uuid.UUID) (*model.Tag, error) {
	return s.store.UpdateTag(ctx, orgID, tagID, name, color, description, leadUserID)
}

func (s *BoardService) ListTrackStats(ctx context.Context, orgID, boardID uuid.UUID) ([]model.TrackStat, error) {
	return s.store.ListTrackStats(ctx, orgID, boardID)
}

func (s *BoardService) DeleteTag(ctx context.Context, orgID, tagID uuid.UUID) error {
	return s.store.DeleteTag(ctx, orgID, tagID)
}

func (s *BoardService) ListTicketTags(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.Tag, error) {
	return s.store.ListTicketTags(ctx, orgID, ticketID)
}

// --- Sprint capacity methods ---

// GetSprintCapacity returns capacity data for a sprint, seeding from team members first.
func (s *BoardService) GetSprintCapacity(ctx context.Context, orgID, boardID, sprintID uuid.UUID) (*model.SprintCapacity, error) {
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, err
	}
	if board.TeamID != nil {
		_ = s.store.SeedSprintCapacity(ctx, orgID, sprintID, *board.TeamID)
	}

	members, err := s.store.GetSprintCapacity(ctx, orgID, sprintID)
	if err != nil {
		return nil, err
	}

	sp, err := s.store.GetSprint(ctx, orgID, sprintID)
	if err != nil {
		return nil, err
	}

	totalFocus := 0
	for _, m := range members {
		totalFocus += m.FocusPct
	}

	return &model.SprintCapacity{
		SprintID:        sprintID,
		Members:         members,
		CommittedPoints: sp.CommittedPoints,
		TotalFocusPct:   totalFocus,
	}, nil
}

// SetMemberCapacity updates one member's focus_pct for a sprint.
func (s *BoardService) SetMemberCapacity(ctx context.Context, orgID, sprintID, userID uuid.UUID, focusPct int) error {
	if focusPct < 0 {
		focusPct = 0
	}
	if focusPct > 100 {
		focusPct = 100
	}
	return s.store.UpsertSprintCapacity(ctx, orgID, sprintID, userID, focusPct)
}

func (s *BoardService) AddTagToTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	return s.store.AddTagToTicket(ctx, orgID, ticketID, tagID)
}

func (s *BoardService) RemoveTagFromTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	return s.store.RemoveTagFromTicket(ctx, orgID, ticketID, tagID)
}

// --- Roadmap ---

// GetRoadmap returns all sprints for a board with their ticket summaries, ordered by start date.
func (s *BoardService) GetRoadmap(ctx context.Context, orgID, boardID uuid.UUID) ([]model.RoadmapSprintView, error) {
	sprints, err := s.store.ListSprints(ctx, orgID, boardID)
	if err != nil {
		return nil, err
	}

	views := make([]model.RoadmapSprintView, 0, len(sprints))
	for _, sp := range sprints {
		tickets, _ := s.store.ListSprintTicketsSummary(ctx, orgID, sp.ID)
		views = append(views, model.RoadmapSprintView{Sprint: sp, Tickets: tickets})
	}
	return views, nil
}

// --- Definition of Done methods ---

func (s *BoardService) ListDodItems(ctx context.Context, orgID, boardID uuid.UUID) ([]model.DodItem, error) {
	return s.store.ListDodItems(ctx, orgID, boardID)
}

func (s *BoardService) CreateDodItem(ctx context.Context, orgID, boardID uuid.UUID, text string) (*model.DodItem, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	return s.store.CreateDodItem(ctx, orgID, boardID, text)
}

func (s *BoardService) DeleteDodItem(ctx context.Context, orgID, itemID uuid.UUID) error {
	return s.store.DeleteDodItem(ctx, orgID, itemID)
}

func (s *BoardService) GetTicketDod(ctx context.Context, orgID, boardID, ticketID uuid.UUID) ([]model.DodItemWithCheck, error) {
	return s.store.GetTicketDod(ctx, orgID, boardID, ticketID)
}

func (s *BoardService) ToggleDodCheck(ctx context.Context, orgID, ticketID, itemID uuid.UUID, checked bool) error {
	return s.store.ToggleDodCheck(ctx, orgID, ticketID, itemID, checked)
}

// GetDailyScrumView returns filtered sprint tickets grouped by assignee for the Daily Scrum page.
func (s *BoardService) GetDailyScrumView(ctx context.Context, orgID, boardID uuid.UUID, filters model.DailyScrumFilters) (*model.DailyScrumView, error) {
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}

	var team *model.Team
	if board.TeamID != nil {
		if t, err := s.store.GetTeam(ctx, orgID, *board.TeamID); err == nil {
			team = t
		}
	}

	allTags, _ := s.store.ListTags(ctx, orgID, boardID)
	var allAssignees []model.User
	if team != nil {
		allAssignees, _ = s.store.ListTeamMembers(ctx, orgID, team.ID)
	}

	activeSprint, _ := s.store.GetActiveSprint(ctx, orgID, boardID)
	if activeSprint == nil {
		return &model.DailyScrumView{
			Board:        *board,
			Team:         team,
			AllAssignees: allAssignees,
			AllTags:      allTags,
			Filters:      filters,
		}, nil
	}

	cols, err := s.store.ListColumns(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	colNames := make(map[uuid.UUID]string, len(cols))
	for _, c := range cols {
		colNames[c.ID] = c.Name
	}

	tickets, err := s.store.ListSprintTickets(ctx, orgID, activeSprint.ID)
	if err != nil {
		return nil, fmt.Errorf("list sprint tickets: %w", err)
	}
	assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, orgID, boardID)
	tagsByTicket, _ := s.store.BulkListTicketTags(ctx, orgID, boardID)
	blockedBy, _ := s.store.BulkGetBlockedBy(ctx, orgID, boardID)
	for i := range tickets {
		tickets[i].Assignees = assigneesByTicket[tickets[i].ID]
		tickets[i].Tags = tagsByTicket[tickets[i].ID]
		if blocker, ok := blockedBy[tickets[i].ID]; ok {
			tickets[i].IsBlocked = true
			tickets[i].BlockedBy = blocker
		}
	}

	filtered := filterDailyScrumTickets(tickets, filters)

	type entry struct {
		user    model.User
		tickets []model.DailyScrumTicket
	}
	groups := make(map[uuid.UUID]*entry)
	var order []uuid.UUID
	var unassigned []model.DailyScrumTicket

	for _, t := range filtered {
		dt := model.DailyScrumTicket{Ticket: t, ColumnName: colNames[t.ColumnID]}
		if len(t.Assignees) == 0 {
			unassigned = append(unassigned, dt)
			continue
		}
		for _, u := range t.Assignees {
			if _, exists := groups[u.ID]; !exists {
				groups[u.ID] = &entry{user: u}
				order = append(order, u.ID)
			}
			groups[u.ID].tickets = append(groups[u.ID].tickets, dt)
		}
	}

	result := make([]model.DailyScrumGroup, 0, len(order))
	for _, id := range order {
		g := groups[id]
		result = append(result, model.DailyScrumGroup{User: g.user, Tickets: g.tickets})
	}

	return &model.DailyScrumView{
		Board:        *board,
		Team:         team,
		ActiveSprint: activeSprint,
		Groups:       result,
		Unassigned:   unassigned,
		AllAssignees: allAssignees,
		AllTags:      allTags,
		Filters:      filters,
	}, nil
}

func filterDailyScrumTickets(tickets []model.Ticket, f model.DailyScrumFilters) []model.Ticket {
	if !f.HasFilters() {
		return tickets
	}
	q := strings.ToLower(f.Q)
	out := make([]model.Ticket, 0, len(tickets))
	for _, t := range tickets {
		if q != "" && !strings.Contains(strings.ToLower(t.Title), q) {
			continue
		}
		if len(f.Priorities) > 0 && !slices.Contains(f.Priorities, string(t.Priority)) {
			continue
		}
		if len(f.AssigneeIDs) > 0 || f.FilterUnassigned {
			matched := false
			if f.FilterUnassigned && len(t.Assignees) == 0 {
				matched = true
			}
			if !matched {
				for _, aid := range f.AssigneeIDs {
					for _, a := range t.Assignees {
						if a.ID.String() == aid {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		if len(f.TagIDs) > 0 {
			matched := false
			for _, tid := range f.TagIDs {
				for _, tag := range t.Tags {
					if tag.ID.String() == tid {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
