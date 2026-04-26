package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/allmend/docket/internal/metrics"
	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type TicketService struct {
	store         *store.Store
	notifications *NotificationService
}

func NewTicketService(st *store.Store, notifications *NotificationService) *TicketService {
	return &TicketService{store: st, notifications: notifications}
}

func (s *TicketService) GetTicket(ctx context.Context, orgID, ticketID uuid.UUID) (*model.Ticket, error) {
	return s.store.GetTicket(ctx, orgID, ticketID)
}

// GetByRef resolves a ticket by its human-readable reference (e.g. "ENG", 42 → ENG-42).
func (s *TicketService) GetByRef(ctx context.Context, orgID uuid.UUID, teamKey string, number int) (*model.Ticket, error) {
	return s.store.GetTicketByTeamRef(ctx, orgID, teamKey, number)
}

// ListByTeam returns all tickets belonging to a team, ordered by number.
func (s *TicketService) ListByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.Ticket, error) {
	return s.store.ListTicketsByTeam(ctx, orgID, teamID)
}

// CreateTicketInTeam creates a ticket with an explicit team (used by the public API
// where the team is resolved from the URL key before calling this).
func (s *TicketService) CreateTicketInTeam(ctx context.Context,
	orgID, boardID, columnID, createdBy, teamID uuid.UUID,
	title, body string, priority model.Priority,
) (*model.Ticket, error) {
	number, err := s.store.NextTicketNumber(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("next ticket number: %w", err)
	}

	maxPos, err := s.store.MaxTicketPositionInColumn(ctx, columnID)
	if err != nil {
		return nil, fmt.Errorf("max position: %w", err)
	}

	return s.store.CreateTicket(ctx,
		orgID, boardID, columnID, createdBy,
		&teamID, number,
		title, body, priority, maxPos+1000,
	)
}

func (s *TicketService) CreateTicket(ctx context.Context,
	orgID, boardID, columnID, createdBy uuid.UUID,
	title, body string,
	priority model.Priority,
) (*model.Ticket, error) {
	// Resolve team from board so we can assign a sequential ticket number.
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}

	var number int
	if board.TeamID != nil {
		number, err = s.store.NextTicketNumber(ctx, *board.TeamID)
		if err != nil {
			return nil, fmt.Errorf("next ticket number: %w", err)
		}
	}

	maxPos, err := s.store.MaxTicketPositionInColumn(ctx, columnID)
	if err != nil {
		return nil, fmt.Errorf("max position: %w", err)
	}

	ticket, err := s.store.CreateTicket(ctx,
		orgID, boardID, columnID, createdBy,
		board.TeamID, number,
		title, body, priority, maxPos+1000,
	)
	if err != nil {
		return nil, err
	}

	// New tickets always land in the backlog (sprint_id = NULL).
	// Users explicitly assign tickets to a sprint from the backlog page.
	return ticket, nil
}

func (s *TicketService) UpdateTicket(ctx context.Context,
	orgID, ticketID, actorID uuid.UUID,
	title, body string,
	priority model.Priority,
	assigneeID *uuid.UUID,
) (*model.Ticket, error) {
	old, err := s.store.GetTicket(ctx, orgID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	updated, err := s.store.UpdateTicket(ctx, orgID, ticketID, title, body, priority, assigneeID)
	if err != nil {
		return nil, err
	}

	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}

	if old.Title != title {
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "title", old.Title, title)
	}
	if old.Priority != priority {
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "priority", string(old.Priority), string(priority))
	}
	oldAssignee := ""
	if old.AssigneeName != nil {
		oldAssignee = *old.AssigneeName
	}
	newAssignee := ""
	if updated.AssigneeName != nil {
		newAssignee = *updated.AssigneeName
	}
	if oldAssignee != newAssignee {
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "assignee", oldAssignee, newAssignee)
	}
	if old.Body != body {
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "description", "(previous)", "(updated)")
	}

	return updated, nil
}

