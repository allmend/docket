package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrTicketClosed is returned when a content mutation targets a closed ticket.
// A closed ticket is immutable: title, body, acceptance criteria, priority, story
// points, assignees, labels, comments, links and DoD checks are all frozen.
//
// Movement is deliberately NOT covered — dragging a ticket out of a Done column
// reopens it (see TicketService.MoveTicket / MoveToColumn), so moves, sprint
// assignment, reopen, close and delete stay legal on a closed ticket.
var ErrTicketClosed = errors.New("ticket is closed")

// ticketCloseChecker is the slice of the store assertTicketOpen needs.
// *store.Store satisfies it; tests supply a fake.
type ticketCloseChecker interface {
	IsTicketClosed(ctx context.Context, orgID, ticketID uuid.UUID) (bool, error)
}

// assertTicketOpen returns ErrTicketClosed if the ticket is closed. Shared by every
// service that mutates ticket content — each holds its own *store.Store.
func assertTicketOpen(ctx context.Context, st ticketCloseChecker, orgID, ticketID uuid.UUID) error {
	closed, err := st.IsTicketClosed(ctx, orgID, ticketID)
	if err != nil {
		return err
	}
	if closed {
		return ErrTicketClosed
	}
	return nil
}
