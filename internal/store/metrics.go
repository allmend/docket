package store

import (
	"context"

	"github.com/google/uuid"
)

// TicketCountRow is one row from the per-column ticket count query.
type TicketCountRow struct {
	OrgSlug  string
	TeamKey  string
	Column   string
	Priority string
	Count    int64
}

// BacklogSizeRow is one row from the per-team backlog size query.
type BacklogSizeRow struct {
	OrgSlug string
	TeamKey string
	Count   int64
}

// BlockedCountRow is one row from the per-team blocked ticket count query.
type BlockedCountRow struct {
	OrgSlug string
	TeamKey string
	Count   int64
}

// SprintStatsRow holds snapshotted stats for one sprint.
type SprintStatsRow struct {
	OrgSlug          string
	TeamKey          string
	SprintName       string
	CommittedTickets int64
	CompletedTickets int64
	CommittedPoints  float64
	CompletedPoints  float64
}

// MetricsTicketCounts returns open ticket counts grouped by org/team/column/priority,
// scoped to a single org.
func (s *Store) MetricsTicketCounts(ctx context.Context, orgID uuid.UUID) ([]TicketCountRow, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT
			o.slug                                   AS org_slug,
			COALESCE(te.key, '')                     AS team_key,
			COALESCE(c.name, 'backlog')              AS column_name,
			COALESCE(NULLIF(t.priority, ''), 'none') AS priority,
			COUNT(*)                                 AS cnt
		FROM tickets t
		JOIN orgs o ON o.id = t.org_id
		JOIN boards b ON b.id = t.board_id
		LEFT JOIN teams te ON te.id = b.team_id
		LEFT JOIN columns c ON c.id = t.column_id
		WHERE t.org_id = $1
		  AND t.closed_at IS NULL
		GROUP BY o.slug, te.key, column_name, priority
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TicketCountRow
	for rows.Next() {
		var r TicketCountRow
		if err := rows.Scan(&r.OrgSlug, &r.TeamKey, &r.Column, &r.Priority, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MetricsBacklogSize returns the count of unassigned open tickets per team, scoped to a single org.
func (s *Store) MetricsBacklogSize(ctx context.Context, orgID uuid.UUID) ([]BacklogSizeRow, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT
			o.slug               AS org_slug,
			COALESCE(te.key, '') AS team_key,
			COUNT(*)             AS cnt
		FROM tickets t
		JOIN orgs o ON o.id = t.org_id
		JOIN boards b ON b.id = t.board_id
		LEFT JOIN teams te ON te.id = b.team_id
		WHERE t.org_id = $1
		  AND t.sprint_id IS NULL
		  AND t.closed_at IS NULL
		GROUP BY o.slug, te.key
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BacklogSizeRow
	for rows.Next() {
		var r BacklogSizeRow
		if err := rows.Scan(&r.OrgSlug, &r.TeamKey, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MetricsBlockedCount returns the count of open tickets with at least one inbound
// "blocks" link per team, scoped to a single org.
func (s *Store) MetricsBlockedCount(ctx context.Context, orgID uuid.UUID) ([]BlockedCountRow, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT
			o.slug               AS org_slug,
			COALESCE(te.key, '') AS team_key,
			COUNT(DISTINCT t.id) AS cnt
		FROM tickets t
		JOIN orgs o ON o.id = t.org_id
		JOIN boards b ON b.id = t.board_id
		LEFT JOIN teams te ON te.id = b.team_id
		WHERE t.org_id = $1
		  AND t.closed_at IS NULL
		  AND EXISTS (
		      SELECT 1 FROM ticket_links tl
		      WHERE tl.to_ticket_id = t.id
		        AND tl.relation_type = 'blocks'
		        AND tl.org_id = $1
		  )
		GROUP BY o.slug, te.key
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BlockedCountRow
	for rows.Next() {
		var r BlockedCountRow
		if err := rows.Scan(&r.OrgSlug, &r.TeamKey, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MetricsSprintStats returns snapshotted sprint stats for active and completed sprints,
// scoped to a single org.
func (s *Store) MetricsSprintStats(ctx context.Context, orgID uuid.UUID) ([]SprintStatsRow, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT
			o.slug               AS org_slug,
			COALESCE(te.key, '') AS team_key,
			sp.name              AS sprint_name,
			sp.committed_tickets,
			sp.completed_tickets,
			sp.committed_points,
			sp.completed_points
		FROM sprints sp
		JOIN boards b ON b.id = sp.board_id
		JOIN orgs o ON o.id = sp.org_id
		LEFT JOIN teams te ON te.id = b.team_id
		WHERE sp.org_id = $1
		  AND (
		    sp.status = 'active'
		    OR (sp.status = 'completed' AND sp.updated_at >= NOW() - INTERVAL '7 days')
		  )
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SprintStatsRow
	for rows.Next() {
		var r SprintStatsRow
		if err := rows.Scan(
			&r.OrgSlug, &r.TeamKey, &r.SprintName,
			&r.CommittedTickets, &r.CompletedTickets,
			&r.CommittedPoints, &r.CompletedPoints,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
