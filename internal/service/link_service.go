package service

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/store"
	"github.com/google/uuid"
)

type LinkService struct {
	store *store.Store
}

func NewLinkService(st *store.Store) *LinkService {
	return &LinkService{store: st}
}

func (s *LinkService) ListLinks(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.TicketLink, error) {
	return s.store.ListLinks(ctx, orgID, ticketID)
}

func (s *LinkService) CreateLink(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType) (*model.TicketLink, error) {
	return s.store.CreateLink(ctx, orgID, fromTicketID, toTicketID, relation)
}

func (s *LinkService) DeleteLink(ctx context.Context, orgID, linkID uuid.UUID) error {
	return s.store.DeleteLink(ctx, orgID, linkID)
}
