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

// GetColumnMeta returns the column name and its team's key in one query.
// Used for populating metric labels without multiple round-trips.
// Returns empty strings (not an error) if the column has no board or team.
func (s *Store) GetColumnMeta(ctx context.Context, columnID uuid.UUID) (colName, teamKey string, err error) {
	err = s.replica.QueryRow(ctx,
		`SELECT c.name, COALESCE(te.key, '')
		 FROM columns c
		 JOIN boards b ON b.id = c.board_id
		 LEFT JOIN teams te ON te.id = b.team_id
		 WHERE c.id = $1`,
		columnID,
	).Scan(&colName, &teamKey)
	return
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

// ListTagsByOrg returns all tags for the org grouped by board ID, for nav rendering.
func (s *Store) ListTagsByOrg(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, board_id, name, color FROM tags WHERE org_id = $1 ORDER BY board_id, name`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]model.Tag)
	for rows.Next() {
		var t model.Tag
		if err := rows.Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		result[t.BoardID] = append(result[t.BoardID], t)
	}
	return result, rows.Err()
}

func (s *Store) CreateTag(ctx context.Context, orgID, boardID uuid.UUID, name, color string) (*model.Tag, error) {
	var t model.Tag
	err := s.primary.QueryRow(ctx,
		`INSERT INTO tags (org_id, board_id, name, color) VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, board_id, name, color`,
		orgID, boardID, name, color,
	).Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color)
	return &t, err
}

func (s *Store) UpdateTag(ctx context.Context, orgID, tagID uuid.UUID, name, color string) (*model.Tag, error) {
	var t model.Tag
	err := s.primary.QueryRow(ctx,
		`UPDATE tags SET name = $3, color = $4 WHERE org_id = $1 AND id = $2
		 RETURNING id, org_id, board_id, name, color`,
		orgID, tagID, name, color,
	).Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color)
	return &t, err
}

func (s *Store) DeleteTag(ctx context.Context, orgID, tagID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM tags WHERE org_id = $1 AND id = $2`,
		orgID, tagID,
	)
	return err
}

func (s *Store) ListTicketTags(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT t.id, t.org_id, t.board_id, t.name, t.color
		 FROM tags t JOIN ticket_tags tt ON tt.tag_id = t.id
		 WHERE t.org_id = $1 AND tt.ticket_id = $2 ORDER BY t.name`,
		orgID, ticketID,
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

func (s *Store) BulkListTicketTags(ctx context.Context, boardID uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT tt.ticket_id, tg.id, tg.org_id, tg.board_id, tg.name, tg.color
		 FROM ticket_tags tt
		 JOIN tickets t ON t.id = tt.ticket_id
		 JOIN tags tg ON tg.id = tt.tag_id
		 WHERE t.board_id = $1
		 ORDER BY tg.name`,
		boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID][]model.Tag)
	for rows.Next() {
		var ticketID uuid.UUID
		var tg model.Tag
		if err := rows.Scan(&ticketID, &tg.ID, &tg.OrgID, &tg.BoardID, &tg.Name, &tg.Color); err != nil {
			return nil, err
		}
		result[ticketID] = append(result[ticketID], tg)
	}
	return result, rows.Err()
}

func (s *Store) AddTagToTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO ticket_tags (ticket_id, tag_id)
		 SELECT $1, $2 FROM tickets WHERE id = $1 AND org_id = $3
		 ON CONFLICT DO NOTHING`,
		ticketID, tagID, orgID,
	)
	return err
}

func (s *Store) RemoveTagFromTicket(ctx context.Context, orgID, ticketID, tagID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM ticket_tags tt USING tickets t
		 WHERE tt.ticket_id = t.id AND t.org_id = $1
		   AND tt.ticket_id = $2 AND tt.tag_id = $3`,
		orgID, ticketID, tagID,
	)
	return err
}

