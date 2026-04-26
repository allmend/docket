package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

// ListDodItems returns all DoD items for a board ordered by position.
func (s *Store) ListDodItems(ctx context.Context, orgID, boardID uuid.UUID) ([]model.DodItem, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT id, org_id, board_id, text, position, created_at
		FROM dod_items
		WHERE org_id = $1 AND board_id = $2
		ORDER BY position, created_at
	`, orgID, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DodItem
	for rows.Next() {
		var d model.DodItem
		if err := rows.Scan(&d.ID, &d.OrgID, &d.BoardID, &d.Text, &d.Position, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateDodItem appends a new item at the end of a board's DoD list.
func (s *Store) CreateDodItem(ctx context.Context, orgID, boardID uuid.UUID, text string) (*model.DodItem, error) {
	var d model.DodItem
	err := s.primary.QueryRow(ctx, `
		INSERT INTO dod_items (org_id, board_id, text, position)
		VALUES ($1, $2, $3, (
			SELECT COALESCE(MAX(position), 0) + 1000
			FROM dod_items WHERE org_id = $1 AND board_id = $2
		))
		RETURNING id, org_id, board_id, text, position, created_at
	`, orgID, boardID, text).Scan(&d.ID, &d.OrgID, &d.BoardID, &d.Text, &d.Position, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// DeleteDodItem removes a DoD item (cascades to dod_checks).
func (s *Store) DeleteDodItem(ctx context.Context, orgID, itemID uuid.UUID) error {
	_, err := s.primary.Exec(ctx, `
		DELETE FROM dod_items WHERE org_id = $1 AND id = $2
	`, orgID, itemID)
	return err
}

// GetTicketDod returns all DoD items for a board annotated with the ticket's check state.
func (s *Store) GetTicketDod(ctx context.Context, orgID, boardID, ticketID uuid.UUID) ([]model.DodItemWithCheck, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT d.id, d.org_id, d.board_id, d.text, d.position, d.created_at,
		       (dc.dod_item_id IS NOT NULL) AS checked
		FROM dod_items d
		LEFT JOIN dod_checks dc ON dc.dod_item_id = d.id AND dc.ticket_id = $3 AND dc.org_id = $1
		WHERE d.org_id = $1 AND d.board_id = $2
		ORDER BY d.position, d.created_at
	`, orgID, boardID, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DodItemWithCheck
	for rows.Next() {
		var d model.DodItemWithCheck
		if err := rows.Scan(&d.ID, &d.OrgID, &d.BoardID, &d.Text, &d.Position, &d.CreatedAt, &d.Checked); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ToggleDodCheck checks or unchecks a DoD item for a ticket.
func (s *Store) ToggleDodCheck(ctx context.Context, orgID, ticketID, itemID uuid.UUID, checked bool) error {
	if checked {
		_, err := s.primary.Exec(ctx, `
			INSERT INTO dod_checks (dod_item_id, ticket_id, org_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (dod_item_id, ticket_id) DO NOTHING
		`, itemID, ticketID, orgID)
		return err
	}
	_, err := s.primary.Exec(ctx, `
		DELETE FROM dod_checks WHERE dod_item_id = $1 AND ticket_id = $2 AND org_id = $3
	`, itemID, ticketID, orgID)
	return err
}
