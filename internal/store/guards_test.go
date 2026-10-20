package store

import (
	"context"
	"errors"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// These cover ownership guards on writes that took an ID from the request and
// used it unchecked — from another org, or within the org from the wrong
// board. Nothing failed before the guards went in; each gap was found by
// reading, which is why every one gets pinned here.

func TestAssignTicketToSprint_Guards(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	ticket := seedTicket(t, orgA.ID, userA.ID, "ticket")

	own, err := s.CreateSprint(ctx, orgA.ID, ticket.BoardID, userA.ID, "own", "", nil, nil)
	if err != nil {
		t.Fatalf("own sprint: %v", err)
	}
	completed, err := s.CreateSprint(ctx, orgA.ID, ticket.BoardID, userA.ID, "finished", "", nil, nil)
	if err != nil {
		t.Fatalf("completed sprint: %v", err)
	}
	if _, err := s.SetSprintStatus(ctx, orgA.ID, completed.ID, model.SprintStatusCompleted); err != nil {
		t.Fatalf("complete sprint: %v", err)
	}

	rejected := []struct {
		name     string
		sprintID uuid.UUID
	}{
		{"sprint on another board", seedSprint(t, orgA.ID, userA.ID).ID},
		{"sprint in another org", seedSprint(t, orgB.ID, userB.ID).ID},
		{"completed sprint", completed.ID},
	}
	for _, tc := range rejected {
		id := tc.sprintID
		if err := s.AssignTicketToSprint(ctx, orgA.ID, ticket.ID, &id); !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("%s: got %v, want pgx.ErrNoRows", tc.name, err)
		}
	}
	if got, _ := s.GetTicket(ctx, orgA.ID, ticket.ID); got.SprintID != nil {
		t.Fatalf("a rejected assignment still set sprint_id to %v", *got.SprintID)
	}

	if err := s.AssignTicketToSprint(ctx, orgA.ID, ticket.ID, &own.ID); err != nil {
		t.Fatalf("assign to own sprint: %v", err)
	}
	if got, _ := s.GetTicket(ctx, orgA.ID, ticket.ID); got.SprintID == nil || *got.SprintID != own.ID {
		t.Fatalf("assign to own sprint did not stick: %v", got.SprintID)
	}
}

func TestMoveTicket_RejectsColumnOnAnotherBoard(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	ticket := seedTicket(t, org.ID, user.ID, "moving")
	elsewhere := seedTicket(t, org.ID, user.ID, "on another board")

	if err := s.MoveTicket(ctx, org.ID, ticket.ID, elsewhere.ColumnID, 2000); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("move to another board's column: got %v, want pgx.ErrNoRows", err)
	}
	if got, _ := s.GetTicket(ctx, org.ID, ticket.ID); got.ColumnID != ticket.ColumnID {
		t.Fatal("rejected move changed the ticket's column")
	}

	doing, err := s.CreateColumn(ctx, org.ID, ticket.BoardID, "Doing", 2000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}
	if err := s.MoveTicket(ctx, org.ID, ticket.ID, doing.ID, 1000); err != nil {
		t.Fatalf("move within own board: %v", err)
	}
	if got, _ := s.GetTicket(ctx, org.ID, ticket.ID); got.ColumnID != doing.ID {
		t.Fatal("move within own board did not stick")
	}
}

func TestBoardChildInserts_RejectForeignBoard(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	boardB := seedTicket(t, orgB.ID, userB.ID, "b").BoardID

	if _, err := s.CreateColumn(ctx, orgA.ID, boardB, "Sneaky", 5000); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CreateColumn: got %v, want pgx.ErrNoRows", err)
	}
	if _, err := s.CreateSprint(ctx, orgA.ID, boardB, userA.ID, "Sneaky", "", nil, nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CreateSprint: got %v, want pgx.ErrNoRows", err)
	}
	if _, err := s.CreateDodItem(ctx, orgA.ID, boardB, "Sneaky"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CreateDodItem: got %v, want pgx.ErrNoRows", err)
	}
	if _, err := s.CreateTag(ctx, orgA.ID, boardB, "Sneaky", "#000000", "", nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("CreateTag: got %v, want pgx.ErrNoRows", err)
	}
}

func TestAddTeamMember_RejectsForeignUser(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	userB := seedUser(t, orgB.ID, "bob")
	team, err := s.CreateTeam(ctx, orgA.ID, userA.ID, "Team", "TM", "team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	if err := s.AddTeamMember(ctx, orgA.ID, team.ID, userB.ID); err != nil {
		t.Fatalf("add foreign user returned error: %v", err)
	}
	if members, _ := s.ListTeamMembers(ctx, orgA.ID, team.ID); len(members) != 0 {
		t.Fatalf("another org's user was listed as a member: %+v", members)
	}

	if err := s.AddTeamMember(ctx, orgA.ID, team.ID, userA.ID); err != nil {
		t.Fatalf("add own user: %v", err)
	}
	if members, _ := s.ListTeamMembers(ctx, orgA.ID, team.ID); len(members) != 1 {
		t.Fatalf("own user not added: got %d members", len(members))
	}
}

