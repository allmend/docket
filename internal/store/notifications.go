package store

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

func (s *Store) CreateNotification(ctx context.Context, orgID, userID uuid.UUID, ticketID *uuid.UUID, actorID *uuid.UUID, actorName, notifType string) error {
	_, err := s.primary.Exec(ctx, `
		INSERT INTO notifications (org_id, user_id, ticket_id, actor_id, actor_name, type)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		orgID, userID, ticketID, actorID, actorName, notifType,
	)
	return err
}

func (s *Store) ListNotificationsForUser(ctx context.Context, orgID, userID uuid.UUID, limit int) ([]model.Notification, error) {
	rows, err := s.replica.Query(ctx, `
		SELECT n.id, n.org_id, n.user_id, n.ticket_id, n.actor_id, n.actor_name,
		       n.type, n.read_at, n.created_at,
		       COALESCE(tm.key || '-' || t.number::text, ''), COALESCE(t.title, '')
		FROM notifications n
		LEFT JOIN tickets t  ON t.id  = n.ticket_id
		LEFT JOIN teams   tm ON tm.id = t.team_id
		WHERE n.org_id = $1 AND n.user_id = $2
		ORDER BY n.created_at DESC
		LIMIT $3`,
		orgID, userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(
			&n.ID, &n.OrgID, &n.UserID, &n.TicketID, &n.ActorID, &n.ActorName,
			&n.Type, &n.ReadAt, &n.CreatedAt,
			&n.TicketDisplayID, &n.TicketTitle,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) UnreadCount(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	var count int
	err := s.replica.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications
		WHERE org_id = $1 AND user_id = $2 AND read_at IS NULL`,
		orgID, userID,
	).Scan(&count)
	return count, err
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, orgID, userID uuid.UUID) error {
	_, err := s.primary.Exec(ctx, `
		UPDATE notifications SET read_at = NOW()
		WHERE org_id = $1 AND user_id = $2 AND read_at IS NULL`,
		orgID, userID,
	)
	return err
}
