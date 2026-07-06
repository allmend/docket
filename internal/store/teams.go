package store

import (
	"context"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

const teamCols = `id, org_id, name, key, slug, description, ticket_counter, sprint_capacity, created_by, created_at, updated_at`

func scanTeam(row interface{ Scan(...any) error }, t *model.Team) error {
	return row.Scan(&t.ID, &t.OrgID, &t.Name, &t.Key, &t.Slug, &t.Description, &t.TicketCounter, &t.SprintCapacity, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
}

func (s *Store) ListTeams(ctx context.Context, orgID uuid.UUID) ([]model.Team, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+teamCols+` FROM teams WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []model.Team
	for rows.Next() {
		var t model.Team
		if err := scanTeam(rows, &t); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *Store) GetTeam(ctx context.Context, orgID, teamID uuid.UUID) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.replica.QueryRow(ctx,
		`SELECT `+teamCols+` FROM teams WHERE org_id = $1 AND id = $2`,
		orgID, teamID,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTeamBySlug looks up a team by its URL slug, scoped to the org.
func (s *Store) GetTeamBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.replica.QueryRow(ctx,
		`SELECT `+teamCols+` FROM teams WHERE org_id = $1 AND slug = $2`,
		orgID, slug,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTeamByKey looks up a team by its short key (e.g. "ENG"), scoped to the org.
func (s *Store) GetTeamByKey(ctx context.Context, orgID uuid.UUID, key string) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.replica.QueryRow(ctx,
		`SELECT `+teamCols+` FROM teams WHERE org_id = $1 AND key = $2`,
		orgID, key,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateTeam(ctx context.Context, orgID, createdBy uuid.UUID, name, key, slug, description string) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.primary.QueryRow(ctx,
		`INSERT INTO teams (org_id, name, key, slug, description, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+teamCols,
		orgID, name, key, slug, description, createdBy,
	), &t)
	return &t, err
}

func (s *Store) UpdateTeam(ctx context.Context, orgID, teamID uuid.UUID, name, slug, description string) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.primary.QueryRow(ctx,
		`UPDATE teams SET name = $3, slug = $4, description = $5, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+teamCols,
		orgID, teamID, name, slug, description,
	), &t)
	return &t, err
}

func (s *Store) UpdateTeamCapacity(ctx context.Context, orgID, teamID uuid.UUID, capacity int) (*model.Team, error) {
	var t model.Team
	err := scanTeam(s.primary.QueryRow(ctx,
		`UPDATE teams SET sprint_capacity = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+teamCols,
		orgID, teamID, capacity,
	), &t)
	return &t, err
}

func (s *Store) DeleteTeam(ctx context.Context, orgID, teamID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM teams WHERE org_id = $1 AND id = $2`,
		orgID, teamID,
	)
	return err
}

// NextTicketNumber atomically increments the team counter and returns the new value.
func (s *Store) NextTicketNumber(ctx context.Context, teamID uuid.UUID) (int, error) {
	var n int
	err := s.primary.QueryRow(ctx,
		`UPDATE teams SET ticket_counter = ticket_counter + 1 WHERE id = $1 RETURNING ticket_counter`,
		teamID,
	).Scan(&n)
	return n, err
}

// GetTicketByTeamRef resolves a ticket from its human-readable reference (team key + number).
// ticketJoins already left-joins teams as "tm", so we filter on that alias directly.
func (s *Store) GetTicketByTeamRef(ctx context.Context, orgID uuid.UUID, teamKey string, number int) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.replica.QueryRow(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE tm.org_id = $1 AND tm.key = $2 AND t.number = $3`,
		orgID, teamKey, number,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// --- Team members ---

func (s *Store) ListTeamMembers(ctx context.Context, orgID, teamID uuid.UUID) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT u.id, u.org_id, u.name, u.email, u.created_at, u.updated_at
		 FROM team_members tm
		 JOIN users u ON u.id = tm.user_id
		 WHERE tm.org_id = $1 AND tm.team_id = $2
		 ORDER BY u.name`,
		orgID, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) AddTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO team_members (org_id, team_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		orgID, teamID, userID,
	)
	return err
}

func (s *Store) RemoveTeamMember(ctx context.Context, orgID, teamID, userID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM team_members WHERE org_id = $1 AND team_id = $2 AND user_id = $3`,
		orgID, teamID, userID,
	)
	return err
}

