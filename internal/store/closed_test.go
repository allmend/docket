package store

import (
	"context"
	"testing"
)

// TestIsTicketClosed verifies the closed-state probe that gates content mutations,
// including its org scoping.
func TestIsTicketClosed(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	other := seedOrg(t, "org-b")
	user := seedUser(t, org.ID, "alice")
	ticket := seedTicket(t, org.ID, user.ID, "work")

	closed, err := s.IsTicketClosed(ctx, org.ID, ticket.ID)
	if err != nil {
		t.Fatalf("probe open ticket: %v", err)
	}
	if closed {
		t.Fatal("freshly created ticket reported closed")
	}

	if _, err := s.CloseTicket(ctx, org.ID, ticket.ID, "done"); err != nil {
		t.Fatalf("close ticket: %v", err)
	}
	closed, err = s.IsTicketClosed(ctx, org.ID, ticket.ID)
	if err != nil {
		t.Fatalf("probe closed ticket: %v", err)
	}
	if !closed {
		t.Fatal("closed ticket reported open")
	}

	if _, err := s.ReopenTicket(ctx, org.ID, ticket.ID); err != nil {
		t.Fatalf("reopen ticket: %v", err)
	}
	closed, err = s.IsTicketClosed(ctx, org.ID, ticket.ID)
	if err != nil {
		t.Fatalf("probe reopened ticket: %v", err)
	}
	if closed {
		t.Fatal("reopened ticket still reported closed")
	}

	// Cross-org lookup must not resolve the ticket at all.
	if _, err := s.IsTicketClosed(ctx, other.ID, ticket.ID); err == nil {
		t.Fatal("cross-org probe resolved a ticket from another org")
	}
}
