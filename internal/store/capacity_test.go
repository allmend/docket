package store

import (
	"context"
	"testing"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// seedSprint creates a board and a planning sprint on it for the given org.
func seedSprint(t *testing.T, orgID, createdBy uuid.UUID) *model.Sprint {
	t.Helper()
	ctx := context.Background()
	board, err := testStore.CreateBoard(ctx, orgID, createdBy, nil, "Board", "", model.BoardModeScrum)
	if err != nil {
		t.Fatalf("seed board: %v", err)
	}
	sprint, err := testStore.CreateSprint(ctx, orgID, board.ID, createdBy, "Sprint 1", "", nil, nil)
	if err != nil {
		t.Fatalf("seed sprint: %v", err)
	}
	return sprint
}

// capacityFocus reads the focus_pct row directly (bypassing any team_members
// merge in GetSprintCapacity) so the test asserts exactly what was persisted.
func capacityFocus(t *testing.T, sprintID, userID uuid.UUID) (int, bool) {
	t.Helper()
	var pct int
	err := testPool.QueryRow(context.Background(),
		`SELECT focus_pct FROM sprint_capacity WHERE sprint_id = $1 AND user_id = $2`,
		sprintID, userID).Scan(&pct)
	if err != nil {
		return 0, false
	}
	return pct, true
}

// TestUpsertSprintCapacity_OrgIsolation locks in the org guard: a caller
// presenting another org's id must not create or overwrite capacity for a
// sprint it does not own.
func TestUpsertSprintCapacity_OrgIsolation(t *testing.T) {
	s := requireStore(t)
	resetDB(t)
	ctx := context.Background()

	orgA := seedOrg(t, "org-a")
	orgB := seedOrg(t, "org-b")
	userA := seedUser(t, orgA.ID, "alice")
	sprintA := seedSprint(t, orgA.ID, userA.ID)

	// Wrong org for org A's sprint: no write.
	if err := s.UpsertSprintCapacity(ctx, orgB.ID, sprintA.ID, userA.ID, 50); err != nil {
		t.Fatalf("cross-org upsert errored: %v", err)
	}
	if _, ok := capacityFocus(t, sprintA.ID, userA.ID); ok {
		t.Fatal("cross-org upsert wrote a capacity row — org isolation breach")
	}

	// Owner writes: succeeds.
	if err := s.UpsertSprintCapacity(ctx, orgA.ID, sprintA.ID, userA.ID, 50); err != nil {
		t.Fatalf("in-org upsert errored: %v", err)
	}
	if pct, ok := capacityFocus(t, sprintA.ID, userA.ID); !ok || pct != 50 {
		t.Fatalf("in-org upsert: got (%d, %v), want (50, true)", pct, ok)
	}

	// Attacker in org B tries to overwrite the existing row: must not change it.
	if err := s.UpsertSprintCapacity(ctx, orgB.ID, sprintA.ID, userA.ID, 99); err != nil {
		t.Fatalf("cross-org overwrite errored: %v", err)
	}
	if pct, _ := capacityFocus(t, sprintA.ID, userA.ID); pct != 50 {
		t.Fatalf("cross-org overwrite changed focus_pct to %d, want 50", pct)
	}
}
