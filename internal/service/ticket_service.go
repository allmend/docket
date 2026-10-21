package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/allmend/docket/internal/metrics"
	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TicketService struct {
	store         *store.Store
	notifications *NotificationService
}

func NewTicketService(st *store.Store, notifications *NotificationService) *TicketService {
	return &TicketService{store: st, notifications: notifications}
}

// actorName resolves the display name recorded in ticket history. Empty when the
// actor can't be found — history is still written, just unattributed.
func (s *TicketService) actorName(ctx context.Context, orgID, actorID uuid.UUID) string {
	if actor, _ := s.store.GetUserByID(ctx, orgID, actorID); actor != nil {
		return actor.Name
	}
	return ""
}

func (s *TicketService) GetTicket(ctx context.Context, orgID, ticketID uuid.UUID) (*model.Ticket, error) {
	return s.store.GetTicket(ctx, orgID, ticketID)
}

// GetByRef resolves a ticket by its human-readable reference (e.g. "ENG", 42 → ENG-42).
func (s *TicketService) GetByRef(ctx context.Context, orgID uuid.UUID, teamKey string, number int) (*model.Ticket, error) {
	return s.store.GetTicketByTeamRef(ctx, orgID, teamKey, number)
}

// GetByRefWithTags is GetByRef with tracks populated. The API hands out tickets
// as standalone documents and has no board view to bulk-load them from.
func (s *TicketService) GetByRefWithTags(ctx context.Context, orgID uuid.UUID, teamKey string, number int) (*model.Ticket, error) {
	ticket, err := s.GetByRef(ctx, orgID, teamKey, number)
	if err != nil {
		return nil, err
	}
	if err := s.LoadTags(ctx, orgID, ticket); err != nil {
		return nil, err
	}
	one := []model.Ticket{*ticket}
	if err := s.markLinks(ctx, orgID, one); err != nil {
		return nil, err
	}
	*ticket = one[0]
	links, err := s.store.ListLinks(ctx, orgID, ticket.ID)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	ticket.Links = links
	return ticket, nil
}

// markLinks sets the blocked / waiting markers the board shows, with one pair
// of bulk queries per board the tickets are on (a team has one board today).
func (s *TicketService) markLinks(ctx context.Context, orgID uuid.UUID, tickets []model.Ticket) error {
	done := map[uuid.UUID]bool{}
	for _, t := range tickets {
		if done[t.BoardID] {
			continue
		}
		done[t.BoardID] = true
		blockedBy, err := s.store.BulkGetBlockedBy(ctx, orgID, t.BoardID)
		if err != nil {
			return fmt.Errorf("bulk blocked: %w", err)
		}
		waitingOn, err := s.store.BulkGetWaitingOn(ctx, orgID, t.BoardID)
		if err != nil {
			return fmt.Errorf("bulk waiting: %w", err)
		}
		for i := range tickets {
			if tickets[i].BoardID != t.BoardID {
				continue
			}
			if b, ok := blockedBy[tickets[i].ID]; ok {
				tickets[i].IsBlocked, tickets[i].BlockedBy = true, b
			}
			if d, ok := waitingOn[tickets[i].ID]; ok {
				tickets[i].IsWaiting, tickets[i].WaitingOn = true, d
			}
		}
	}
	return nil
}

// LoadTags populates Tags on a single ticket.
func (s *TicketService) LoadTags(ctx context.Context, orgID uuid.UUID, ticket *model.Ticket) error {
	tags, err := s.store.ListTicketTags(ctx, orgID, ticket.ID)
	if err != nil {
		return fmt.Errorf("list ticket tags: %w", err)
	}
	ticket.Tags = tags
	return nil
}

