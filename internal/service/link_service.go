package service

import (
	"context"
	"strings"

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

// CreateLink creates a ticket link and records a history entry on the viewed ticket.
// viewTicketID is the ticket the user was on when they added the link.
func (s *LinkService) CreateLink(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType, actorID, viewTicketID uuid.UUID) (*model.TicketLink, error) {
	link, err := s.store.CreateLink(ctx, orgID, fromTicketID, toTicketID, relation)
	if err != nil {
		return nil, err
	}

	label := s.linkLabel(ctx, orgID, fromTicketID, toTicketID, relation, viewTicketID)
	actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
	actorName := ""
	if actor != nil {
		actorName = actor.Name
	}
	_ = s.store.AppendHistory(ctx, viewTicketID, actorID, actorName, "link_added", "", label)

	return link, nil
}

// DeleteLink deletes a ticket link and records a history entry on the viewed ticket.
func (s *LinkService) DeleteLink(ctx context.Context, orgID, linkID, viewTicketID, actorID uuid.UUID) error {
	link, _ := s.store.GetLink(ctx, orgID, linkID)

	if err := s.store.DeleteLink(ctx, orgID, linkID); err != nil {
		return err
	}

	if link != nil {
		label := s.linkLabel(ctx, orgID, link.FromTicketID, link.ToTicketID, link.Relation, viewTicketID)
		actor, _ := s.store.GetUserByID(ctx, orgID, actorID)
		actorName := ""
		if actor != nil {
			actorName = actor.Name
		}
		_ = s.store.AppendHistory(ctx, viewTicketID, actorID, actorName, "link_removed", label, "")
	}

	return nil
}

// linkLabel returns a human-readable label for a link from the perspective of viewTicketID.
func (s *LinkService) linkLabel(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType, viewTicketID uuid.UUID) string {
	var otherID uuid.UUID
	var verb string

	if fromTicketID == viewTicketID {
		otherID = toTicketID
		switch relation {
		case model.RelationBlocks:
			verb = "blocks"
		case model.RelationDependsOn:
			verb = "depends on"
		case model.RelationDuplicates:
			verb = "duplicates"
		default:
			verb = strings.ReplaceAll(string(relation), "_", " ")
		}
	} else {
		otherID = fromTicketID
		verb = "blocked by"
	}

	other, _ := s.store.GetTicket(ctx, orgID, otherID)
	if other != nil {
		return verb + " " + other.DisplayID()
	}
	return verb
}
