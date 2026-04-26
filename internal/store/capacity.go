package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// GetSprintCapacity returns all capacity rows for a sprint, joined with user info.
// Only returns members that have a capacity row (call SeedSprintCapacity first).
func (s *Store) GetSprintCapacity(ctx context.Context, orgID, sprintID uuid.UUID) ([]model.SprintCapacityMember, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT sc.user_id, u.name, u.username, sc.focus_pct
		FROM sprint_capacity sc
		JOIN users u ON u.id = sc.user_id
		WHERE sc.org_id = $1 AND sc.sprint_id = $2
		ORDER BY u.name
	`, orgID, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.SprintCapacityMember
	for rows.Next() {
		var m model.SprintCapacityMember
		if err := rows.Scan(&m.UserID, &m.Name, &m.Username, &m.FocusPct); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SeedSprintCapacity inserts a 100% row for every team member not already present.
// Safe to call multiple times (ON CONFLICT DO NOTHING).
func (s *Store) SeedSprintCapacity(ctx context.Context, orgID, sprintID, teamID uuid.UUID) error {
	_, err := s.primary.Exec(ctx, `
		INSERT INTO sprint_capacity (sprint_id, user_id, org_id)
		SELECT $1, tm.user_id, $2
		FROM team_members tm
		WHERE tm.org_id = $2 AND tm.team_id = $3
		ON CONFLICT (sprint_id, user_id) DO NOTHING
	`, sprintID, orgID, teamID)
	return err
}

// UpsertSprintCapacity sets focus_pct for one member in a sprint.
func (s *Store) UpsertSprintCapacity(ctx context.Context, orgID, sprintID, userID uuid.UUID, focusPct int) error {
	_, err := s.primary.Exec(ctx, `
		INSERT INTO sprint_capacity (sprint_id, user_id, org_id, focus_pct, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (sprint_id, user_id) DO UPDATE
		  SET focus_pct = EXCLUDED.focus_pct,
		      updated_at = NOW()
	`, sprintID, userID, orgID, focusPct)
	return err
}