// ListByTeam returns a team's tickets ordered by number, tracks included.
// See GetByRefWithTags for why the API needs them inline.
func (s *TicketService) ListByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.Ticket, error) {
	tickets, err := s.store.ListTicketsByTeam(ctx, orgID, teamID)
	if err != nil {
		return nil, err
	}
	tagsByTicket, err := s.store.BulkListTicketTagsByTeam(ctx, orgID, teamID)
	if err != nil {
		return nil, fmt.Errorf("bulk list ticket tags: %w", err)
	}
	for i := range tickets {
		tickets[i].Tags = tagsByTicket[tickets[i].ID]
	}
	if err := s.markLinks(ctx, orgID, tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

// CreateTicketInTeam creates a ticket with an explicit team (used by the public API
// where the team is resolved from the URL key before calling this).
func (s *TicketService) CreateTicketInTeam(ctx context.Context,
	orgID, boardID, columnID, createdBy, teamID uuid.UUID,
	title, body string, priority model.Priority,
) (*model.Ticket, error) {
	if !priority.Valid() {
		return nil, fmt.Errorf("%w: unknown priority %q", ErrInvalidInput, priority)
	}
	number, err := s.store.NextTicketNumber(ctx, orgID, teamID)
	if err != nil {
		return nil, fmt.Errorf("next ticket number: %w", err)
	}

	maxPos, err := s.store.MaxTicketPositionInColumn(ctx, orgID, columnID)
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
	if !priority.Valid() {
		return nil, fmt.Errorf("%w: unknown priority %q", ErrInvalidInput, priority)
	}
	// Resolve team from board so we can assign a sequential ticket number.
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}

	var number int
	if board.TeamID != nil {
		number, err = s.store.NextTicketNumber(ctx, orgID, *board.TeamID)
		if err != nil {
			return nil, fmt.Errorf("next ticket number: %w", err)
		}
	}

	maxPos, err := s.store.MaxTicketPositionInColumn(ctx, orgID, columnID)
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
	if !priority.Valid() {
		return nil, fmt.Errorf("%w: unknown priority %q", ErrInvalidInput, priority)
	}
	old, err := s.store.GetTicket(ctx, orgID, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	if old.ClosedAt != nil {
		return nil, ErrTicketClosed
	}
	// A nil assignee means "not supplied", not "unassign": the UI edit form has
	// no assignee field, so treating nil as a clear wiped the assignee on every
	// title/body edit. A supplied one must be a user of this org.
	if assigneeID == nil {
		assigneeID = old.AssigneeID
	} else if _, err := s.store.GetUserByID(ctx, orgID, *assigneeID); err != nil {
		return nil, fmt.Errorf("%w: unknown assignee", ErrInvalidInput)
	}

	updated, err := s.store.UpdateTicket(ctx, orgID, ticketID, title, body, priority, assigneeID)
	if err != nil {
		return nil, err
	}

	actorName := s.actorName(ctx, orgID, actorID)

	for _, c := range ticketFieldChanges(old, updated) {
		_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, c.Field, c.Old, c.New)
	}

	return updated, nil
}

// MoveTicket repositions a ticket using fractional indexing.
// prevPos and nextPos are the positions of the adjacent cards (0 means boundary);
// nextID is the ticket below the drop, when the client knows it, and is only
// used to find the drop slot again if the list has to be rebalanced.
func (s *TicketService) MoveTicket(ctx context.Context,
	orgID, ticketID, targetColumnID, actorID uuid.UUID,
	prevPos, nextPos float64, nextID uuid.UUID,
) (float64, error) {
	old, err := s.store.GetTicket(ctx, orgID, ticketID)
	if err != nil {
		return 0, err
	}
	// A nil target keeps the current column (backlog reordering). Anything else
	// must be a column on the ticket's own board — checked before the rebalance
	// below, which would otherwise renumber some other board's column first.
	var newCol *model.Column
	if targetColumnID != uuid.Nil {
		newCol, err = s.store.GetColumn(ctx, orgID, targetColumnID)
		if err != nil {
			return 0, err
		}
		if newCol.BoardID != old.BoardID {
			return 0, fmt.Errorf("%w: column is on a different board", ErrInvalidInput)
		}
	}

	// Capture from-column info before the move for the transition counter.
	var fromCol, fromTeam string
	if old.ColumnID != uuid.Nil {
		fromCol, fromTeam, _ = s.store.GetColumnMeta(ctx, orgID, old.ColumnID)
	}

	newPos, needsRebalance := nextPosition(prevPos, nextPos)
	if !needsRebalance {
		if err := s.store.MoveTicket(ctx, orgID, ticketID, targetColumnID, newPos); err != nil {
			return 0, err
		}
	} else {
		// Renumber the list the user dropped into and compute the slot from the
		// renumbered neighbours. The client's prevPos/nextPos are meaningless
		// after the renumber, so recomputing via nextPosition would demand a
		// rebalance again, forever — the slot must come from fresh positions.
		// One transaction, so a concurrent drop can't interleave with it.
		err := s.store.InTx(ctx, func(tx *store.Store) error {
			list, err := rebalanceScope(ctx, tx, orgID, old, targetColumnID)
			if err != nil {
				return fmt.Errorf("rebalance: %w", err)
			}
			newPos = rebalanceDropSlot(list, ticketID, nextID, nextPos)
			if err := tx.RenumberTicketPositions(ctx, orgID, ticketIDs(list)); err != nil {
				return fmt.Errorf("rebalance: %w", err)
			}
			return tx.MoveTicket(ctx, orgID, ticketID, targetColumnID, newPos)
		})
		if err != nil {
			return 0, err
		}
	}

	if newCol != nil {
		s.applyColumnTransition(ctx, orgID, ticketID, actorID,
			decideColumnTransition(fromCol, newCol.Name, old.ClosedAt != nil))

		// Drag is how most tickets move, so it must leave the same trail as the
		// status dropdown (MoveToColumn) — ticket_history is the audit log.
		if c, ok := columnMoveChange(old.ColumnID, fromCol, newCol); ok {
			_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, s.actorName(ctx, orgID, actorID), c.Field, c.Old, c.New)
		}

		toCol, toTeam, _ := s.store.GetColumnMeta(ctx, orgID, targetColumnID)
		team := fromTeam
		if team == "" {
			team = toTeam
		}
		metrics.TicketTransitions.WithLabelValues(orgID.String(), team, fromCol, toCol).Inc()
	}

	return newPos, nil
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
	if newCol.BoardID != old.BoardID {
		return nil, fmt.Errorf("%w: column is on a different board", ErrInvalidInput)
	}

	maxPos, _ := s.store.MaxTicketPositionInColumn(ctx, orgID, columnID)
	if err := s.store.MoveTicket(ctx, orgID, ticketID, columnID, maxPos+1000); err != nil {
		return nil, err
	}

	oldName := ""
	if oldCol != nil {
		oldName = oldCol.Name
	}
	s.applyColumnTransition(ctx, orgID, ticketID, actorID,
		decideColumnTransition(oldName, newCol.Name, old.ClosedAt != nil))

	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "column", oldName, newCol.Name)

	return s.store.GetTicket(ctx, orgID, ticketID)
}

