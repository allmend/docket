package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// ListBlockedTickets returns up to 10 open blocked tickets for the org (any sprint or backlog).
// A ticket is blocked when another ticket has a 'blocks' link pointing to it.
func (s *Store) ListBlockedTickets(ctx context.Context, orgID uuid.UUID) ([]model.DashboardBlockedTicket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT t.id,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title, t.priority,
		        COALESCE(c.name, ''),
		        COALESCE((
		            SELECT u.name FROM ticket_assignees ta
		            JOIN users u ON u.id = ta.user_id
		            WHERE ta.ticket_id = t.id
		            ORDER BY u.name LIMIT 1
		        ), '')
		 FROM tickets t
		 LEFT JOIN columns c ON c.id = t.column_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1 AND t.closed_at IS NULL
		   AND EXISTS (
		       SELECT 1 FROM ticket_links tl
		       WHERE tl.to_ticket_id = t.id AND tl.relation_type = 'blocks'
		   )
		 ORDER BY `+priorityOrder+`, t.number
		 LIMIT 10`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DashboardBlockedTicket
	for rows.Next() {
		var r model.DashboardBlockedTicket
		if err := rows.Scan(&r.ID, &r.DisplayID, &r.Title, &r.Priority, &r.ColumnName, &r.AssigneeName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListMyOpenTickets returns all open tickets assigned to the user, ordered by:
// active-sprint first, then planning sprint, then backlog; within each group by priority then number.
func (s *Store) ListMyOpenTickets(ctx context.Context, orgID, userID uuid.UUID) ([]model.DashboardMyTicket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT t.id,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title, t.priority, t.story_points,
		        CASE
		            WHEN sp.status = 'active'   THEN 'In sprint'
		            WHEN sp.status = 'planning' THEN 'Planning'
		            ELSE 'Backlog'
		        END AS group_label
		 FROM tickets t
		 JOIN ticket_assignees ta ON ta.ticket_id = t.id
		 LEFT JOIN sprints sp ON sp.id = t.sprint_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1 AND ta.user_id = $2 AND t.closed_at IS NULL
		 ORDER BY
		     CASE WHEN sp.status = 'active'   THEN 0
		          WHEN sp.status = 'planning' THEN 1
		          ELSE 2 END,
		     `+priorityOrder+`,
		     t.number
		 LIMIT 25`,
		orgID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DashboardMyTicket
	for rows.Next() {
		var r model.DashboardMyTicket
		if err := rows.Scan(&r.ID, &r.DisplayID, &r.Title, &r.Priority, &r.StoryPoints, &r.ColumnName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListBlockedTicketsByTeam is like ListBlockedTickets but scoped to a single team's tickets.
func (s *Store) ListBlockedTicketsByTeam(ctx context.Context, orgID, teamID uuid.UUID) ([]model.DashboardBlockedTicket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT t.id,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title, t.priority,
		        COALESCE(c.name, ''),
		        COALESCE((
		            SELECT u.name FROM ticket_assignees ta
		            JOIN users u ON u.id = ta.user_id
		            WHERE ta.ticket_id = t.id
		            ORDER BY u.name LIMIT 1
		        ), '')
		 FROM tickets t
		 LEFT JOIN columns c ON c.id = t.column_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1 AND t.team_id = $2 AND t.closed_at IS NULL
		   AND EXISTS (
		       SELECT 1 FROM ticket_links tl
		       WHERE tl.to_ticket_id = t.id AND tl.relation_type = 'blocks'
		   )
		 ORDER BY `+priorityOrder+`, t.number
		 LIMIT 10`,
		orgID, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DashboardBlockedTicket
	for rows.Next() {
		var r model.DashboardBlockedTicket
		if err := rows.Scan(&r.ID, &r.DisplayID, &r.Title, &r.Priority, &r.ColumnName, &r.AssigneeName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListMyOpenTicketsByTeam is like ListMyOpenTickets but scoped to a single team.
func (s *Store) ListMyOpenTicketsByTeam(ctx context.Context, orgID, userID, teamID uuid.UUID) ([]model.DashboardMyTicket, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT t.id,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title, t.priority, t.story_points,
		        COALESCE(c.name,
		            CASE
		                WHEN sp.status = 'active'   THEN 'In sprint'
		                WHEN sp.status = 'planning' THEN 'Planning'
		                ELSE 'Backlog'
		            END)
		 FROM tickets t
		 JOIN ticket_assignees ta ON ta.ticket_id = t.id
		 LEFT JOIN sprints sp ON sp.id = t.sprint_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 LEFT JOIN columns c ON c.id = t.column_id
		 WHERE t.org_id = $1 AND ta.user_id = $2 AND t.team_id = $3 AND t.closed_at IS NULL
		 ORDER BY
		     CASE WHEN sp.status = 'active'   THEN 0
		          WHEN sp.status = 'planning' THEN 1
		          ELSE 2 END,
		     `+priorityOrder+`,
		     t.number
		 LIMIT 20`,
		orgID, userID, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DashboardMyTicket
	for rows.Next() {
		var r model.DashboardMyTicket
		if err := rows.Scan(&r.ID, &r.DisplayID, &r.Title, &r.Priority, &r.StoryPoints, &r.ColumnName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRecentTeamActivity returns the most recent history entries for a single team's tickets.
func (s *Store) ListRecentTeamActivity(ctx context.Context, orgID, teamID uuid.UUID, limit int) ([]model.InboxEntry, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT h.id, h.ticket_id, h.actor_id, h.actor_name, h.field, h.old_value, h.new_value, h.created_at,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title
		 FROM ticket_history h
		 JOIN tickets t ON t.id = h.ticket_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1 AND t.team_id = $2
		 ORDER BY h.created_at DESC
		 LIMIT $3`,
		orgID, teamID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.InboxEntry
	for rows.Next() {
		var e model.InboxEntry
		if err := rows.Scan(
			&e.ID, &e.TicketID, &e.ActorID, &e.ActorName,
			&e.Field, &e.OldValue, &e.NewValue, &e.CreatedAt,
			&e.TicketDisplayID, &e.TicketTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListRecentOrgActivity returns the most recent history entries across all org tickets.
func (s *Store) ListRecentOrgActivity(ctx context.Context, orgID uuid.UUID, limit int) ([]model.InboxEntry, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT h.id, h.ticket_id, h.actor_id, h.actor_name, h.field, h.old_value, h.new_value, h.created_at,
		        COALESCE(tm.key || '-' || t.number::text, t.id::text),
		        t.title
		 FROM ticket_history h
		 JOIN tickets t ON t.id = h.ticket_id
		 LEFT JOIN teams tm ON tm.id = t.team_id
		 WHERE t.org_id = $1
		 ORDER BY h.created_at DESC
		 LIMIT $2`,
		orgID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.InboxEntry
	for rows.Next() {
		var e model.InboxEntry
		if err := rows.Scan(
			&e.ID, &e.TicketID, &e.ActorID, &e.ActorName,
			&e.Field, &e.OldValue, &e.NewValue, &e.CreatedAt,
			&e.TicketDisplayID, &e.TicketTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
