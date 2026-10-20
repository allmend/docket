package store

import (
	"context"
	"os"
	"testing"
)

// TestTeamCreatorBackfill runs the backfill migration's SQL against a team
// created the old way (no member rows) and checks the creator is added once,
// and that running it again changes nothing.
func TestTeamCreatorBackfill(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	org := seedOrg(t, "org-backfill")
	user := seedUser(t, org.ID, "alice")
	team, err := s.CreateTeam(ctx, org.ID, user.ID, "Eng", "ENG", "eng", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	sql, err := os.ReadFile("../../migrations/000026_team_creator_membership.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	members := func() int {
		t.Helper()
		var n int
		if err := testPool.QueryRow(ctx,
			`SELECT count(*) FROM team_members WHERE org_id = $1 AND team_id = $2 AND user_id = $3`,
			org.ID, team.ID, user.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := members(); n != 0 {
		t.Fatalf("precondition: store.CreateTeam alone adds no members, got %d", n)
	}
	for i := 0; i < 2; i++ {
		if _, err := testPool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("run backfill (pass %d): %v", i+1, err)
		}
		if n := members(); n != 1 {
			t.Fatalf("after pass %d: creator membership rows = %d, want 1", i+1, n)
		}
	}
}