// applyColumnTransition closes or reopens a ticket after a column move, with
// the same history entries as the explicit Close and Reopen actions.
func (s *TicketService) applyColumnTransition(ctx context.Context, orgID, ticketID, actorID uuid.UUID, tr columnTransition) {
	switch tr {
	case transitionClose:
		if _, err := s.store.CloseTicket(ctx, orgID, ticketID, "done"); err != nil {
			return
		}
		_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, s.actorName(ctx, orgID, actorID), "closed", "", "done")
		s.unblockOnClose(ctx, orgID, ticketID, actorID)
	case transitionReopen:
		if _, err := s.store.ReopenTicket(ctx, orgID, ticketID); err != nil {
			return
		}
		_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, s.actorName(ctx, orgID, actorID), "reopened", "", "")
	}
}

// rebalanceScope is the list a drop renumbers: the one the user is looking at,
// in position order. Renumbering any other set would interleave its new
// positions with tickets the view also shows and scramble the order.
//   - a ticket in a sprint: that sprint's tickets in the target column (the
//     Sprint Board column, and the per-column order of the planning list);
//   - a backlog ticket on a Scrum board: the whole backlog, every column;
//   - anything else (Kanban, Blank): the target column.
func rebalanceScope(ctx context.Context, st *store.Store, orgID uuid.UUID, t *model.Ticket, targetColumnID uuid.UUID) ([]model.Ticket, error) {
	col := targetColumnID
	if col == uuid.Nil {
		col = t.ColumnID
	}
	if t.SprintID != nil {
		all, err := st.ListSprintTickets(ctx, orgID, *t.SprintID)
		if err != nil {
			return nil, err
		}
		var inCol []model.Ticket
		for _, x := range all {
			if x.ColumnID == col {
				inCol = append(inCol, x)
			}
		}
		return inCol, nil
	}
	board, err := st.GetBoard(ctx, orgID, t.BoardID)
	if err != nil {
		return nil, err
	}
	if board.Mode.IsScrum() {
		return st.ListBacklogTickets(ctx, orgID, t.BoardID)
	}
	return st.ListTicketsByColumn(ctx, orgID, col)
}

