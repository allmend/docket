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
//
// viewTicketID is the ticket the actor is looking at, which is not always the
// stored source: an inverse phrasing like "Blocked by ENG-11" stores the link the
// other way round. The immutability guard applies to the viewed ticket, so a link
// pointing at a closed ticket can still be added from the open end. DeleteLink
// works the same way.
func (s *LinkService) CreateLink(ctx context.Context, orgID, viewTicketID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType, actorID uuid.UUID) (*model.TicketLink, error) {
	if err := assertTicketOpen(ctx, s.store, orgID, viewTicketID); err != nil {
		return nil, err
	}
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
// ticketID is the ticket the actor is viewing — the link is only removable while
// that ticket is open, so a link to a closed ticket can still be cleared from the
// open end.
func (s *LinkService) DeleteLink(ctx context.Context, orgID, ticketID, linkID, actorID uuid.UUID) error {
	if err := assertTicketOpen(ctx, s.store, orgID, ticketID); err != nil {
		return err
	}
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

// linkLabel describes a link from viewTicketID's side: "blocks ENG-11" on one
// ticket, "blocked by ENG-4" on the other. The words come from the model so
// history and the UI never drift apart.
func (s *LinkService) linkLabel(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType, viewTicketID uuid.UUID) string {
	otherID := toTicketID
	if fromTicketID != viewTicketID {
		otherID = fromTicketID
		relation = relation.Inverse()
	}
	verb := strings.ToLower(relation.Label())

	other, _ := s.store.GetTicket(ctx, orgID, otherID)
	if other != nil {
		return verb + " " + other.DisplayID()
	}
	return verb
}