func TestDeleteLink_OnlyFromItsOwnEnds(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	from := seedTicket(t, org.ID, user.ID, "from")
	to := seedTicket(t, org.ID, user.ID, "to")
	bystander := seedTicket(t, org.ID, user.ID, "bystander")

	link, _, err := s.CreateLink(ctx, org.ID, from.ID, to.ID, model.RelationRelatesTo)
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := s.DeleteLink(ctx, org.ID, bystander.ID, link.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete via an unrelated ticket: got %v, want pgx.ErrNoRows", err)
	}
	if _, err := s.GetLink(ctx, org.ID, link.ID); err != nil {
		t.Fatalf("link gone after a rejected delete: %v", err)
	}
	if err := s.DeleteLink(ctx, org.ID, to.ID, link.ID); err != nil {
		t.Fatalf("delete from the target end: %v", err)
	}
}

func TestRetroCards_StackingAndClosedBoard(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	rb, err := s.GetOrCreateRetroBoard(ctx, org.ID, seedTicket(t, org.ID, user.ID, "x").BoardID, nil)
	if err != nil {
		t.Fatalf("retro board: %v", err)
	}
	card := func(rbID uuid.UUID, body string) *model.RetroCard {
		t.Helper()
		c, err := s.CreateRetroCard(ctx, org.ID, rbID, user.ID, model.RetroWentWell, body)
		if err != nil {
			t.Fatalf("create card %q: %v", body, err)
		}
		return c
	}
	a, b, c := card(rb.ID, "a"), card(rb.ID, "b"), card(rb.ID, "c")

	if err := s.StackRetroCard(ctx, org.ID, b.ID, a.ID); err != nil {
		t.Fatalf("stack b onto a: %v", err)
	}
	// Stacking a (which now has a child) onto c must carry b along; a child of
	// a child would drop out of ListRetroCards.
	if err := s.StackRetroCard(ctx, org.ID, a.ID, c.ID); err != nil {
		t.Fatalf("stack a onto c: %v", err)
	}
	roots, err := s.ListRetroCards(ctx, org.ID, rb.ID)
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != c.ID || len(roots[0].Children) != 2 {
		t.Fatalf("want one root c with two children, got %d roots: %+v", len(roots), roots)
	}

	other, err := s.GetOrCreateRetroBoard(ctx, org.ID, seedTicket(t, org.ID, user.ID, "y").BoardID, nil)
	if err != nil {
		t.Fatalf("second retro board: %v", err)
	}
	if err := s.StackRetroCard(ctx, org.ID, card(other.ID, "d").ID, c.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stack across retro boards: got %v, want pgx.ErrNoRows", err)
	}

	if err := s.CloseRetroBoard(ctx, org.ID, rb.ID); err != nil {
		t.Fatalf("close retro: %v", err)
	}
	if _, err := s.CreateRetroCard(ctx, org.ID, rb.ID, user.ID, model.RetroActionItem, "late"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("card on a closed retro: got %v, want pgx.ErrNoRows", err)
	}
}

func TestUpdateUserRole_KeepsOneAdmin(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	newAdmin := func(orgID uuid.UUID, username string) *model.User {
		t.Helper()
		u, err := s.CreateUser(ctx, orgID, username, username, username+"@example.com", "admin")
		if err != nil {
			t.Fatalf("create admin %q: %v", username, err)
		}
		return u
	}
	root := newAdmin(orgA.ID, "root")
	newAdmin(orgB.ID, "elsewhere") // an admin in another org must not count

	if err := s.UpdateUserRole(ctx, orgA.ID, root.ID, "member"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("demote the only admin: got %v, want pgx.ErrNoRows", err)
	}
	if u, _ := s.GetUserByID(ctx, orgA.ID, root.ID); u.Role != "admin" {
		t.Fatalf("rejected demotion changed role to %q", u.Role)
	}

	second := newAdmin(orgA.ID, "second")
	if err := s.UpdateUserRole(ctx, orgA.ID, root.ID, "member"); err != nil {
		t.Fatalf("demote with another admin present: %v", err)
	}
	if err := s.UpdateUserRole(ctx, orgA.ID, second.ID, "member"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("demote the now-last admin: got %v, want pgx.ErrNoRows", err)
	}
	if err := s.UpdateUserRole(ctx, orgA.ID, root.ID, "admin"); err != nil {
		t.Fatalf("promotion is always allowed: %v", err)
	}
}

