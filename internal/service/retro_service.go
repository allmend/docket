package service

import (
	"context"
	"fmt"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type RetroService struct {
	store   *store.Store
	tickets *TicketService
	boards  *BoardService
}

func NewRetroService(st *store.Store, tickets *TicketService, boards *BoardService) *RetroService {
	return &RetroService{store: st, tickets: tickets, boards: boards}
}

func (s *RetroService) GetOrCreateRetroBoard(ctx context.Context, orgID, boardID uuid.UUID, sprintID *uuid.UUID) (*model.RetroBoard, error) {
	return s.store.GetOrCreateRetroBoard(ctx, orgID, boardID, sprintID)
}

func (s *RetroService) GetRetroBoard(ctx context.Context, orgID, retroBoardID uuid.UUID) (*model.RetroBoard, error) {
	return s.store.GetRetroBoard(ctx, orgID, retroBoardID)
}

func (s *RetroService) GetRetroView(ctx context.Context, orgID, boardID uuid.UUID, sprintID *uuid.UUID) (*model.RetroView, error) {
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}

	rb, err := s.store.GetOrCreateRetroBoard(ctx, orgID, boardID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("get retro board: %w", err)
	}

	cards, err := s.store.ListRetroCards(ctx, orgID, rb.ID)
	if err != nil {
		return nil, fmt.Errorf("list retro cards: %w", err)
	}

	var sprint *model.Sprint
	if rb.SprintID != nil {
		sp, _ := s.store.GetSprint(ctx, orgID, *rb.SprintID)
		sprint = sp
	}

	view := &model.RetroView{
		RetroBoard: *rb,
		Board:      *board,
		Sprint:     sprint,
	}
	for _, c := range cards {
		switch c.Column {
		case model.RetroWentWell:
			view.WentWell = append(view.WentWell, c)
		case model.RetroDidntGoWell:
			view.DidntGoWell = append(view.DidntGoWell, c)
		case model.RetroActionItem:
			view.ActionItems = append(view.ActionItems, c)
		}
	}
	return view, nil
}

func (s *RetroService) CreateCard(ctx context.Context, orgID, boardID, retroBoardID, authorID uuid.UUID, column model.RetroColumn, body string) (*model.RetroCard, error) {
	return s.store.CreateRetroCard(ctx, orgID, retroBoardID, authorID, column, body)
}

func (s *RetroService) DeleteCard(ctx context.Context, orgID, cardID, authorID uuid.UUID) error {
	return s.store.DeleteRetroCard(ctx, orgID, cardID, authorID)
}

func (s *RetroService) CloseRetroBoard(ctx context.Context, orgID, retroBoardID, actorID uuid.UUID) error {
	rb, err := s.store.GetRetroBoard(ctx, orgID, retroBoardID)
	if err != nil {
		return fmt.Errorf("get retro board: %w", err)
	}

	board, err := s.store.GetBoard(ctx, orgID, rb.BoardID)
	if err != nil {
		return fmt.Errorf("get board: %w", err)
	}

	if board.TeamID != nil {
		cols, err := s.store.ListColumns(ctx, orgID, board.ID)
		if err == nil && len(cols) > 0 {
			cards, err := s.store.ListRetroCards(ctx, orgID, retroBoardID)
			if err == nil {
				for _, card := range cards {
					if card.Column != model.RetroActionItem || card.TicketID != nil {
						continue
					}
					title := "Action Item: " + card.Body
					ticket, err := s.tickets.CreateTicketInTeam(ctx, orgID, board.ID, cols[0].ID, actorID, *board.TeamID, title, "", model.PriorityNone)
					if err == nil {
						_ = s.store.AssignRetroCardOwner(ctx, orgID, card.ID, actorID, ticket.ID, "")
					}
				}
			}
		}
	}

	return s.store.CloseRetroBoard(ctx, orgID, retroBoardID)
}