// SearchNonMembers returns users in the org that are NOT already members of the team.
func (s *Store) SearchNonMembers(ctx context.Context, orgID, teamID uuid.UUID, q string) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT u.id, u.org_id, u.name, u.email, u.created_at, u.updated_at
		 FROM users u
		 WHERE u.org_id = $1
		   AND (u.name ILIKE '%' || $3 || '%' OR u.email ILIKE '%' || $3 || '%')
		   AND u.id NOT IN (
		       SELECT user_id FROM team_members WHERE org_id = $1 AND team_id = $2
		   )
		 ORDER BY u.name
		 LIMIT 20`,
		orgID, teamID, q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListTeamsWithBoards returns all teams joined with their board and active sprint (if any), for nav rendering.
func (s *Store) ListTeamsWithBoards(ctx context.Context, orgID uuid.UUID) ([]model.TeamWithBoard, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT
		     t.id, t.org_id, t.name, t.key, t.slug, t.description, t.ticket_counter, t.created_by, t.created_at, t.updated_at,
		     b.id, b.org_id, b.team_id, b.name, b.description, b.mode, b.created_by, b.created_at, b.updated_at,
		     s.id, s.org_id, s.board_id, s.name, s.goal, s.status, s.start_date, s.end_date,
		     s.committed_tickets, s.completed_tickets, s.committed_points, s.completed_points,
		     s.created_by, s.created_at, s.updated_at
		 FROM teams t
		 LEFT JOIN boards b ON b.team_id = t.id
		 LEFT JOIN sprints s ON s.board_id = b.id AND s.status = 'active'
		 WHERE t.org_id = $1
		 ORDER BY t.name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.TeamWithBoard
	for rows.Next() {
		var team model.Team
		// Board columns — nullable due to LEFT JOIN.
		var bID, bOrgID, bTeamID *uuid.UUID
		var bName, bDesc, bMode *string
		var bCreatedBy *uuid.UUID
		var bCreatedAt, bUpdatedAt *time.Time
		// Sprint columns — nullable due to LEFT JOIN.
		var sID, sOrgID, sBoardID, sCreatedBy *uuid.UUID
		var sName, sGoal, sStatus *string
		var sStartDate, sEndDate *time.Time
		var sCommittedTickets, sCompletedTickets *int
		var sCommittedPoints, sCompletedPoints *float64
		var sCreatedAt, sUpdatedAt *time.Time

		err := rows.Scan(
			&team.ID, &team.OrgID, &team.Name, &team.Key, &team.Slug, &team.Description,
			&team.TicketCounter, &team.CreatedBy, &team.CreatedAt, &team.UpdatedAt,
			&bID, &bOrgID, &bTeamID, &bName, &bDesc, &bMode, &bCreatedBy, &bCreatedAt, &bUpdatedAt,
			&sID, &sOrgID, &sBoardID, &sName, &sGoal, &sStatus, &sStartDate, &sEndDate,
			&sCommittedTickets, &sCompletedTickets, &sCommittedPoints, &sCompletedPoints,
			&sCreatedBy, &sCreatedAt, &sUpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		twb := model.TeamWithBoard{Team: team}
		if bID != nil {
			board := model.Board{
				ID:          *bID,
				OrgID:       *bOrgID,
				TeamID:      bTeamID,
				Name:        *bName,
				Description: *bDesc,
				Mode:        model.BoardMode(*bMode),
				CreatedBy:   *bCreatedBy,
				CreatedAt:   *bCreatedAt,
				UpdatedAt:   *bUpdatedAt,
			}
			twb.Board = &board
		}
		if sID != nil {
			sprint := model.Sprint{
				ID:               *sID,
				OrgID:            *sOrgID,
				BoardID:          *sBoardID,
				Name:             *sName,
				Goal:             *sGoal,
				Status:           model.SprintStatus(*sStatus),
				StartDate:        sStartDate,
				EndDate:          sEndDate,
				CommittedTickets: *sCommittedTickets,
				CompletedTickets: *sCompletedTickets,
				CommittedPoints:  *sCommittedPoints,
				CompletedPoints:  *sCompletedPoints,
				CreatedBy:        *sCreatedBy,
				CreatedAt:        *sCreatedAt,
				UpdatedAt:        *sUpdatedAt,
			}
			twb.ActiveSprint = &sprint
		}
		result = append(result, twb)
	}
	return result, rows.Err()
}

// CountOrgUsers returns the total number of users in the org.
func (s *Store) CountOrgUsers(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := s.replica.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE org_id = $1`,
		orgID,
	).Scan(&n)
	return n, err
}

// ListTeamMembersForOrg returns up to 4 members per team across all teams in the org,
// returned as a map of teamID → []User for efficient lookup.
func (s *Store) ListTeamMembersForOrg(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID][]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT tm.team_id, u.id, u.org_id, u.name, u.email, u.created_at, u.updated_at
		 FROM (
		     SELECT team_id, user_id,
		            ROW_NUMBER() OVER (PARTITION BY team_id ORDER BY joined_at) AS rn
		     FROM team_members
		     WHERE org_id = $1
		 ) tm
		 JOIN users u ON u.id = tm.user_id
		 WHERE tm.rn <= 4
		 ORDER BY tm.team_id, u.name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]model.User)
	for rows.Next() {
		var teamID uuid.UUID
		var u model.User
		if err := rows.Scan(&teamID, &u.ID, &u.OrgID, &u.Name, &u.Email, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result[teamID] = append(result[teamID], u)
	}
	return result, rows.Err()
}
