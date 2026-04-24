package service

import (
	"context"
	"fmt"
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

func (s *BoardService) ListBoards(ctx context.Context, orgID uuid.UUID) ([]model.Board, error) {
	return s.store.ListBoards(ctx, orgID)
}

func (s *BoardService) ListBoardsByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.Board, error) {
	return s.store.ListBoardsByTeam(ctx, orgID, teamID)
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
		assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, boardID)
		tagsByTicket, _ := s.store.BulkListTicketTags(ctx, boardID)
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
	assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, boardID)
	tagsByTicket, _ := s.store.BulkListTicketTags(ctx, boardID)
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
func (s *BoardService) StartSprint(ctx context.Context, orgID, sprintID uuid.UUID) (*model.Sprint, error) {
	sp, err := s.store.GetSprint(ctx, orgID, sprintID)
	if err != nil {
		return nil, err
	}
	if sp.Status != model.SprintStatusPlanning {
		return nil, fmt.Errorf("sprint must be in planning status to start")
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

	assigneesByTicket, _ := s.store.BulkListTicketAssignees(ctx, boardID)
	blockedByBacklog, _ := s.store.BulkGetBlockedBy(ctx, orgID, boardID)
	for i := range tickets {
		tickets[i].Assignees = assigneesByTicket[tickets[i].ID]
		if blocker, ok := blockedByBacklog[tickets[i].ID]; ok {
			tickets[i].IsBlocked = true
			tickets[i].BlockedBy = blocker
		}
	}

	view := &model.BoardView{
		Board:        *board,
		Team:         team,
		Sprints:      sprints,
		Columns:      []model.ColumnView{{Tickets: tickets}},
		BacklogCount: len(tickets),
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

	return view, nil
}

// --- Tag service methods ---

func (s *BoardService) ListBoardTags(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Tag, error) {
	return s.store.ListTags(ctx, orgID, boardID)
}

func (s *BoardService) CreateTag(ctx context.Context, orgID, boardID uuid.UUID, name, color string) (*model.Tag, error) {
	return s.store.CreateTag(ctx, orgID, boardID, name, color)
}

func (s *BoardService) DeleteTag(ctx context.Context, orgID, tagID uuid.UUID) error {
	return s.store.DeleteTag(ctx, orgID, tagID)
}

func (s *BoardService) ListTicketTags(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.Tag, error) {
	return s.store.ListTicketTags(ctx, orgID, ticketID)
}

func (s *BoardService) AddTagToTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	return s.store.AddTagToTicket(ctx, orgID, ticketID, tagID)
}

func (s *BoardService) RemoveTagFromTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	return s.store.RemoveTagFromTicket(ctx, orgID, ticketID, tagID)
}
