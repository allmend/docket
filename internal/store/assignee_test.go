package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestAddTicketAssignee_OrgIsolation locks in the fix for the cross-tenant
// assignee write: AddTicketAssignee must only link a ticket and user that both
// belong to the given org. A caller presenting another org's id (or a ticket /
// user from another org) must not mutate anything.
func TestAddTicketAssignee_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	ticketA := seedTicket(t, orgA.ID, userA.ID, "A ticket")

	assignees := func() int {
		t.Helper()
		list, err := s.ListTicketAssignees(ctx, ticketA.ID)
		if err != nil {
			t.Fatalf("list assignees: %v", err)
		}
		return len(list)
	}

	// Wrong org id for an in-org ticket+user: no write.
	if err := s.AddTicketAssignee(ctx, orgB.ID, ticketA.ID, userA.ID); err != nil {
		t.Fatalf("add (wrong org) returned error: %v", err)
	}
	if n := assignees(); n != 0 {
		t.Fatalf("cross-org add leaked: got %d assignees, want 0", n)
	}

	// Right org, but the assignee belongs to another org: no write.
	if err := s.AddTicketAssignee(ctx, orgA.ID, ticketA.ID, userB.ID); err != nil {
		t.Fatalf("add (foreign user) returned error: %v", err)
	}
	if n := assignees(); n != 0 {
		t.Fatalf("foreign-user add leaked: got %d assignees, want 0", n)
	}

	// Fully in-org: the write succeeds.
	if err := s.AddTicketAssignee(ctx, orgA.ID, ticketA.ID, userA.ID); err != nil {
		t.Fatalf("add (in org) returned error: %v", err)
	}
	if n := assignees(); n != 1 {
		t.Fatalf("in-org add failed: got %d assignees, want 1", n)
	}
}

// TestRemoveTicketAssignee_OrgIsolation asserts an out-of-org caller cannot
// strip an assignee off a ticket it does not own.
func TestRemoveTicketAssignee_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticketA := seedTicket(t, orgA.ID, userA.ID, "A ticket")

	if err := s.AddTicketAssignee(ctx, orgA.ID, ticketA.ID, userA.ID); err != nil {
		t.Fatalf("setup add: %v", err)
	}

	// Attacker in org B tries to remove org A's assignee: no-op.
	if err := s.RemoveTicketAssignee(ctx, orgB.ID, ticketA.ID, userA.ID); err != nil {
		t.Fatalf("remove (wrong org) returned error: %v", err)
	}
	list, err := s.ListTicketAssignees(ctx, ticketA.ID)
	if err != nil {
		t.Fatalf("list assignees: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("cross-org remove succeeded: got %d assignees, want 1", len(list))
	}

	// Owner removes: succeeds.
	if err := s.RemoveTicketAssignee(ctx, orgA.ID, ticketA.ID, userA.ID); err != nil {
		t.Fatalf("remove (in org) returned error: %v", err)
	}
	list, _ = s.ListTicketAssignees(ctx, ticketA.ID)
	if len(list) != 0 {
		t.Fatalf("in-org remove failed: got %d assignees, want 0", len(list))
	}
}

// TestGetTicket_OrgIsolation is a canary for the core multi-tenancy rule:
// loading a ticket by id with the wrong org must fail, not leak.
func TestGetTicket_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticketA := seedTicket(t, orgA.ID, userA.ID, "A ticket")

	if _, err := s.GetTicket(ctx, orgA.ID, ticketA.ID); err != nil {
		t.Fatalf("owner GetTicket failed: %v", err)
	}
	if _, err := s.GetTicket(ctx, orgB.ID, ticketA.ID); err == nil {
		t.Fatal("cross-org GetTicket succeeded — org isolation breach")
	}
	// A random org id must also miss.
	if _, err := s.GetTicket(ctx, uuid.New(), ticketA.ID); err == nil {
		t.Fatal("unknown-org GetTicket succeeded — org isolation breach")
	}
}