func (s *RetroService) ListRetros(ctx context.Context, orgID, boardID uuid.UUID) ([]model.RetroListItem, error) {
	boards, err := s.store.ListRetroBoardsForBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(boards))
	for i, rb := range boards {
		ids[i] = rb.ID
	}
	counts, _ := s.store.BulkCountRetroCards(ctx, orgID, ids)

	var items []model.RetroListItem
	for _, rb := range boards {
		item := model.RetroListItem{RetroBoard: rb, ClosedAt: rb.ClosedAt}
		if rb.SprintID != nil {
			if sp, err := s.store.GetSprint(ctx, orgID, *rb.SprintID); err == nil {
				item.SprintName = sp.Name
				item.Sprint = sp
			}
		}
		if c, ok := counts[rb.ID]; ok {
			item.WentWellCount = c.WentWellCount
			item.DidntGoWellCount = c.DidntGoWellCount
			item.ActionItemCount = c.ActionItemCount
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *RetroService) GetSprintReviewData(ctx context.Context, orgID, boardID, sprintID uuid.UUID) (*model.SprintReviewData, error) {
	sprint, err := s.store.GetSprint(ctx, orgID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("get sprint: %w", err)
	}
	board, err := s.store.GetBoard(ctx, orgID, boardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}
	completed, returning, err := s.store.ListSprintTicketsForReview(ctx, orgID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("list sprint tickets: %w", err)
	}
	return &model.SprintReviewData{
		Sprint:    *sprint,
		Board:     *board,
		Completed: completed,
		Returned:  returning,
	}, nil
}

// AssignActionItemOwner assigns an owner to an action item card and creates a backlog ticket.
func (s *RetroService) StackCard(ctx context.Context, orgID, cardID, parentID uuid.UUID) error {
	if cardID == parentID {
		return fmt.Errorf("cannot stack a card with itself")
	}
	parent, err := s.store.GetRetroCard(ctx, orgID, parentID)
	if err != nil {
		return fmt.Errorf("parent card not found")
	}
	if parent.ParentID != nil {
		return fmt.Errorf("cannot stack onto a child card")
	}
	return s.store.StackRetroCard(ctx, orgID, cardID, parentID)
}

func (s *RetroService) UnstackCard(ctx context.Context, orgID, cardID uuid.UUID) error {
	return s.store.UnstackRetroCard(ctx, orgID, cardID)
}

func (s *RetroService) AssignActionItemOwner(ctx context.Context, orgID, cardID, ownerID, actorID uuid.UUID) (*model.RetroCard, error) {
	card, err := s.store.GetRetroCard(ctx, orgID, cardID)
	if err != nil {
		return nil, fmt.Errorf("get card: %w", err)
	}
	if card.Column != model.RetroActionItem {
		return nil, fmt.Errorf("only action items can have an owner")
	}

	owner, err := s.store.GetUserByID(ctx, orgID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get owner: %w", err)
	}

	// Get the retro board → board → columns to create a backlog ticket.
	rb, err := s.store.GetRetroBoard(ctx, orgID, card.RetroBoardID)
	if err != nil {
		return nil, fmt.Errorf("get retro board: %w", err)
	}
	board, err := s.store.GetBoard(ctx, orgID, rb.BoardID)
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}
	if board.TeamID == nil {
		return nil, fmt.Errorf("board has no team")
	}
	cols, err := s.store.ListColumns(ctx, orgID, board.ID)
	if err != nil || len(cols) == 0 {
		return nil, fmt.Errorf("board has no columns")
	}

	ticket, err := s.tickets.CreateTicketInTeam(ctx, orgID, board.ID, cols[0].ID, actorID, *board.TeamID, card.Body, "", model.PriorityNone)
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	_ = s.tickets.AddAssignee(ctx, orgID, ticket.ID, ownerID, actorID)

	if err := s.store.AssignRetroCardOwner(ctx, orgID, cardID, ownerID, ticket.ID, owner.Name); err != nil {
		return nil, fmt.Errorf("assign owner: %w", err)
	}

	card.OwnerID = &ownerID
	card.OwnerName = owner.Name
	card.TicketID = &ticket.ID
	card.TicketDisplay = ticket.DisplayID()
	return card, nil
}
