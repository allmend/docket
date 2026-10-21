package store

import (
	"context"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const boardCols = `id, org_id, team_id, name, description, mode, created_by, created_at, updated_at`

func scanBoard(row interface{ Scan(dest ...any) error }, b *model.Board) error {
	return row.Scan(&b.ID, &b.OrgID, &b.TeamID, &b.Name, &b.Description, &b.Mode, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
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
// The team key is empty if the board has no team; a column outside orgID is
// pgx.ErrNoRows.
func (s *Store) GetColumnMeta(ctx context.Context, orgID, columnID uuid.UUID) (colName, teamKey string, err error) {
	err = s.replica.QueryRow(ctx,
		`SELECT c.name, COALESCE(te.key, '')
		 FROM columns c
		 JOIN boards b ON b.id = c.board_id
		 LEFT JOIN teams te ON te.id = b.team_id
		 WHERE c.org_id = $1 AND c.id = $2`,
		orgID, columnID,
	).Scan(&colName, &teamKey)
	return
}

// CreateColumn inserts a column. The insert only proceeds when boardID belongs
// to orgID; otherwise it surfaces as pgx.ErrNoRows and nothing is written.
func (s *Store) CreateColumn(ctx context.Context, orgID, boardID uuid.UUID, name string, position float64) (*model.Column, error) {
	var c model.Column
	err := s.primary.QueryRow(ctx,
		`INSERT INTO columns (org_id, board_id, name, position)
		 SELECT $1, $2, $3, $4
		 WHERE EXISTS (SELECT 1 FROM boards WHERE id = $2 AND org_id = $1)
		 RETURNING id, org_id, board_id, name, position, created_at, updated_at`,
		orgID, boardID, name, position,
	).Scan(&c.ID, &c.OrgID, &c.BoardID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Store) MaxColumnPosition(ctx context.Context, orgID, boardID uuid.UUID) (float64, error) {
	var pos float64
	err := s.replica.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM columns WHERE org_id = $1 AND board_id = $2`,
		orgID, boardID,
	).Scan(&pos)
	return pos, err
}

// ColumnTicketCounts is where a column's tickets are, as the board shows them.
// Only the visible ones (Active and Planning sprints on a Scrum board, every
// ticket on a Kanban or Blank board) stop the column being deleted. The rest
// keep column_id after leaving a sprint but appear on no board.
type ColumnTicketCounts struct {
	ActiveSprint   int
	PlanningSprint int
	OnBoard        int // Kanban / Blank
	Hidden         int // backlog, completed sprints
}

// Visible is the number of tickets the user can see in the column.
func (c ColumnTicketCounts) Visible() int { return c.ActiveSprint + c.PlanningSprint + c.OnBoard }

func (s *Store) CountColumnTickets(ctx context.Context, orgID, columnID uuid.UUID) (ColumnTicketCounts, error) {
	var c ColumnTicketCounts
	err := s.replica.QueryRow(ctx,
		`SELECT
		   COUNT(*) FILTER (WHERE b.mode = 'scrum' AND sp.status = 'active'),
		   COUNT(*) FILTER (WHERE b.mode = 'scrum' AND sp.status = 'planning'),
		   COUNT(*) FILTER (WHERE b.mode <> 'scrum'),
		   COUNT(*) FILTER (WHERE b.mode = 'scrum' AND (sp.status IS NULL OR sp.status NOT IN ('active', 'planning')))
		 FROM tickets t
		 JOIN boards b ON b.id = t.board_id AND b.org_id = t.org_id
		 LEFT JOIN sprints sp ON sp.id = t.sprint_id AND sp.org_id = t.org_id
		 WHERE t.org_id = $1 AND t.column_id = $2`,
		orgID, columnID,
	).Scan(&c.ActiveSprint, &c.PlanningSprint, &c.OnBoard, &c.Hidden)
	return c, err
}

// FlowSummary loads a board's open tickets per column, in board order, and how
// many tickets closed in the last 7 days. Two queries regardless of size.
func (s *Store) FlowSummary(ctx context.Context, orgID, boardID uuid.UUID) (*model.FlowSummary, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT c.name,
		        COUNT(t.id) FILTER (WHERE t.closed_at IS NULL),
		        COALESCE(SUM(t.story_points) FILTER (WHERE t.closed_at IS NULL), 0)::float8
		 FROM columns c
		 LEFT JOIN tickets t ON t.column_id = c.id AND t.org_id = c.org_id
		 WHERE c.org_id = $1 AND c.board_id = $2
		 GROUP BY c.id, c.name, c.position
		 ORDER BY c.position`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	f := &model.FlowSummary{}
	for rows.Next() {
		var c model.FlowColumn
		if err := rows.Scan(&c.Name, &c.Open, &c.OpenPoints); err != nil {
			return nil, err
		}
		f.Columns = append(f.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	err = s.replica.QueryRow(ctx,
		`SELECT COUNT(*) FROM tickets
		 WHERE org_id = $1 AND board_id = $2 AND closed_at >= NOW() - INTERVAL '7 days'`,
		orgID, boardID,
	).Scan(&f.DoneLast7d)
	return f, err
}

// ReassignColumnTickets moves every ticket in one column to another column on
// the same board, keeping positions. Used to empty a column of tickets that
// aren't on the board (backlog, completed sprints) before deleting it.
func (s *Store) ReassignColumnTickets(ctx context.Context, orgID, fromColumnID, toColumnID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE tickets SET column_id = $3
		 WHERE org_id = $1 AND column_id = $2
		   AND EXISTS (SELECT 1 FROM columns f JOIN columns t2 ON t2.board_id = f.board_id
		               WHERE f.id = $2 AND t2.id = $3 AND f.org_id = $1 AND t2.org_id = $1)`,
		orgID, fromColumnID, toColumnID,
	)
	return err
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

// SwapColumnPositions exchanges the positions of two columns on the same board
// in one statement. Both columns must belong to the org and to boardID; if
// either doesn't, nothing changes and pgx.ErrNoRows is returned.
func (s *Store) SwapColumnPositions(ctx context.Context, orgID, boardID, aID, bID uuid.UUID) error {
	tag, err := s.primary.Exec(ctx,
		`UPDATE columns c
		 SET position = o.position, updated_at = NOW()
		 FROM columns o
		 WHERE c.org_id = $1 AND o.org_id = $1
		   AND c.board_id = $2 AND o.board_id = $2
		   AND ((c.id = $3 AND o.id = $4) OR (c.id = $4 AND o.id = $3))`,
		orgID, boardID, aID, bID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 2 {
		return pgx.ErrNoRows
	}
	return nil
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

// ListOrgTags returns every tag in the org, ordered by board then name. The
// New Ticket dialog uses it to offer each workspace's tracks without a query
// per workspace.
func (s *Store) ListOrgTags(ctx context.Context, orgID uuid.UUID) ([]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, org_id, board_id, name, color FROM tags WHERE org_id = $1 ORDER BY board_id, name`,
		orgID,
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

func (s *Store) CreateTag(ctx context.Context, orgID, boardID uuid.UUID, name, color, description string, leadUserID *uuid.UUID) (*model.Tag, error) {
	var t model.Tag
	err := s.primary.QueryRow(ctx,
		// Resolve lead_user_id through an org-scoped lookup so a cross-org user
		// ID is stored as NULL rather than referencing another tenant's user.
		// The board must belong to the org too, or nothing is inserted.
		`INSERT INTO tags (org_id, board_id, name, color, description, lead_user_id)
		 SELECT $1, $2, $3, $4, $5, (SELECT id FROM users WHERE id = $6 AND org_id = $1)
		 WHERE EXISTS (SELECT 1 FROM boards WHERE id = $2 AND org_id = $1)
		 RETURNING id, org_id, board_id, name, color, description, lead_user_id`,
		orgID, boardID, name, color, description, leadUserID,
	).Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color, &t.Description, &t.LeadUserID)
	if isUniqueViolation(err) {
		return nil, ErrDuplicate // tag names are unique per board
	}
	return &t, err
}

func (s *Store) UpdateTag(ctx context.Context, orgID, tagID uuid.UUID, name, color, description string, leadUserID *uuid.UUID) (*model.Tag, error) {
	var t model.Tag
	err := s.primary.QueryRow(ctx,
		// lead_user_id resolved via an org-scoped lookup — a cross-org user ID
		// becomes NULL instead of referencing another tenant's user.
		`UPDATE tags SET name = $3, color = $4, description = $5,
		        lead_user_id = (SELECT id FROM users WHERE id = $6 AND org_id = $1)
		 WHERE org_id = $1 AND id = $2
		 RETURNING id, org_id, board_id, name, color, description, lead_user_id`,
		orgID, tagID, name, color, description, leadUserID,
	).Scan(&t.ID, &t.OrgID, &t.BoardID, &t.Name, &t.Color, &t.Description, &t.LeadUserID)
	if isUniqueViolation(err) {
		return nil, ErrDuplicate // tag names are unique per board
	}
	return &t, err
}

// ListTrackStats returns the board's tags with lead names and open-ticket
// counters for the workspace settings Tracks panel.
func (s *Store) ListTrackStats(ctx context.Context, orgID, boardID uuid.UUID) ([]model.TrackStat, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT g.id, g.org_id, g.board_id, g.name, g.color, g.description, g.lead_user_id,
		        COALESCE(u.name, ''),
		        COUNT(t.id) FILTER (WHERE t.closed_at IS NULL),
		        COALESCE(SUM(t.story_points) FILTER (WHERE t.closed_at IS NULL), 0)
		 FROM tags g
		 LEFT JOIN users u ON u.id = g.lead_user_id
		 LEFT JOIN ticket_tags tt ON tt.tag_id = g.id
		 LEFT JOIN tickets t ON t.id = tt.ticket_id
		 WHERE g.org_id = $1 AND g.board_id = $2
		 GROUP BY g.id, u.name
		 ORDER BY g.name`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TrackStat
	for rows.Next() {
		var ts model.TrackStat
		if err := rows.Scan(&ts.ID, &ts.OrgID, &ts.BoardID, &ts.Name, &ts.Color, &ts.Description, &ts.LeadUserID,
			&ts.LeadName, &ts.OpenCount, &ts.OpenPoints); err != nil {
			return nil, err
		}
		out = append(out, ts)
	}
	return out, rows.Err()
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

func (s *Store) BulkListTicketTags(ctx context.Context, orgID, boardID uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT tt.ticket_id, tg.id, tg.org_id, tg.board_id, tg.name, tg.color
		 FROM ticket_tags tt
		 JOIN tickets t ON t.id = tt.ticket_id
		 JOIN tags tg ON tg.id = tt.tag_id
		 WHERE t.board_id = $1 AND t.org_id = $2
		 ORDER BY tg.name`,
		boardID, orgID,
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

// BulkListTicketTagsByTeam loads tags for every ticket in a team, keyed by ticket
// ID. The public API lists by team, which the board-scoped variant above cannot
// serve because a team may own several boards.
func (s *Store) BulkListTicketTagsByTeam(ctx context.Context, orgID, teamID uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT tt.ticket_id, tg.id, tg.org_id, tg.board_id, tg.name, tg.color
		 FROM ticket_tags tt
		 JOIN tickets t ON t.id = tt.ticket_id
		 JOIN tags tg ON tg.id = tt.tag_id
		 WHERE t.org_id = $1 AND t.team_id = $2
		 ORDER BY tg.name`,
		orgID, teamID,
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
		 SELECT $1, $2
		 WHERE EXISTS (SELECT 1 FROM tickets WHERE id = $1 AND org_id = $3)
		   AND EXISTS (SELECT 1 FROM tags    WHERE id = $2 AND org_id = $3)
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
	s.fillSprintLiveStats(ctx, orgID, &sp)
	return &sp, nil
}

// fillSprintLiveStats queries the tickets table for live ticket and story-point
// totals and writes them into the sprint's committed/completed fields. The snapshot
// columns on the sprints table are only written at sprint-close time (see
// SnapshotSprintStats), so they read as 0 for an in-flight sprint; this gives the
// dashboard real numbers. Completed sprints are skipped: their snapshot is accurate
// and must not be overwritten, since non-done tickets leave the sprint at close and
// a live query would undercount what was committed.
func (s *Store) fillSprintLiveStats(ctx context.Context, orgID uuid.UUID, sp *model.Sprint) {
	if sp.Status.IsCompleted() {
		return
	}
	s.replica.QueryRow(ctx,
		`SELECT
		    COUNT(*),
		    COUNT(*) FILTER (WHERE LOWER(c.name) = 'done' AND COALESCE(t.close_reason, 'done') = 'done'),
		    COALESCE(SUM(t.story_points), 0),
		    COALESCE(SUM(t.story_points) FILTER (WHERE LOWER(c.name) = 'done' AND COALESCE(t.close_reason, 'done') = 'done'), 0)
		 FROM tickets t
		 LEFT JOIN columns c ON c.id = t.column_id
		 WHERE t.org_id = $1 AND t.sprint_id = $2`,
		orgID, sp.ID,
	).Scan(&sp.CommittedTickets, &sp.CompletedTickets, &sp.CommittedPoints, &sp.CompletedPoints)
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
		s.fillSprintLiveStats(ctx, orgID, &sp)
	}
	return &sp, nil
}

func (s *Store) CreateSprint(ctx context.Context, orgID, boardID, createdBy uuid.UUID, name, goal string, startDate, endDate *time.Time) (*model.Sprint, error) {
	var sp model.Sprint
	err := scanSprint(s.primary.QueryRow(ctx,
		`INSERT INTO sprints (org_id, board_id, name, goal, status, start_date, end_date, created_by)
		 SELECT $1, $2, $3, $4, 'planning', $5, $6, $7
		 WHERE EXISTS (SELECT 1 FROM boards WHERE id = $2 AND org_id = $1)
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

// CompleteSprint flips an active sprint to completed. It is the first statement
// of the close transaction, and the row lock it takes serialises closes: a
// second, concurrent close waits, then finds the sprint no longer active and
// gets pgx.ErrNoRows — rather than re-snapshotting a sprint whose unfinished
// tickets have already gone back to the backlog, which would overwrite the
// real numbers with an undercount.
func (s *Store) CompleteSprint(ctx context.Context, orgID, sprintID uuid.UUID) error {
	tag, err := s.primary.Exec(ctx,
		`UPDATE sprints SET status = 'completed', updated_at = NOW()
		 WHERE org_id = $1 AND id = $2 AND status = 'active'`,
		orgID, sprintID,
	)
	return noRowsIfUnaffected(tag, err)
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
//
// The sprint must belong to the org and to the ticket's own board, and must not
// be completed: a sprint ID from another board would drag the ticket onto that
// board's columns, one from another org would store a cross-tenant reference,
// and completed sprints are read-only. A rejected assignment writes nothing and
// returns pgx.ErrNoRows, as does an unknown ticket.
func (s *Store) AssignTicketToSprint(ctx context.Context, orgID, ticketID uuid.UUID, sprintID *uuid.UUID) error {
	if sprintID == nil {
		tag, err := s.primary.Exec(ctx,
			`UPDATE tickets SET sprint_id = NULL, updated_at = NOW()
			 WHERE org_id = $1 AND id = $2`,
			orgID, ticketID,
		)
		return noRowsIfUnaffected(tag, err)
	}
	tag, err := s.primary.Exec(ctx,
		`UPDATE tickets t SET
		   sprint_id = sp.id,
		   column_id = (
		     SELECT c.id FROM columns c
		     WHERE c.org_id = $1 AND c.board_id = sp.board_id
		     ORDER BY c.position
		     LIMIT 1
		   ),
		   updated_at = NOW()
		 FROM sprints sp
		 WHERE t.org_id = $1 AND t.id = $2
		   AND sp.org_id = $1 AND sp.id = $3
		   AND sp.board_id = t.board_id
		   AND sp.status <> 'completed'`,
		orgID, ticketID, *sprintID,
	)
	return noRowsIfUnaffected(tag, err)
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
		        WHERE t.org_id = $1 AND t.sprint_id = $2 AND LOWER(c.name) = 'done' AND COALESCE(t.close_reason, 'done') = 'done'
		    ),
		    committed_points = (
		        SELECT COALESCE(SUM(story_points), 0) FROM tickets
		        WHERE org_id = $1 AND sprint_id = $2 AND story_points IS NOT NULL
		    ),
		    completed_points = (
		        SELECT COALESCE(SUM(t.story_points), 0) FROM tickets t
		        JOIN columns c ON c.id = t.column_id
		        WHERE t.org_id = $1 AND t.sprint_id = $2
		          AND LOWER(c.name) = 'done' AND COALESCE(t.close_reason, 'done') = 'done' AND t.story_points IS NOT NULL
		    ),
		    updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, sprintID,
	)
	return err
}

// ListSprintTicketsForReview returns a sprint's completed tickets (in Done and
// closed as done) and the ones returning to the backlog (not in Done). Tickets
// closed for another reason (duplicate, won't do, …) sit in Done but are in
// neither list: they were neither delivered nor carried over.
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
		WHERE t.org_id = $1 AND t.sprint_id = $2 AND LOWER(c2.name) = 'done' AND COALESCE(t.close_reason, 'done') = 'done'
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

// ListBoardSprintTicketsSummary returns lightweight ticket rows for every
// sprint on a board, keyed by sprint, in position order — one query for the
// whole roadmap.
func (s *Store) ListBoardSprintTicketsSummary(ctx context.Context, orgID, boardID uuid.UUID) (map[uuid.UUID][]model.RoadmapTicket, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT t.sprint_id, t.id, t.title, t.priority, COALESCE(tm.key, ''), t.number,
		       (t.closed_at IS NOT NULL OR LOWER(c.name) = 'done') AS is_done
		FROM tickets t
		JOIN sprints sp ON sp.id = t.sprint_id AND sp.org_id = t.org_id
		LEFT JOIN teams tm ON tm.id = t.team_id
		LEFT JOIN columns c ON c.id = t.column_id
		WHERE t.org_id = $1 AND sp.board_id = $2
		ORDER BY t.position
	`, orgID, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID][]model.RoadmapTicket)
	for rows.Next() {
		var sprintID uuid.UUID
		var t model.RoadmapTicket
		if err := rows.Scan(&sprintID, &t.ID, &t.Title, &t.Priority, &t.TeamKey, &t.Number, &t.IsDone); err != nil {
			return nil, err
		}
		out[sprintID] = append(out[sprintID], t)
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
