package store

import (
	"context"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

const boardCols = `id, org_id, team_id, name, description, mode, created_by, created_at, updated_at`

func scanBoard(row interface{ Scan(dest ...any) error }, b *model.Board) error {
	return row.Scan(&b.ID, &b.OrgID, &b.TeamID, &b.Name, &b.Description, &b.Mode, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
}

func (s *Store) ListBoards(ctx context.Context, orgID uuid.UUID) ([]model.Board, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+boardCols+` FROM boards WHERE org_id = $1 ORDER BY name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []model.Board
	for rows.Next() {
		var b model.Board
		if err := scanBoard(rows, &b); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *Store) ListBoardsByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.Board, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+boardCols+` FROM boards WHERE org_id = $1 AND team_id = $2 ORDER BY name`,
		orgID, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []model.Board
	for rows.Next() {
		var b model.Board
		if err := scanBoard(rows, &b); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func (s *Store) GetBoard(ctx context.Context, orgID, boardID uuid.UUID) (*model.Board, error) {
	var b model.Board
	err := scanBoard(s.replica.QueryRow(ctx,
		`SELECT `+boardCols+` FROM boards WHERE org_id = $1 AND id = $2`,
		orgID, boardID,
	), &b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) CreateBoard(ctx context.Context, orgID, createdBy uuid.UUID, teamID *uuid.UUID, name, description string, mode model.BoardMode) (*model.Board, error) {
	var b model.Board
	err := scanBoard(s.primary.QueryRow(ctx,
		`INSERT INTO boards (org_id, team_id, name, description, mode, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+boardCols,
		orgID, teamID, name, description, mode, createdBy,
	), &b)
	return &b, err
}

// GetBoardByTeamID returns the single board belonging to a team.
func (s *Store) GetBoardByTeamID(ctx context.Context, orgID, teamID uuid.UUID) (*model.Board, error) {
	var b model.Board
	err := scanBoard(s.replica.QueryRow(ctx,
		`SELECT `+boardCols+` FROM boards WHERE org_id = $1 AND team_id = $2 LIMIT 1`,
		orgID, teamID,
	), &b)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Store) UpdateBoard(ctx context.Context, orgID, boardID uuid.UUID, name, description string) (*model.Board, error) {
	var b model.Board
	err := scanBoard(s.primary.QueryRow(ctx,
		`UPDATE boards SET name = $3, description = $4, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+boardCols,
		orgID, boardID, name, description,
	), &b)
	return &b, err
}

func (s *Store) DeleteBoard(ctx context.Context, orgID, boardID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM boards WHERE org_id = $1 AND id = $2`,
		orgID, boardID,
	)
	return err
}

func (s *Store) ListColumns(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Column, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, board_id, name, position, created_at, updated_at
		 FROM columns WHERE org_id = $1 AND board_id = $2 ORDER BY position`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []model.Column
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.OrgID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func (s *Store) GetColumn(ctx context.Context, orgID, columnID uuid.UUID) (*model.Column, error) {
	var c model.Column
	err := s.replica.QueryRow(ctx,
		`SELECT id, org_id, board_id, name, position, created_at, updated_at
		 FROM columns WHERE org_id = $1 AND id = $2`,
		orgID, columnID,
	).Scan(&c.ID, &c.OrgID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateColumn(ctx context.Context, orgID, boardID uuid.UUID, name string, position float64) (*model.Column, error) {
	var c model.Column
	err := s.primary.QueryRow(ctx,
		`INSERT INTO columns (org_id, board_id, name, position)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, board_id, name, position, created_at, updated_at`,
		orgID, boardID, name, position,
	).Scan(&c.ID, &c.OrgID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Store) MaxColumnPosition(ctx context.Context, boardID uuid.UUID) (float64, error) {
	var pos float64
	err := s.replica.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM columns WHERE board_id = $1`,
		boardID,
	).Scan(&pos)
	return pos, err
}

func (s *Store) RenameColumn(ctx context.Context, orgID, columnID uuid.UUID, name string) (*model.Column, error) {
	var c model.Column
	err := s.primary.QueryRow(ctx,
		`UPDATE columns SET name = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING id, org_id, board_id, name, position, created_at, updated_at`,
		orgID, columnID, name,
	).Scan(&c.ID, &c.OrgID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Store) DeleteColumn(ctx context.Context, orgID, columnID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM columns WHERE org_id = $1 AND id = $2`,
		orgID, columnID,
	)
	return err
}

func (s *Store) ListTags(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, board_id, name, color FROM tags WHERE org_id = $1 AND board_id = $2 ORDER BY name`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.Tag
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// --- Sprint store methods ---

const sprintCols = `id, org_id, board_id, name, status, start_date, end_date, created_by, created_at, updated_at`

func scanSprint(row interface{ Scan(dest ...any) error }, s *model.Sprint) error {
	return row.Scan(&s.ID, &s.OrgID, &s.BoardID, &s.Name, &s.Status,
		&s.StartDate, &s.EndDate, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
}

func (s *Store) ListSprints(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Sprint, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+sprintCols+`
		 FROM sprints WHERE org_id = $1 AND board_id = $2
		 ORDER BY created_at`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sprints []model.Sprint
	for rows.Next() {
		var sp model.Sprint
		if err := scanSprint(rows, &sp); err != nil {
			return nil, err
		}
		sprints = append(sprints, sp)
	}
	return sprints, rows.Err()
}

func (s *Store) GetActiveSprint(ctx context.Context, orgID, boardID uuid.UUID) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.replica.QueryRow(ctx,
		`SELECT `+sprintCols+`
		 FROM sprints WHERE org_id = $1 AND board_id = $2 AND status = 'active'`,
		orgID, boardID,
	), &sp)
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Store) GetSprint(ctx context.Context, orgID, sprintID uuid.UUID) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.replica.QueryRow(ctx,
		`SELECT `+sprintCols+` FROM sprints WHERE org_id = $1 AND id = $2`,
		orgID, sprintID,
	), &sp)
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Store) CreateSprint(ctx context.Context, orgID, boardID, createdBy uuid.UUID, name string, startDate, endDate *time.Time) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`INSERT INTO sprints (org_id, board_id, name, status, start_date, end_date, created_by)
		 VALUES ($1, $2, $3, 'planning', $4, $5, $6)
		 RETURNING `+sprintCols,
		orgID, boardID, name, startDate, endDate, createdBy,
	), &sp)
	return &sp, err
}

