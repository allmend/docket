package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// linkCols is the SELECT list for ticket_links with denormalised display IDs.
const linkCols = `
	tl.id, tl.org_id, tl.from_ticket_id, tl.to_ticket_id, tl.relation_type, tl.created_at,
	COALESCE(ftm.key || '-' || ft.number::text, ft.id::text),
	ft.title,
	COALESCE(ttm.key || '-' || tt.number::text, tt.id::text),
	tt.title`

const linkJoins = `
	FROM ticket_links tl
	JOIN tickets ft  ON ft.id  = tl.from_ticket_id
	JOIN tickets tt  ON tt.id  = tl.to_ticket_id
	LEFT JOIN teams ftm ON ftm.id = ft.team_id
	LEFT JOIN teams ttm ON ttm.id = tt.team_id`

func scanLink(row interface{ Scan(dest ...any) error }, l *model.TicketLink) error {
	return row.Scan(
		&l.ID, &l.OrgID, &l.FromTicketID, &l.ToTicketID, &l.Relation, &l.CreatedAt,
		&l.FromDisplayID, &l.FromTitle,
		&l.ToDisplayID, &l.ToTitle,
	)
}

// ListLinks returns all links where the given ticket is either source or target,
// with the relation rewritten so it always reads from the given ticket's perspective.
func (s *Store) ListLinks(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.TicketLink, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+linkCols+linkJoins+`
		 WHERE tl.org_id = $1 AND (tl.from_ticket_id = $2 OR tl.to_ticket_id = $2)
		 ORDER BY tl.created_at`,
		orgID, ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []model.TicketLink
	for rows.Next() {
		var l model.TicketLink
		if err := scanLink(rows, &l); err != nil {
			return nil, err
		}
		// Rewrite inverse links so the template always reads "this ticket → other ticket".
		// e.g. if another ticket "blocks" ours, we surface it as "blocked by".
		if l.ToTicketID == ticketID {
			l.FromTicketID, l.ToTicketID = l.ToTicketID, l.FromTicketID
			l.FromDisplayID, l.ToDisplayID = l.ToDisplayID, l.FromDisplayID
			l.FromTitle, l.ToTitle = l.ToTitle, l.FromTitle
			switch l.Relation {
			case model.RelationBlocks:
				l.Relation = "blocked_by" // virtual — not stored, only for display
			}
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *Store) GetLink(ctx context.Context, orgID, linkID uuid.UUID) (*model.TicketLink, error) {
	var l model.TicketLink
	err := scanLink(s.replica.QueryRow(ctx,
		`SELECT `+linkCols+linkJoins+`
		 WHERE tl.org_id = $1 AND tl.id = $2`,
		orgID, linkID,
	), &l)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Store) CreateLink(ctx context.Context, orgID, fromTicketID, toTicketID uuid.UUID, relation model.RelationType) (*model.TicketLink, error) {
	var l model.TicketLink
	err := s.primary.QueryRow(ctx,
		`INSERT INTO ticket_links (org_id, from_ticket_id, to_ticket_id, relation_type)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, from_ticket_id, to_ticket_id, relation_type, created_at`,
		orgID, fromTicketID, toTicketID, relation,
	).Scan(&l.ID, &l.OrgID, &l.FromTicketID, &l.ToTicketID, &l.Relation, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Store) DeleteLink(ctx context.Context, orgID, linkID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM ticket_links WHERE org_id = $1 AND id = $2`,
		orgID, linkID,
	)
	return err
}

// ListBlockingLinksForDoneTickets returns "blocks" links where the blocker is in a
// Done column of the sprint, so callers can record history before clearing them.
func (s *Store) ListBlockingLinksForDoneTickets(ctx context.Context, orgID, sprintID uuid.UUID) ([]model.TicketLink, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+linkCols+linkJoins+`
		 JOIN tickets ft2 ON ft2.id = tl.from_ticket_id
		 JOIN columns  c  ON c.id   = ft2.column_id
		 WHERE tl.org_id = $1
		   AND tl.relation_type = 'blocks'
		   AND ft2.sprint_id = $2
		   AND LOWER(c.name) = 'done'`,
		orgID, sprintID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []model.TicketLink
	for rows.Next() {
		var l model.TicketLink
		if err := scanLink(rows, &l); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ClearBlockingLinksForDoneTickets removes "blocks" links where the blocker ticket
// is in a Done column of the given sprint. Called on sprint close so that tickets
// resolved within the sprint no longer block others going into the next sprint.
func (s *Store) ClearBlockingLinksForDoneTickets(ctx context.Context, orgID, sprintID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM ticket_links
		 WHERE org_id = $1
		   AND relation_type = 'blocks'
		   AND from_ticket_id IN (
		       SELECT t.id
		       FROM tickets t
		       JOIN columns c ON c.id = t.column_id
		       WHERE t.org_id = $1
		         AND t.sprint_id = $2
		         AND LOWER(c.name) = 'done'
		   )`,
		orgID, sprintID,
	)
	return err
}

// BulkGetBlockedBy returns a map of ticketID → blocker display ID for all tickets
// on a board that are blocked (have at least one inbound "blocks" link).
// Used by GetBoardView to annotate board cards without N+1 queries.
func (s *Store) BulkGetBlockedBy(ctx context.Context, orgID, boardID uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT tl.to_ticket_id,
		        COALESCE(btm.key || '-' || bt.number::text, bt.id::text)
		 FROM ticket_links tl
		 JOIN tickets t   ON t.id   = tl.to_ticket_id   AND t.board_id = $2
		 JOIN tickets bt  ON bt.id  = tl.from_ticket_id
		 LEFT JOIN teams btm ON btm.id = bt.team_id
		 WHERE tl.org_id = $1 AND tl.relation_type = 'blocks'`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var ticketID uuid.UUID
		var blockerDisplayID string
		if err := rows.Scan(&ticketID, &blockerDisplayID); err != nil {
			return nil, err
		}
		// Keep first blocker found if multiple.
		if _, exists := result[ticketID]; !exists {
			result[ticketID] = blockerDisplayID
		}
	}
	return result, rows.Err()
}
