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

// CreateLink creates a ticket link and records a history entry on both tickets,
// each labelled from that ticket's perspective.
func (s *LinkService) CreateLink(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType, actorID uuid.UUID) (*model.TicketLink, error) {
	link, err := s.store.CreateLink(ctx, orgID, fromTicketID, toTicketID, relation)
	if err != nil {
		return nil, err
	}

	actorName := s.actorName(ctx, orgID, actorID)
	_ = s.store.AppendHistory(ctx, fromTicketID, actorID, actorName, "link_added", "",
		s.linkLabel(ctx, orgID, fromTicketID, toTicketID, relation, fromTicketID))
	_ = s.store.AppendHistory(ctx, toTicketID, actorID, actorName, "link_added", "",
		s.linkLabel(ctx, orgID, fromTicketID, toTicketID, relation, toTicketID))

	return link, nil
}

// DeleteLink deletes a ticket link and records a history entry on both tickets.
func (s *LinkService) DeleteLink(ctx context.Context, orgID, linkID, actorID uuid.UUID) error {
	link, _ := s.store.GetLink(ctx, orgID, linkID)

	if err := s.store.DeleteLink(ctx, orgID, linkID); err != nil {
		return err
	}

	if link != nil {
		actorName := s.actorName(ctx, orgID, actorID)
		_ = s.store.AppendHistory(ctx, link.FromTicketID, actorID, actorName, "link_removed",
			s.linkLabel(ctx, orgID, link.FromTicketID, link.ToTicketID, link.Relation, link.FromTicketID), "")
		_ = s.store.AppendHistory(ctx, link.ToTicketID, actorID, actorName, "link_removed",
			s.linkLabel(ctx, orgID, link.FromTicketID, link.ToTicketID, link.Relation, link.ToTicketID), "")
	}

	return nil
}

func (s *LinkService) actorName(ctx context.Context, orgID, actorID uuid.UUID) string {
	if actor, _ := s.store.GetUserByID(ctx, orgID, actorID); actor != nil {
		return actor.Name
	}
	return ""
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
		switch relation {
		case model.RelationBlocks:
			verb = "blocked by"
		case model.RelationDependsOn:
			verb = "needed by"
		case model.RelationDuplicates:
			verb = "duplicated by"
		default:
			verb = strings.ReplaceAll(string(relation), "_", " ")
		}
	}

	other, _ := s.store.GetTicket(ctx, orgID, otherID)
	if other != nil {
		return verb + " " + other.DisplayID()
	}
	return verb
}