func (s *Store) UpdateSprint(ctx context.Context, orgID, sprintID uuid.UUID, name string, startDate, endDate *time.Time) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`UPDATE sprints SET name = $3, start_date = $4, end_date = $5, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+sprintCols,
		orgID, sprintID, name, startDate, endDate,
	), &sp)
	return &sp, err
}

func (s *Store) SetSprintStatus(ctx context.Context, orgID, sprintID uuid.UUID, status model.SprintStatus) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`UPDATE sprints SET status = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+sprintCols,
		orgID, sprintID, status,
	), &sp)
	return &sp, err
}

func (s *Store) DeleteSprint(ctx context.Context, orgID, sprintID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM sprints WHERE org_id = $1 AND id = $2`,
		orgID, sprintID,
	)
	return err
}

// CountBacklogTickets returns the number of tickets with sprint_id IS NULL for a board.
func (s *Store) CountBacklogTickets(ctx context.Context, orgID, boardID uuid.UUID) (int, error) {
	var n int
	err := s.replica.QueryRow(ctx,
		`SELECT COUNT(*) FROM tickets WHERE org_id = $1 AND board_id = $2 AND sprint_id IS NULL`,
		orgID, boardID,
	).Scan(&n)
	return n, err
}

// ListBacklogTickets returns tickets with sprint_id IS NULL for a scrum board, ordered by position.
func (s *Store) ListBacklogTickets(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.board_id = $2 AND t.sprint_id IS NULL
		 ORDER BY t.position`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []model.Ticket
	for rows.Next() {
		var t model.Ticket
		if err := scanTicket(rows, &t); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// ListSprintTickets returns tickets for a specific sprint, ordered by column position.
func (s *Store) ListSprintTickets(ctx context.Context, orgID, sprintID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.sprint_id = $2
		 ORDER BY t.column_id, t.position`,
		orgID, sprintID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []model.Ticket
	for rows.Next() {
		var t model.Ticket
		if err := scanTicket(rows, &t); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// AssignTicketToSprint sets sprint_id on a ticket (nil = move to backlog).
func (s *Store) AssignTicketToSprint(ctx context.Context, orgID, ticketID uuid.UUID, sprintID *uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE tickets SET sprint_id = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, ticketID, sprintID,
	)
	return err
}

// ReturnSprintTicketsToBacklog sets sprint_id = NULL for all non-done tickets in a sprint.
// "Done" is any column whose name is 'done' (case-insensitive).
func (s *Store) ReturnSprintTicketsToBacklog(ctx context.Context, orgID, sprintID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE tickets t SET sprint_id = NULL, updated_at = NOW()
		 FROM columns c
		 WHERE t.org_id = $1
		   AND t.sprint_id = $2
		   AND t.column_id = c.id
		   AND LOWER(c.name) != 'done'`,
		orgID, sprintID,
	)
	return err
}