func (s *TicketService) UpdatePriority(ctx context.Context, orgID, ticketID, actorID uuid.UUID, priority model.Priority) (*model.Ticket, error) {
	if !priority.Valid() {
		return nil, fmt.Errorf("%w: unknown priority %q", ErrInvalidInput, priority)
	}
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	if old != nil && old.ClosedAt != nil {
		return nil, ErrTicketClosed
	}
	t, err := s.store.UpdateTicketPriority(ctx, orgID, ticketID, priority)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Priority != priority {
		actorName := s.actorName(ctx, orgID, actorID)
		_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "priority", string(old.Priority), string(priority))
	}
	return t, nil
}

// UpdatePoints sets a ticket's estimate. Tickets in the active sprint are
// committed work and are not re-estimated mid-sprint: changing their points
// needs override, which the Backlog page's active-sprint section sends after a
// warning, and is recorded as such in the history.
func (s *TicketService) UpdatePoints(ctx context.Context, orgID, ticketID, actorID uuid.UUID, points *float64, override bool) (*model.Ticket, error) {
	if points != nil && (*points < 0 || math.IsNaN(*points) || math.IsInf(*points, 0)) {
		return nil, fmt.Errorf("%w: story points must be a non-negative number", ErrInvalidInput)
	}
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	if old != nil && old.ClosedAt != nil {
		return nil, ErrTicketClosed
	}
	inActiveSprint := false
	if old != nil && old.SprintID != nil {
		if sp, err := s.store.GetSprint(ctx, orgID, *old.SprintID); err == nil {
			inActiveSprint = sp.Status.IsActive()
		}
	}
	if inActiveSprint && !override && !samePoints(old.StoryPoints, points) {
		return nil, fmt.Errorf("%w: the ticket is in the active sprint; its points are locked unless overridden", ErrInvalidInput)
	}
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
			field := "story_points"
			if inActiveSprint {
				field = "story_points_override"
			}
			actorName := s.actorName(ctx, orgID, actorID)
			_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, field, oldVal, newVal)
		}
	}
	return t, nil
}

func samePoints(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// CloseTicket closes a ticket with one of the picker's reasons (see
// model.CloseReasonOptions); "other" takes a free-text note instead.
func (s *TicketService) CloseTicket(ctx context.Context, orgID, ticketID, actorID uuid.UUID, reason, note string) (*model.Ticket, error) {
	reason, ok := model.ParseCloseReason(reason, note)
	if !ok {
		return nil, fmt.Errorf("%w: choose a close reason (\"other\" needs a note of at most %d characters)", ErrInvalidInput, model.MaxCloseNoteLen)
	}
	t, err := s.store.CloseTicket(ctx, orgID, ticketID, reason)
	if err != nil {
		return nil, err
	}
	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "closed", "", reason)
	s.unblockOnClose(ctx, orgID, ticketID, actorID)
	return t, nil
}

