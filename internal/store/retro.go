package store

import (
	"context"
	"fmt"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

const retroBoardCols = `id, org_id, board_id, sprint_id, status, closed_at, created_at`

func scanRetroBoard(row interface{ Scan(...any) error }, rb *model.RetroBoard) error {
	return row.Scan(&rb.ID, &rb.OrgID, &rb.BoardID, &rb.SprintID, &rb.Status, &rb.ClosedAt, &rb.CreatedAt)
}

func (s *Store) GetRetroBoard(ctx context.Context, orgID, retroBoardID uuid.UUID) (*model.RetroBoard, error) {
	var rb model.RetroBoard
	err := scanRetroBoard(s.replica.QueryRow(ctx,
		`SELECT `+retroBoardCols+` FROM retro_boards WHERE org_id = $1 AND id = $2`,
		orgID, retroBoardID,
	), &rb)
	if err != nil {
		return nil, err
	}
	return &rb, nil
}

func (s *Store) GetOrCreateRetroBoard(ctx context.Context, orgID, boardID uuid.UUID, sprintID *uuid.UUID) (*model.RetroBoard, error) {
	var rb model.RetroBoard

	var row interface{ Scan(...any) error }
	if sprintID != nil {
		row = s.primary.QueryRow(ctx,
			`SELECT `+retroBoardCols+` FROM retro_boards WHERE org_id = $1 AND board_id = $2 AND sprint_id = $3`,
			orgID, boardID, sprintID,
		)
	} else {
		row = s.primary.QueryRow(ctx,
			`SELECT `+retroBoardCols+` FROM retro_boards WHERE org_id = $1 AND board_id = $2 AND sprint_id IS NULL
			 ORDER BY created_at DESC LIMIT 1`,
			orgID, boardID,
		)
	}

	if err := scanRetroBoard(row, &rb); err == nil {
		return &rb, nil
	}

	err := scanRetroBoard(s.primary.QueryRow(ctx,
		`INSERT INTO retro_boards (org_id, board_id, sprint_id)
		 VALUES ($1, $2, $3)
		 RETURNING `+retroBoardCols,
		orgID, boardID, sprintID,
	), &rb)
	if err != nil {
		return nil, fmt.Errorf("create retro board: %w", err)
	}
	return &rb, nil
}

func (s *Store) CloseRetroBoard(ctx context.Context, orgID, retroBoardID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE retro_boards SET status = 'closed', closed_at = NOW()
		 WHERE org_id = $1 AND id = $2`,
		orgID, retroBoardID,
	)
	return err
}

func (s *Store) ListRetroBoardsForBoard(ctx context.Context, orgID, boardID uuid.UUID) ([]model.RetroBoard, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT `+retroBoardCols+` FROM retro_boards
		 WHERE org_id = $1 AND board_id = $2
		 ORDER BY created_at DESC`,
		orgID, boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RetroBoard
	for rows.Next() {
		var rb model.RetroBoard
		if err := scanRetroBoard(rows, &rb); err != nil {
			return nil, err
		}
		out = append(out, rb)
	}
	return out, rows.Err()
}

type RetroBoardCounts struct {
	RetroBoardID     uuid.UUID
	WentWellCount    int
	DidntGoWellCount int
	ActionItemCount  int
}

func (s *Store) BulkCountRetroCards(ctx context.Context, orgID uuid.UUID, retroBoardIDs []uuid.UUID) (map[uuid.UUID]RetroBoardCounts, error) {
	if len(retroBoardIDs) == 0 {
		return nil, nil
	}
	rows, err := s.replica.Query(ctx,
		`SELECT retro_board_id, column_name, COUNT(*)
		 FROM retro_cards
		 WHERE org_id = $1 AND retro_board_id = ANY($2)
		 GROUP BY retro_board_id, column_name`,
		orgID, retroBoardIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]RetroBoardCounts)
	for rows.Next() {
		var id uuid.UUID
		var col string
		var n int
		if err := rows.Scan(&id, &col, &n); err != nil {
			return nil, err
		}
		c := out[id]
		c.RetroBoardID = id
		switch model.RetroColumn(col) {
		case model.RetroWentWell:
			c.WentWellCount = n
		case model.RetroDidntGoWell:
			c.DidntGoWellCount = n
		case model.RetroActionItem:
			c.ActionItemCount = n
		}
		out[id] = c
	}
	return out, rows.Err()
}

func (s *Store) ListRetroCards(ctx context.Context, orgID, retroBoardID uuid.UUID) ([]model.RetroCard, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT rc.id, rc.org_id, rc.retro_board_id, rc.column_name, rc.body,
		        rc.author_id, rc.owner_id, COALESCE(rc.owner_name, ''), rc.ticket_id,
		        COALESCE(tm.key || '-' || t.number::text, '')
		 FROM retro_cards rc
		 LEFT JOIN tickets t  ON t.id = rc.ticket_id
		 LEFT JOIN teams   tm ON tm.id = t.team_id
		 WHERE rc.org_id = $1 AND rc.retro_board_id = $2
		 ORDER BY rc.created_at`,
		orgID, retroBoardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []model.RetroCard
	for rows.Next() {
		var c model.RetroCard
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.RetroBoardID, &c.Column, &c.Body,
			&c.AuthorID, &c.OwnerID, &c.OwnerName, &c.TicketID, &c.TicketDisplay,
		); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (s *Store) CreateRetroCard(ctx context.Context, orgID, retroBoardID, authorID uuid.UUID, column model.RetroColumn, body string) (*model.RetroCard, error) {
	var c model.RetroCard
	err := s.primary.QueryRow(ctx,
		`INSERT INTO retro_cards (org_id, retro_board_id, author_id, column_name, body)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, org_id, retro_board_id, column_name, body, author_id, owner_id, COALESCE(owner_name,''), ticket_id`,
		orgID, retroBoardID, authorID, column, body,
	).Scan(&c.ID, &c.OrgID, &c.RetroBoardID, &c.Column, &c.Body, &c.AuthorID, &c.OwnerID, &c.OwnerName, &c.TicketID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) GetRetroCard(ctx context.Context, orgID, cardID uuid.UUID) (*model.RetroCard, error) {
	var c model.RetroCard
	err := s.primary.QueryRow(ctx,
		`SELECT id, org_id, retro_board_id, column_name, body, author_id, owner_id, COALESCE(owner_name,''), ticket_id
		 FROM retro_cards WHERE org_id = $1 AND id = $2`,
		orgID, cardID,
	).Scan(&c.ID, &c.OrgID, &c.RetroBoardID, &c.Column, &c.Body, &c.AuthorID, &c.OwnerID, &c.OwnerName, &c.TicketID)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) DeleteRetroCard(ctx context.Context, orgID, cardID, authorID uuid.UUID) error {
	tag, err := s.primary.Exec(ctx,
		`DELETE FROM retro_cards WHERE org_id = $1 AND id = $2 AND author_id = $3`,
		orgID, cardID, authorID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("card not found or not yours")
	}
	return nil
}

func (s *Store) AssignRetroCardOwner(ctx context.Context, orgID, cardID, ownerID, ticketID uuid.UUID, ownerName string) error {
	_, err := s.primary.Exec(ctx,
		`UPDATE retro_cards SET owner_id = $3, owner_name = $4, ticket_id = $5
		 WHERE org_id = $1 AND id = $2`,
		orgID, cardID, ownerID, ownerName, ticketID,
	)
	return err
}
