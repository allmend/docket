package service

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

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
	return s.store.CreateComment(ctx, orgID, ticketID, authorID, body)
}

func (s *CommentService) UpdateComment(ctx context.Context, orgID, commentID uuid.UUID, body string) (*model.Comment, error) {
	return s.store.UpdateComment(ctx, orgID, commentID, body)
}

func (s *CommentService) DeleteComment(ctx context.Context, orgID, commentID uuid.UUID) error {
	return s.store.DeleteComment(ctx, orgID, commentID)
}

func (s *CommentService) ListHistory(ctx context.Context, ticketID uuid.UUID) ([]model.HistoryEntry, error) {
	return s.store.ListHistory(ctx, ticketID)
}

func (s *CommentService) AppendHistory(ctx context.Context, ticketID, actorID uuid.UUID, actorName, field, oldValue, newValue string) error {
	return s.store.AppendHistory(ctx, ticketID, actorID, actorName, field, oldValue, newValue)
}
