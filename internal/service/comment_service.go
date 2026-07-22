package service

import (
	"context"
	"errors"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

// ErrForbidden is returned when the actor may not perform the requested action.
var ErrForbidden = errors.New("forbidden")

type CommentService struct {
	store *store.Store
}

func NewCommentService(st *store.Store) *CommentService {
	return &CommentService{store: st}
}

func (s *CommentService) GetComment(ctx context.Context, orgID, commentID uuid.UUID) (*model.Comment, error) {
	return s.store.GetComment(ctx, orgID, commentID)
}

func (s *CommentService) ListComments(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.Comment, error) {
	return s.store.ListComments(ctx, orgID, ticketID)
}

func (s *CommentService) CreateComment(ctx context.Context, orgID, ticketID, authorID uuid.UUID, body string) (*model.Comment, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return nil, err
	}
	return s.store.CreateComment(ctx, orgID, ticketID, authorID, body)
}

func (s *CommentService) UpdateComment(ctx context.Context, orgID, commentID, actorID uuid.UUID, actorRole, body string) (*model.Comment, error) {
	if err := s.authorizeCommentMutation(ctx, orgID, commentID, actorID, actorRole); err != nil {
		return nil, err
	}
	return s.store.UpdateComment(ctx, orgID, commentID, body)
}

func (s *CommentService) DeleteComment(ctx context.Context, orgID, commentID, actorID uuid.UUID, actorRole string) error {
	if err := s.authorizeCommentMutation(ctx, orgID, commentID, actorID, actorRole); err != nil {
		return err
	}
	return s.store.DeleteComment(ctx, orgID, commentID)
}

// authorizeCommentMutation permits editing/deleting a comment only for its
// author or an org admin, and only while the ticket is open. Returns ErrForbidden
// or ErrTicketClosed otherwise (or the store error if the comment can't be loaded).
func (s *CommentService) authorizeCommentMutation(ctx context.Context, orgID, commentID, actorID uuid.UUID, actorRole string) error {
	c, err := s.store.GetComment(ctx, orgID, commentID)
	if err != nil {
		return err
	}
	if c.AuthorID != actorID && actorRole != "admin" {
		return ErrForbidden
	}
	return assertTicketOpen(ctx, s.store, orgID, c.TicketID)
}

func (s *CommentService) ListHistory(ctx context.Context, ticketID uuid.UUID) ([]model.HistoryEntry, error) {
	return s.store.ListHistory(ctx, ticketID)
}

func (s *CommentService) AppendHistory(ctx context.Context, ticketID, actorID uuid.UUID, actorName, field, oldValue, newValue string) error {
	return s.store.AppendHistory(ctx, ticketID, actorID, actorName, field, oldValue, newValue)
}
