package store

import (
	"context"
	"errors"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/jackc/pgx/v5"
)

// TestDeleteTeamCascades pins that deleting a workspace takes its board and
// everything hanging off its tickets with it. tickets.team_id has no ON DELETE
// action of its own; the delete only works because the tickets go first via
// boards → tickets, so a schema change there would turn this into an FK error.
func TestDeleteTeamCascades(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-a")
	user := seedUser(t, org.ID, "alice")

	team, err := s.CreateTeam(ctx, org.ID, user.ID, "Eng", "ENG", "eng", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	board, err := s.CreateBoard(ctx, org.ID, user.ID, &team.ID, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	col, err := s.CreateColumn(ctx, org.ID, board.ID, "To Do", 1000)
	if err != nil {
		t.Fatalf("create column: %v", err)
	}
	a, err := s.CreateTicket(ctx, org.ID, board.ID, col.ID, user.ID, &team.ID, 1, "a", "", model.PriorityMedium, 1000)
	if err != nil {
		t.Fatalf("create ticket a: %v", err)
	}
	b, err := s.CreateTicket(ctx, org.ID, board.ID, col.ID, user.ID, &team.ID, 2, "b", "", model.PriorityMedium, 2000)
	if err != nil {
		t.Fatalf("create ticket b: %v", err)
	}
	if _, err := s.CreateComment(ctx, org.ID, a.ID, user.ID, "hi"); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if err := s.AppendHistory(ctx, a.ID, user.ID, "alice", "title", "", "a"); err != nil {
		t.Fatalf("append history: %v", err)
	}
	if _, err := s.CreateLink(ctx, org.ID, a.ID, b.ID, model.RelationBlocks); err != nil {
		t.Fatalf("create link: %v", err)
	}

	if err := s.DeleteTeam(ctx, org.ID, team.ID); err != nil {
		t.Fatalf("delete team: %v", err)
	}

	if _, err := s.GetTeam(ctx, org.ID, team.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("team still readable after delete: err=%v", err)
	}
	if _, err := s.GetTicket(ctx, org.ID, a.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("ticket survived its workspace: err=%v", err)
	}
}

// TestDeleteTeamOrgIsolation pins that a foreign org's ID deletes nothing.
func TestDeleteTeamOrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	user := seedUser(t, orgA.ID, "alice")

	team, err := s.CreateTeam(ctx, orgA.ID, user.ID, "Eng", "ENG", "eng", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := s.DeleteTeam(ctx, orgB.ID, team.ID); err != nil {
		t.Fatalf("delete team from org B: %v", err)
	}
	if _, err := s.GetTeam(ctx, orgA.ID, team.ID); err != nil {
		t.Errorf("org B deleted org A's team: %v", err)
	}
}
