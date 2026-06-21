package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// ticketCols is the SELECT column list shared by all ticket queries.
// Joins users for assignee name and teams for the display key.
const ticketCols = `
	t.id, t.org_id, t.board_id, t.column_id,
	t.team_id, t.assignee_id, t.created_by,
	t.number, COALESCE(tm.key, ''),
	t.title, t.body, t.acceptance_criteria, t.priority, t.story_points, t.position,
	t.sprint_id, t.external_ref, t.closed_at, t.close_reason, t.created_at, t.updated_at,
	u.name, COALESCE(ub.name, '')`

const ticketJoins = `
	FROM tickets t
	LEFT JOIN users u  ON u.id  = t.assignee_id
	LEFT JOIN users ub ON ub.id = t.created_by
	LEFT JOIN teams tm ON tm.id = t.team_id`

// ticketColsReturning is used after INSERT/UPDATE where no JOIN is available.
// It fetches the team key via a subquery so DisplayID() works immediately.
const ticketColsReturning = `
	id, org_id, board_id, column_id,
	team_id, assignee_id, created_by,
	number, COALESCE((SELECT key FROM teams WHERE id = team_id), ''),
	title, body, acceptance_criteria, priority, story_points, position,
	sprint_id, external_ref, closed_at, close_reason, created_at, updated_at,
	(SELECT name FROM users WHERE id = assignee_id),
	COALESCE((SELECT name FROM users WHERE id = created_by), '')`

func scanTicket(row interface{ Scan(dest ...any) error }, t *model.Ticket) error {
	return row.Scan(
		&t.ID, &t.OrgID, &t.BoardID, &t.ColumnID,
		&t.TeamID, &t.AssigneeID, &t.CreatedBy,
		&t.Number, &t.TeamKey,
		&t.Title, &t.Body, &t.AcceptanceCriteria, &t.Priority, &t.StoryPoints, &t.Position,
		&t.SprintID, &t.ExternalRef, &t.ClosedAt, &t.CloseReason, &t.CreatedAt, &t.UpdatedAt,
		&t.AssigneeName, &t.CreatedByName,
	)
}

func (s *Store) ListTicketsByBoard(ctx context.Context, orgID, boardID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.board_id = $2
		 ORDER BY t.column_id, t.position`,
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

func (s *Store) ListTicketsByColumn(ctx context.Context, orgID, columnID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.column_id = $2
		 ORDER BY t.position`,
		orgID, columnID,
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

func (s *Store) ListTicketsByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.team_id = $2
		 ORDER BY t.number`,
		orgID, teamID,
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

func (s *Store) GetTicket(ctx context.Context, orgID, ticketID uuid.UUID) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.replica.QueryRow(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.id = $2`,
		orgID, ticketID,
	), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) CreateTicket(ctx context.Context,
	orgID, boardID, columnID, createdBy uuid.UUID,
	teamID *uuid.UUID, number int,
	title, body string, priority model.Priority, position float64,
) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`INSERT INTO tickets
		     (org_id, board_id, column_id, created_by, team_id, number, title, body, priority, position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING `+ticketColsReturning,
		orgID, boardID, columnID, createdBy, teamID, number, title, body, priority, position,
	), &t)
	return &t, err
}

func (s *Store) MoveTicket(ctx context.Context, orgID, ticketID, columnID uuid.UUID, position float64) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE tickets
		 SET column_id  = CASE WHEN $3 = '00000000-0000-0000-0000-000000000000'::uuid THEN column_id ELSE $3 END,
		     position   = $4,
		     updated_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, ticketID, columnID, position,
	)
	return err
}

func (s *Store) UpdateTicket(ctx context.Context,
	orgID, ticketID uuid.UUID,
	title, body string, priority model.Priority, assigneeID *uuid.UUID,
) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets
		 SET title = $3, body = $4, priority = $5, assignee_id = $6, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, title, body, priority, assigneeID,
	), &t)
	return &t, err
}

func (s *Store) UpdateTicketTitle(ctx context.Context, orgID, ticketID uuid.UUID, title string) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET title = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, title,
	), &t)
	return &t, err
}

func (s *Store) UpdateTicketBody(ctx context.Context, orgID, ticketID uuid.UUID, body string) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET body = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, body,
	), &t)
	return &t, err
}

func (s *Store) GetTicketAC(ctx context.Context, orgID, ticketID uuid.UUID) (string, error) {
	var ac string
	err := s.replica.QueryRow(ctx,
		`SELECT acceptance_criteria FROM tickets WHERE org_id = $1 AND id = $2`,
		orgID, ticketID,
	).Scan(&ac)
	return ac, err
}

func (s *Store) UpdateTicketAC(ctx context.Context, orgID, ticketID uuid.UUID, ac string) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET acceptance_criteria = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, ac,
	), &t)
	return &t, err
}

