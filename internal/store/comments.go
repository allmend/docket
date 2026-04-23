package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func (s *Store) ListComments(ctx context.Context, orgID, ticketID uuid.UUID) ([]model.Comment, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT c.id, c.org_id, c.ticket_id, c.author_id, u.name, c.body, c.edited, c.created_at, c.updated_at
		 FROM ticket_comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.org_id = $1 AND c.ticket_id = $2
		 ORDER BY c.created_at ASC`,
		orgID, ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.OrgID, &c.TicketID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Edited, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Store) CreateComment(ctx context.Context, orgID, ticketID, authorID uuid.UUID, body string) (*model.Comment, error) {
	var c model.Comment
	err := s.primary.QueryRow(ctx,
		`INSERT INTO ticket_comments (org_id, ticket_id, author_id, body)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, ticket_id, author_id,
		   (SELECT name FROM users WHERE id = $3),
		   body, edited, created_at, updated_at`,
		orgID, ticketID, authorID, body,
	).Scan(&c.ID, &c.OrgID, &c.TicketID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Edited, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Store) GetComment(ctx context.Context, orgID, commentID uuid.UUID) (*model.Comment, error) {
	var c model.Comment
	err := s.replica.QueryRow(ctx,
		`SELECT c.id, c.org_id, c.ticket_id, c.author_id, u.name, c.body, c.edited, c.created_at, c.updated_at
		 FROM ticket_comments c
		 JOIN users u ON u.id = c.author_id
		 WHERE c.org_id = $1 AND c.id = $2`,
		orgID, commentID,
	).Scan(&c.ID, &c.OrgID, &c.TicketID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Edited, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateComment(ctx context.Context, orgID, commentID uuid.UUID, body string) (*model.Comment, error) {
	var c model.Comment
	err := s.primary.QueryRow(ctx,
		`UPDATE ticket_comments SET body = $3, edited = TRUE, updated_at = NOW()
		 WHERE org_id = $1 AND id = $2
		 RETURNING id, org_id, ticket_id, author_id,
		   (SELECT name FROM users WHERE id = author_id),
		   body, edited, created_at, updated_at`,
		orgID, commentID, body,
	).Scan(&c.ID, &c.OrgID, &c.TicketID, &c.AuthorID, &c.AuthorName, &c.Body, &c.Edited, &c.CreatedAt, &c.UpdatedAt)
	return &c, err
}

func (s *Store) DeleteComment(ctx context.Context, orgID, commentID uuid.UUID) error {
	_, err := s.primary.Exec(ctx,
		`DELETE FROM ticket_comments WHERE org_id = $1 AND id = $2`,
		orgID, commentID,
	)
	return err
}

func (s *Store) AppendHistory(ctx context.Context, ticketID, actorID uuid.UUID, actorName, field, oldValue, newValue string) error {
	_, err := s.primary.Exec(ctx,
		`INSERT INTO ticket_history (ticket_id, actor_id, actor_name, field, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		ticketID, actorID, actorName, field, oldValue, newValue,
	)
	return err
}

func (s *Store) ListHistory(ctx context.Context, ticketID uuid.UUID) ([]model.HistoryEntry, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT id, ticket_id, actor_id, actor_name, field, old_value, new_value, created_at
		 FROM ticket_history WHERE ticket_id = $1 ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.HistoryEntry
	for rows.Next() {
		var e model.HistoryEntry
		if err := rows.Scan(&e.ID, &e.TicketID, &e.ActorID, &e.ActorName, &e.Field, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