// MoveTicket repositions a ticket using fractional indexing.
// prevPos and nextPos are the positions of the adjacent cards (0 means boundary).
func (s *TicketService) MoveTicket(ctx context.Context,
	orgID, ticketID, targetColumnID uuid.UUID,
	prevPos, nextPos float64,
) error {
	var newPos float64
	switch {
	case prevPos == 0 && nextPos == 0:
		newPos = 1000
	case prevPos == 0:
		newPos = nextPos / 2
	case nextPos == 0:
		newPos = prevPos + 1000
	default:
		newPos = (prevPos + nextPos) / 2
	}

	if nextPos != 0 && (newPos-prevPos) < 0.001 {
		if err := s.rebalanceColumn(ctx, orgID, targetColumnID); err != nil {
			return fmt.Errorf("rebalance: %w", err)
		}
		return s.MoveTicket(ctx, orgID, ticketID, targetColumnID, prevPos, nextPos)
	}

	// Capture from-column info before the move for the transition counter.
	var fromCol, fromTeam string
	if old, err := s.store.GetTicket(ctx, orgID, ticketID); err == nil && old.ColumnID != uuid.Nil {
		fromCol, fromTeam, _ = s.store.GetColumnMeta(ctx, old.ColumnID)
	}

	if err := s.store.MoveTicket(ctx, orgID, ticketID, targetColumnID, newPos); err != nil {
		return err
	}

	ticket, _ := s.store.GetTicket(ctx, orgID, ticketID)
	newCol, _ := s.store.GetColumn(ctx, orgID, targetColumnID)
	if ticket != nil && newCol != nil {
		isDone := strings.EqualFold(newCol.Name, "done")
		if isDone && ticket.ClosedAt == nil {
			_, _ = s.store.CloseTicket(ctx, orgID, ticketID, "done")
		} else if !isDone && ticket.ClosedAt != nil {
			_, _ = s.store.ReopenTicket(ctx, orgID, ticketID)
		}

		toCol, toTeam, _ := s.store.GetColumnMeta(ctx, targetColumnID)
		team := fromTeam
		if team == "" {
			team = toTeam
		}
		metrics.TicketTransitions.WithLabelValues(orgID.String(), team, fromCol, toCol).Inc()
	}

	return nil
}

// MoveToColumn moves a ticket to a target column (placing it last) and records history.
func (s *TicketService) MoveToColumn(ctx context.Context, orgID, ticketID, columnID, actorID uuid.UUID) (*model.Ticket, error) {
	old, err := s.store.GetTicket(ctx, orgID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	oldCol, _ := s.store.GetColumn(ctx, orgID, old.ColumnID)
	newCol, err := s.store.GetColumn(ctx, orgID, columnID)
	if err != nil {
		return nil, fmt.Errorf("get column: %w", err)
	}

	maxPos, _ := s.store.MaxTicketPositionInColumn(ctx, columnID)
	if err := s.store.MoveTicket(ctx, orgID, ticketID, columnID, maxPos+1000); err != nil {
		return nil, err
	}

	isDone := strings.EqualFold(newCol.Name, "done")
	wasDone := oldCol != nil && strings.EqualFold(oldCol.Name, "done")
	if isDone && old.ClosedAt == nil {
		_, _ = s.store.CloseTicket(ctx, orgID, ticketID, "done")
	} else if !isDone && wasDone {
		_, _ = s.store.ReopenTicket(ctx, orgID, ticketID)
	}

	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	oldName := ""
	if oldCol != nil {
		oldName = oldCol.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "column", oldName, newCol.Name)

	return s.store.GetTicket(ctx, orgID, ticketID)
}

func (s *TicketService) UpdatePriority(ctx context.Context, orgID, ticketID, actorID uuid.UUID, priority model.Priority) (*model.Ticket, error) {
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	t, err := s.store.UpdateTicketPriority(ctx, orgID, ticketID, priority)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Priority != priority {
		actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
		actorName := ""
		if actor != nil {
			actorName = actor.Name
		}
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "priority", string(old.Priority), string(priority))
	}
	return t, nil
}

func (s *TicketService) UpdatePoints(ctx context.Context, orgID, ticketID, actorID uuid.UUID, points *float64) (*model.Ticket, error) {
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	t, err := s.store.UpdateTicketPoints(ctx, orgID, ticketID, points)
	if err != nil {
		return nil, err
	}
	if old != nil {
		oldVal, newVal := "(none)", "(none)"
		if old.StoryPoints != nil {
			oldVal = fmt.Sprintf("%g", *old.StoryPoints)
		}
		if points != nil {
			newVal = fmt.Sprintf("%g", *points)
		}
		if oldVal != newVal {
			actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
			actorName := ""
			if actor != nil {
				actorName = actor.Name
			}
			_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "story_points", oldVal, newVal)
		}
	}
	return t, nil
}

func (s *TicketService) CloseTicket(ctx context.Context, orgID, ticketID, actorID uuid.UUID, reason string) (*model.Ticket, error) {
	t, err := s.store.CloseTicket(ctx, orgID, ticketID, reason)
	if err != nil {
		return nil, err
	}
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "closed", "", reason)
	return t, nil
}

func (s *TicketService) ReopenTicket(ctx context.Context, orgID, ticketID, actorID uuid.UUID) (*model.Ticket, error) {
	t, err := s.store.ReopenTicket(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "reopened", "", "")
	return t, nil
}

func (s *TicketService) ListAssignees(ctx context.Context, ticketID uuid.UUID) ([]model.User, error) {
	return s.store.ListTicketAssignees(ctx, ticketID)
}

func (s *TicketService) AddAssignee(ctx context.Context, orgID, ticketID, userID, actorID uuid.UUID) error {
	if err := s.store.AddTicketAssignee(ctx, ticketID, userID); err != nil {
		return err
	}
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	added, _ := s.store.GetUserByID(ctx, orgID, userID)
	actorName, addedName := "", ""
	if actor != nil {
		actorName = actor.Name
	}
	if added != nil {
		addedName = added.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "assignee", "", addedName)
	if s.notifications != nil && userID != actorID {
		s.notifications.Notify(ctx, orgID, userID, &ticketID, &actorID, actorName, "assigned")
	}
	return nil
}