func TestToggleDodCheck_RequiresItemOnTicketsBoard(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	ticket := seedTicket(t, org.ID, user.ID, "ticket")
	elsewhere := seedTicket(t, org.ID, user.ID, "elsewhere")

	own, err := s.CreateDodItem(ctx, org.ID, ticket.BoardID, "tested")
	if err != nil {
		t.Fatalf("own item: %v", err)
	}
	foreign, err := s.CreateDodItem(ctx, org.ID, elsewhere.BoardID, "reviewed")
	if err != nil {
		t.Fatalf("foreign item: %v", err)
	}

	checks := func() int {
		t.Helper()
		var n int
		if err := testPool.QueryRow(ctx, `SELECT COUNT(*) FROM dod_checks WHERE ticket_id = $1`, ticket.ID).Scan(&n); err != nil {
			t.Fatalf("count checks: %v", err)
		}
		return n
	}

	if err := s.ToggleDodCheck(ctx, org.ID, ticket.ID, foreign.ID, true); err != nil {
		t.Fatalf("toggle foreign item: %v", err)
	}
	if n := checks(); n != 0 {
		t.Fatalf("an item from another board was checked on the ticket: %d rows", n)
	}
	if err := s.ToggleDodCheck(ctx, org.ID, ticket.ID, own.ID, true); err != nil {
		t.Fatalf("toggle own item: %v", err)
	}
	if n := checks(); n != 1 {
		t.Fatalf("own item not checked: %d rows", n)
	}
}

func TestTicketSubresourceReads_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	ticket := seedTicket(t, orgA.ID, userA.ID, "ticket")

	if err := s.AppendHistory(ctx, orgA.ID, ticket.ID, userA.ID, "alice", "title", "old", "new"); err != nil {
		t.Fatalf("append history: %v", err)
	}
	if err := s.AddTicketAssignee(ctx, orgA.ID, ticket.ID, userA.ID); err != nil {
		t.Fatalf("add assignee: %v", err)
	}

	if h, _ := s.ListHistory(ctx, orgB.ID, ticket.ID); len(h) != 0 {
		t.Errorf("history leaked across orgs: %d entries", len(h))
	}
	if a, _ := s.ListTicketAssignees(ctx, orgB.ID, ticket.ID); len(a) != 0 {
		t.Errorf("assignees leaked across orgs: %d users", len(a))
	}
	if h, _ := s.ListHistory(ctx, orgA.ID, ticket.ID); len(h) != 1 {
		t.Errorf("own history: got %d entries, want 1", len(h))
	}
	if a, _ := s.ListTicketAssignees(ctx, orgA.ID, ticket.ID); len(a) != 1 {
		t.Errorf("own assignees: got %d users, want 1", len(a))
	}
}

func TestInTx_RollsBackOnError(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	boardID := seedTicket(t, org.ID, user.ID, "ticket").BoardID
	hasColumn := func(st *Store, name string) bool {
		t.Helper()
		cols, err := st.ListColumns(ctx, org.ID, boardID)
		if err != nil {
			t.Fatalf("list columns: %v", err)
		}
		for _, c := range cols {
			if c.Name == name {
				return true
			}
		}
		return false
	}

	boom := errors.New("boom")
	err := s.InTx(ctx, func(tx *Store) error {
		if _, err := tx.CreateColumn(ctx, org.ID, boardID, "Doomed", 9000); err != nil {
			return err
		}
		if !hasColumn(tx, "Doomed") {
			t.Error("a write is not visible to reads inside its own transaction")
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx returned %v, want the callback's error", err)
	}
	if hasColumn(s, "Doomed") {
		t.Fatal("write survived a rolled-back transaction")
	}

	if err := s.InTx(ctx, func(tx *Store) error {
		_, err := tx.CreateColumn(ctx, org.ID, boardID, "Kept", 9000)
		return err
	}); err != nil {
		t.Fatalf("committing transaction: %v", err)
	}
	if !hasColumn(s, "Kept") {
		t.Fatal("write lost from a committed transaction")
	}
}

func TestCompleteSprint_OnlyFromActive(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")
	sprint := seedSprint(t, org.ID, user.ID) // planning

	if err := s.CompleteSprint(ctx, org.ID, sprint.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("complete a planning sprint: got %v, want pgx.ErrNoRows", err)
	}
	if _, err := s.SetSprintStatus(ctx, org.ID, sprint.ID, model.SprintStatusActive); err != nil {
		t.Fatalf("start sprint: %v", err)
	}
	if err := s.CompleteSprint(ctx, org.ID, sprint.ID); err != nil {
		t.Fatalf("complete the active sprint: %v", err)
	}
	// A second close — the loser of a race — must match nothing.
	if err := s.CompleteSprint(ctx, org.ID, sprint.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("complete twice: got %v, want pgx.ErrNoRows", err)
	}
	if got, _ := s.GetSprint(ctx, org.ID, sprint.ID); got.Status != model.SprintStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
}