// GetTag returns a single tag by ID.
func (s *Store) GetTag(ctx context.Context, orgID, tagID uuid.UUID) (*model.Tag, error) {
	var t model.Tag
	err := s.replica.QueryRow(ctx,
		`SELECT id, org_id, board_id, name, color FROM tags WHERE org_id = $1 AND id = $2`,
		orgID, tagID,
	).Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTicketsByTag returns all tickets tagged with a given tag, newest first.
func (s *Store) ListTicketsByTag(ctx context.Context, orgID, tagID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 JOIN ticket_tags tt ON tt.ticket_id = t.id
		 WHERE t.org_id = $1 AND tt.tag_id = $2
		 ORDER BY t.closed_at NULLS FIRST, t.updated_at DESC`,
		orgID, tagID,
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

// --- Sprint store methods ---

const sprintCols = `id, org_id, board_id, name, goal, status, start_date, end_date,
	committed_tickets, completed_tickets, committed_points, completed_points,
	created_by, created_at, updated_at`

func scanSprint(row interface{ Scan(dest ...any) error }, s *model.Sprint) error {
	return row.Scan(&s.ID, &s.OrgID, &s.BoardID, &s.Name, &s.Goal, &s.Status,
		&s.StartDate, &s.EndDate,
		&s.CommittedTickets, &s.CompletedTickets, &s.CommittedPoints, &s.CompletedPoints,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
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
	s.fillSprintLivePoints(ctx, orgID, &sp)
	return &sp, nil
}

// fillSprintLivePoints queries the tickets table for live story-point totals and
// writes them into sp.CommittedPoints / sp.CompletedPoints. The snapshot columns
// on the sprints table are only written at sprint-close time, so they read as 0
// for the current active sprint; this gives the dashboard real numbers.
func (s *Store) fillSprintLivePoints(ctx context.Context, orgID uuid.UUID, sp *model.Sprint) {
	s.replica.QueryRow(ctx,
		`SELECT
		    COALESCE(SUM(t.story_points), 0),
		    COALESCE(SUM(t.story_points) FILTER (WHERE LOWER(c.name) = 'done'), 0)
		 FROM tickets t
		 LEFT JOIN columns c ON c.id = t.column_id
		 WHERE t.org_id = $1 AND t.sprint_id = $2
		   AND t.story_points IS NOT NULL`,
		orgID, sp.ID,
	).Scan(&sp.CommittedPoints, &sp.CompletedPoints)
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
	if sp.Status.IsActive() {
		s.fillSprintLivePoints(ctx, orgID, &sp)
	}
	return &sp, nil
}

func (s *Store) CreateSprint(ctx context.Context, orgID, boardID, createdBy uuid.UUID, name, goal string, startDate, endDate *time.Time) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`INSERT INTO sprints (org_id, board_id, name, goal, status, start_date, end_date, created_by)
		 VALUES ($1, $2, $3, $4, 'planning', $5, $6, $7)
		 RETURNING `+sprintCols,
		orgID, boardID, name, goal, startDate, endDate, createdBy,
	), &sp)
	return &sp, err
}

func (s *Store) UpdateSprint(ctx context.Context, orgID, sprintID uuid.UUID, name, goal string, startDate, endDate *time.Time) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`UPDATE sprints SET name = $3, goal = $4, start_date = $5, end_date = $6, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+sprintCols,
		orgID, sprintID, name, goal, startDate, endDate,
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
// When assigning to a sprint, column_id is set to the first column of the sprint's board
// so the ticket always lands in "To Do" regardless of where it was in a previous sprint.
// When moving to backlog, column_id is cleared.
func (s *Store) AssignTicketToSprint(ctx context.Context, orgID, ticketID uuid.UUID, sprintID *uuid.UUID) error {
	if sprintID == nil {
		_, err := s.primary.Exec(ctx,
			`UPDATE tickets SET sprint_id = NULL, updated_at = NOW()
			 WHERE org_id = $1 AND id = $2`,
			orgID, ticketID,
		)
		return err
	}
	_, err := s.primary.Exec(ctx,
		`UPDATE tickets SET
		   sprint_id = $3,
		   column_id = (
		     SELECT c.id FROM columns c
		     JOIN sprints sp ON sp.board_id = c.board_id
		     WHERE sp.org_id = $1 AND sp.id = $3
		     ORDER BY c.position
		     LIMIT 1
		   ),
		   updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, ticketID, *sprintID,
	)
	return err
}

// SnapshotSprintStats captures committed/completed ticket and point counts onto the sprint row.
// Must be called BEFORE ReturnSprintTicketsToBacklog so the data is still accurate.
func (s *Store) SnapshotSprintStats(ctx context.Context, orgID, sprintID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE sprints SET
		    committed_tickets = (
		        SELECT COUNT(*) FROM tickets WHERE org_id = $1 AND sprint_id = $2
		    ),
		    completed_tickets = (
		        SELECT COUNT(*) FROM tickets t
		        JOIN columns c ON c.id = t.column_id
		        WHERE t.org_id = $1 AND t.sprint_id = $2 AND LOWER(c.name) = 'done'
		    ),
		    committed_points = (
		        SELECT COALESCE(SUM(story_points), 0) FROM tickets
		        WHERE org_id = $1 AND sprint_id = $2 AND story_points IS NOT NULL
		    ),
		    completed_points = (
		        SELECT COALESCE(SUM(t.story_points), 0) FROM tickets t
		        JOIN columns c ON c.id = t.column_id
		        WHERE t.org_id = $1 AND t.sprint_id = $2
		          AND LOWER(c.name) = 'done' AND t.story_points IS NOT NULL
		    ),
		    updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, sprintID,
	)
	return err
}

// ListSprintTicketsForReview returns all tickets in a sprint partitioned into done/not-done.
func (s *Store) ListSprintTicketsForReview(ctx context.Context, orgID, sprintID uuid.UUID) (completed, returning []model.Ticket, err error) {
	scan := func(q string) ([]model.Ticket, error) {
		rows, err := s.replica.Query(ctx, q, orgID, sprintID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []model.Ticket
		for rows.Next() {
			var t model.Ticket
			if err := scanTicket(rows, &t); err != nil {
				return nil, err
			}
			out = append(out, t)
		}
		return out, rows.Err()
	}

	doneQ := `SELECT ` + ticketCols + ticketJoins + `
		JOIN columns c2 ON c2.id = t.column_id
		WHERE t.org_id = $1 AND t.sprint_id = $2 AND LOWER(c2.name) = 'done'
		ORDER BY t.position`

	returnQ := `SELECT ` + ticketCols + ticketJoins + `
		JOIN columns c2 ON c2.id = t.column_id
		WHERE t.org_id = $1 AND t.sprint_id = $2 AND LOWER(c2.name) != 'done'
		ORDER BY t.position`

	completed, err = scan(doneQ)
	if err != nil {
		return nil, nil, err
	}
	returning, err = scan(returnQ)
	return completed, returning, err
}

// ListSprintTicketsSummary returns lightweight ticket rows for a sprint, used by the roadmap.
func (s *Store) ListSprintTicketsSummary(ctx context.Context, orgID, sprintID uuid.UUID) ([]model.RoadmapTicket, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT t.id, t.title, t.priority, COALESCE(tm.key, ''), t.number,
		       (t.closed_at IS NOT NULL OR LOWER(c.name) = 'done') AS is_done
		FROM tickets t
		LEFT JOIN teams tm ON tm.id = t.team_id
		LEFT JOIN columns c ON c.id = t.column_id
		WHERE t.org_id = $1 AND t.sprint_id = $2
		ORDER BY t.position
	`, orgID, sprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.RoadmapTicket
	for rows.Next() {
		var t model.RoadmapTicket
		if err := rows.Scan(&t.ID, &t.Title, &t.Priority, &t.TeamKey, &t.Number, &t.IsDone); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
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
		   AND LOWER(c.name) != 'done'
		   AND t.closed_at IS NULL`,
		orgID, sprintID,
	)
	return err
}