func (s *TicketService) RemoveAssignee(ctx context.Context, orgID, ticketID, userID, actorID uuid.UUID) error {
	removed, _ := s.store.GetUserByID(ctx, orgID, userID)
	if err := s.store.RemoveTicketAssignee(ctx, ticketID, userID); err != nil {
		return err
	}
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName, removedName := "", ""
	if actor != nil {
		actorName = actor.Name
	}
	if removed != nil {
		removedName = removed.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "assignee", removedName, "")
	return nil
}

func (s *TicketService) SearchUsers(ctx context.Context, orgID uuid.UUID, q string) ([]model.User, error) {
	return s.store.SearchUsers(ctx, orgID, q)
}

func (s *TicketService) GetUserByUsername(ctx context.Context, orgID uuid.UUID, username string) (*model.User, error) {
	return s.store.GetUserByUsername(ctx, orgID, username)
}

func (s *TicketService) DeleteTicket(ctx context.Context, orgID, ticketID uuid.UUID) error {
	return s.store.DeleteTicket(ctx, orgID, ticketID)
}

func (s *TicketService) Search(ctx context.Context, orgID uuid.UUID, query string) ([]model.Ticket, error) {
	if query == "" {
		return nil, nil
	}
	return s.store.SearchTickets(ctx, orgID, query)
}

func (s *TicketService) UpdateTicketTitle(ctx context.Context, orgID, ticketID, actorID uuid.UUID, title string) (*model.Ticket, error) {
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	t, err := s.store.UpdateTicketTitle(ctx, orgID, ticketID, title)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Title != title {
		actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
		actorName := ""
		if actor != nil {
			actorName = actor.Name
		}
		_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "title", old.Title, title)
	}
	return t, nil
}

func (s *TicketService) UpdateTicketBody(ctx context.Context, orgID, ticketID, actorID uuid.UUID, body string) (*model.Ticket, error) {
	t, err := s.store.UpdateTicketBody(ctx, orgID, ticketID, body)
	if err != nil {
		return nil, err
	}
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	_ = s.store.AppendHistory(ctx, ticketID, actorID, actorName, "description", "(previous)", "(updated)")
	return t, nil
}

func (s *TicketService) UpdateTicketAC(ctx context.Context, orgID, ticketID, actorID uuid.UUID, ac string) (*model.Ticket, error) {
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, ac)
}

// ToggleACCheckbox flips the Nth task-list checkbox in the acceptance_criteria field.
func (s *TicketService) ToggleACCheckbox(ctx context.Context, orgID, ticketID uuid.UUID, index int) (*model.Ticket, error) {
	ac, err := s.store.GetTicketAC(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	ac = toggleNthCheckbox(ac, index)
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, ac)
}

// checkboxRe matches GFM task-list markers: [ ] or [x] (case-insensitive).
var checkboxRe = regexp.MustCompile(`\[([ xX])\]`)

func toggleNthCheckbox(src string, n int) string {
	count := 0
	return checkboxRe.ReplaceAllStringFunc(src, func(match string) string {
		if count == n {
			count++
			if match == "[ ]" {
				return "[x]"
			}
			return "[ ]"
		}
		count++
		return match
	})
}

func (s *TicketService) SearchTicketsForLink(ctx context.Context, orgID, excludeID uuid.UUID, q string) ([]model.Ticket, error) {
	if q == "" {
		return nil, nil
	}
	return s.store.SearchTicketsForLink(ctx, orgID, excludeID, q)
}

func (s *TicketService) SearchTicketsForMention(ctx context.Context, orgID uuid.UUID, q string) ([]model.Ticket, error) {
	if q == "" {
		return nil, nil
	}
	return s.store.SearchTicketsForMention(ctx, orgID, q)
}

// ListMyTickets returns all tickets assigned to the given user, grouped by column name.
// Returns a map of columnName → tickets, plus an ordered list of column names for template rendering.
func (s *TicketService) ListMyTickets(ctx context.Context, orgID, userID uuid.UUID) ([]model.Ticket, error) {
	return s.store.ListTicketsByAssignee(ctx, orgID, userID)
}

// ListInboxActivity returns recent history entries on tickets assigned to the given user.
func (s *TicketService) ListInboxActivity(ctx context.Context, orgID, userID uuid.UUID) ([]model.InboxEntry, error) {
	return s.store.ListInboxActivity(ctx, orgID, userID, 50)
}

func (s *TicketService) rebalanceColumn(ctx context.Context, orgID, columnID uuid.UUID) error {
	tickets, err := s.store.ListTicketsByColumn(ctx, orgID, columnID)
	if err != nil {
		return err
	}
	for i, t := range tickets {
		if err := s.store.MoveTicket(ctx, orgID, t.ID, columnID, float64((i+1)*1000)); err != nil {
			return err
		}
	}
	return nil
}