// SearchTicketsForLink returns tickets matching a query string by display ID prefix or title,
// excluding the given ticket. Used for the link autocomplete.
func (s *Store) SearchTicketsForLink(ctx context.Context, orgID, excludeID uuid.UUID, q string) ([]model.Ticket, error) {
	like := "%" + q + "%"
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1 AND t.id != $2
		   AND t.closed_at IS NULL
		   AND (COALESCE(tm.key || '-' || t.number::text, '') ILIKE $3 OR t.title ILIKE $3)
		 ORDER BY tm.key, t.number
		 LIMIT 20`,
		orgID, excludeID, like,
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

// SearchTicketsForMention returns tickets matching a query string by display ID or title.
// Used for the #ticket autocomplete in the editor.
func (s *Store) SearchTicketsForMention(ctx context.Context, orgID uuid.UUID, q string) ([]model.Ticket, error) {
	like := "%" + q + "%"
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1
		   AND (COALESCE(tm.key || '-' || t.number::text, '') ILIKE $2 OR t.title ILIKE $2)
		 ORDER BY tm.key, t.number
		 LIMIT 10`,
		orgID, like,
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

// ListTicketsByAssignee returns all tickets the given user is assigned to, ordered by priority then number.
func (s *Store) ListTicketsByAssignee(ctx context.Context, orgID, userID uuid.UUID) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 JOIN ticket_assignees ta ON ta.ticket_id = t.id
		 WHERE t.org_id = $1 AND ta.user_id = $2
		 ORDER BY t.priority DESC, t.number`,
		orgID, userID,
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

// ListActivityByActor returns history entries where the given user was the actor
// within the last 24 hours — used for "since yesterday" in the daily stand-up.
func (s *Store) ListActivityByActor(ctx context.Context, orgID, actorID uuid.UUID) ([]model.InboxEntry, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT h.id, h.ticket_id, h.actor_id, h.actor_name, h.field, h.old_value, h.new_value, h.created_at,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title
		 FROM ticket_history h
		 JOIN tickets t ON t.id = h.ticket_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1
		   AND h.actor_id = $2
		   AND h.created_at > NOW() - INTERVAL '24 hours'
		 ORDER BY h.created_at DESC
		 LIMIT 15`,
		orgID, actorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []model.InboxEntry
	for rows.Next() {
		var e model.InboxEntry
		if err := rows.Scan(
			&e.ID, &e.TicketID, &e.ActorID, &e.ActorName,
			&e.Field, &e.OldValue, &e.NewValue, &e.CreatedAt,
			&e.TicketDisplayID, &e.TicketTitle,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) DeleteTicket(ctx context.Context, orgID, ticketID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM tickets WHERE org_id = $1 AND id = $2`,
		orgID, ticketID,
	)
	return err
}

func (s *Store) MaxTicketPositionInColumn(ctx context.Context, columnID uuid.UUID) (float64, error) {
	var pos float64
	err := s.replica.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) FROM tickets WHERE column_id = $1`,
		columnID,
	).Scan(&pos)
	return pos, err
}

func (s *Store) BulkListTicketAssignees(ctx context.Context, boardID uuid.UUID) (map[uuid.UUID][]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT ta.ticket_id, u.id, u.org_id, u.username, u.name, u.email, u.role, u.created_at, u.updated_at
		 FROM ticket_assignees ta
		 JOIN tickets t ON t.id = ta.ticket_id
		 JOIN users u ON u.id = ta.user_id
		 WHERE t.board_id = $1
		 ORDER BY u.name`,
		boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[uuid.UUID][]model.User)
	for rows.Next() {
		var ticketID uuid.UUID
		var u model.User
		if err := rows.Scan(&ticketID, &u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		result[ticketID] = append(result[ticketID], u)
	}
	return result, rows.Err()
}

func (s *Store) UpdateTicketPriority(ctx context.Context, orgID, ticketID uuid.UUID, priority model.Priority) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET priority = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, priority,
	), &t)
	return &t, err
}

func (s *Store) CloseTicket(ctx context.Context, orgID, ticketID uuid.UUID, reason string) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET closed_at = NOW(), close_reason = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, reason,
	), &t)
	return &t, err
}

func (s *Store) ReopenTicket(ctx context.Context, orgID, ticketID uuid.UUID) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET closed_at = NULL, close_reason = NULL, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID,
	), &t)
	return &t, err
}

func (s *Store) UpdateTicketPoints(ctx context.Context, orgID, ticketID uuid.UUID, points *float64) (*model.Ticket, error) {
	var t model.Ticket
	err := scanTicket(s.primary.QueryRow(ctx,
		`UPDATE tickets SET story_points = $3, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING `+ticketColsReturning,
		orgID, ticketID, points,
	), &t)
	return &t, err
}

func (s *Store) ListTicketAssignees(ctx context.Context, ticketID uuid.UUID) ([]model.User, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT u.id, u.org_id, u.username, u.name, u.email, u.role, u.created_at, u.updated_at
		 FROM ticket_assignees ta
		 JOIN users u ON u.id = ta.user_id
		 WHERE ta.ticket_id = $1
		 ORDER BY u.name`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Username, &u.Name, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) AddTicketAssignee(ctx context.Context, ticketID, userID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO ticket_assignees (ticket_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		ticketID, userID,
	)
	return err
}

func (s *Store) RemoveTicketAssignee(ctx context.Context, ticketID, userID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM ticket_assignees WHERE ticket_id = $1 AND user_id = $2`,
		ticketID, userID,
	)
	return err
}

func (s *Store) SearchTickets(ctx context.Context, orgID uuid.UUID, query string) ([]model.Ticket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+ticketCols+ticketJoins+`
		 WHERE t.org_id = $1
		   AND t.search_vector @@ websearch_to_tsquery('english', $2)
		 ORDER BY ts_rank(t.search_vector, websearch_to_tsquery('english', $2)) DESC
		 LIMIT 50`,
		orgID, query,
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