// unblockOnClose clears outbound "blocks" links when a ticket closes, so the
// tickets it was blocking become unblocked. Each cleared link is recorded as
// history on both ends before deletion.
func (s *TicketService) unblockOnClose(ctx context.Context, orgID, ticketID, actorID uuid.UUID) {
	links, err := s.store.ListBlockingLinksFromTicket(ctx, orgID, ticketID)
	if err != nil || len(links) == 0 {
		return
	}
	actorName := s.actorName(ctx, orgID, actorID)
	for _, l := range links {
		_ = s.store.AppendHistory(ctx, orgID, l.ToTicketID, actorID, actorName, "link_cleared", "blocked by "+l.FromDisplayID, "")
		_ = s.store.AppendHistory(ctx, orgID, l.FromTicketID, actorID, actorName, "link_cleared", "blocks "+l.ToDisplayID, "")
	}
	_ = s.store.DeleteBlockingLinksFromTicket(ctx, orgID, ticketID)
}

func (s *TicketService) ReopenTicket(ctx context.Context, orgID, ticketID, actorID uuid.UUID) (*model.Ticket, error) {
	t, err := s.store.ReopenTicket(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "reopened", "", "")
	return t, nil
}

func (s *TicketService) ListAssignees(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.User, error) {
	return s.store.ListTicketAssignees(ctx, orgID, ticketID)
}

func (s *TicketService) AddAssignee(ctx context.Context, orgID, ticketID, userID, actorID uuid.UUID) error {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return err
	}
	// Resolved up front: the store insert silently skips a user from another
	// org, and history plus a notification would still be written for them.
	added, err := s.store.GetUserByID(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("%w: unknown user", ErrInvalidInput)
	}
	if err := s.store.AddTicketAssignee(ctx, orgID, ticketID, userID); err != nil {
		return err
	}
	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "assignee", "", added.Name)
	if s.notifications != nil && userID != actorID {
		s.notifications.Notify(ctx, orgID, userID, &ticketID, &actorID, actorName, "assigned")
	}
	return nil
}

func (s *TicketService) RemoveAssignee(ctx context.Context, orgID, ticketID, userID, actorID uuid.UUID) error {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return err
	}
	removed, _ := s.store.GetUserByID(ctx, orgID, userID)
	if err := s.store.RemoveTicketAssignee(ctx, orgID, ticketID, userID); err != nil {
		return err
	}
	removedName := ""
	if removed != nil {
		removedName = removed.Name
	}
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, s.actorName(ctx, orgID, actorID), "assignee", removedName, "")
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

// SimilarOpen returns up to three open tickets resembling a title being typed,
// so a likely duplicate shows before it is filed. Very short titles match too
// loosely to be useful and return nothing.
func (s *TicketService) SimilarOpen(ctx context.Context, orgID uuid.UUID, title string) ([]model.Ticket, error) {
	title = strings.TrimSpace(title)
	if len([]rune(title)) < 4 {
		return nil, nil
	}
	return s.store.SimilarOpenTickets(ctx, orgID, title, 3)
}

func (s *TicketService) Search(ctx context.Context, orgID uuid.UUID, query string) ([]model.Ticket, error) {
	if query == "" {
		return nil, nil
	}
	return s.store.SearchTickets(ctx, orgID, query)
}

func (s *TicketService) UpdateTicketTitle(ctx context.Context, orgID, ticketID, actorID uuid.UUID, title string) (*model.Ticket, error) {
	old, _ := s.store.GetTicket(ctx, orgID, ticketID)
	if old != nil && old.ClosedAt != nil {
		return nil, ErrTicketClosed
	}
	t, err := s.store.UpdateTicketTitle(ctx, orgID, ticketID, title)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Title != title {
		actorName := s.actorName(ctx, orgID, actorID)
		_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "title", old.Title, title)
	}
	return t, nil
}

// UpdateTicketBody saves a ticket description. baseVersion is the
// model.BodyVersion the editor started from; when someone else saved a
// different description since, it returns ErrEditConflict and writes nothing.
// An empty baseVersion saves unconditionally.
func (s *TicketService) UpdateTicketBody(ctx context.Context, orgID, ticketID, actorID uuid.UUID, body, baseVersion string) (*model.Ticket, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	t, err := s.store.UpdateTicketBody(ctx, orgID, ticketID, body, baseVersion)
	if errors.Is(err, pgx.ErrNoRows) && baseVersion != "" {
		// The ticket exists (assertTicketOpen found it), so the guard refused.
		return nil, ErrEditConflict
	}
	if err != nil {
		return nil, err
	}
	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, orgID, ticketID, actorID, actorName, "description", "(previous)", "(updated)")
	return t, nil
}

func (s *TicketService) UpdateTicketAC(ctx context.Context, orgID, ticketID, actorID uuid.UUID, ac string) (*model.Ticket, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, ac)
}

// ToggleACCheckbox flips the Nth task-list checkbox in the acceptance_criteria field.
func (s *TicketService) ToggleACCheckbox(ctx context.Context, orgID, ticketID uuid.UUID, index int) (*model.Ticket, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	ac, err := s.store.GetTicketAC(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	ac = toggleNthCheckbox(ac, index)
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, ac)
}

// AddACItem appends a new unchecked item to the acceptance criteria.
func (s *TicketService) AddACItem(ctx context.Context, orgID, ticketID uuid.UUID, text string) (*model.Ticket, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	ac, err := s.store.GetTicketAC(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, acAppendItem(ac, text))
}

// DeleteACItem removes the Nth checklist item from the acceptance criteria.
func (s *TicketService) DeleteACItem(ctx context.Context, orgID, ticketID uuid.UUID, index int) (*model.Ticket, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	ac, err := s.store.GetTicketAC(ctx, orgID, ticketID)
	if err != nil {
		return nil, err
	}
	return s.store.UpdateTicketAC(ctx, orgID, ticketID, acDeleteItem(ac, index))
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

// ListActivityByActor returns history entries where the user was the actor in the last 24h.
func (s *TicketService) ListActivityByActor(ctx context.Context, orgID, actorID uuid.UUID) ([]model.InboxEntry, error) {
	return s.store.ListActivityByActor(ctx, orgID, actorID)
}

// ListBlockedTickets returns open blocked tickets org-wide (any sprint or backlog).
func (s *TicketService) ListBlockedTickets(ctx context.Context, orgID uuid.UUID) ([]model.DashboardBlockedTicket, error) {
	return s.store.ListBlockedTickets(ctx, orgID)
}

// ListMyOpenTickets returns all open tickets assigned to the user, grouped by sprint/backlog.
func (s *TicketService) ListMyOpenTickets(ctx context.Context, orgID, userID uuid.UUID) ([]model.DashboardMyTicket, error) {
	return s.store.ListMyOpenTickets(ctx, orgID, userID)
}

// ListRecentOrgActivity returns recent ticket history across the whole org.
func (s *TicketService) ListRecentOrgActivity(ctx context.Context, orgID uuid.UUID) ([]model.InboxEntry, error) {
	return s.store.ListRecentOrgActivity(ctx, orgID, 15)
}

// ListBlockedTicketsByTeam returns blocked tickets scoped to a single team.
func (s *TicketService) ListBlockedTicketsByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.DashboardBlockedTicket, error) {
	return s.store.ListBlockedTicketsByTeam(ctx, orgID, teamID)
}

// ListMyOpenTicketsByTeam returns my open tickets scoped to a single team.
func (s *TicketService) ListMyOpenTicketsByTeam(ctx context.Context, orgID, userID, teamID uuid.UUID) ([]model.DashboardMyTicket, error) {
	return s.store.ListMyOpenTicketsByTeam(ctx, orgID, userID, teamID)
}

// ListRecentTeamActivity returns recent ticket history for a single team.
func (s *TicketService) ListRecentTeamActivity(ctx context.Context, orgID, teamID uuid.UUID) ([]model.InboxEntry, error) {
	return s.store.ListRecentTeamActivity(ctx, orgID, teamID, 12)
}
