package service

import (
	"context"

	"github.com/allmend/docket/internal/model"
	"github.com/google/uuid"
)

type notificationStore interface {
	CreateNotification(ctx context.Context, orgID, userID uuid.UUID, ticketID *uuid.UUID, actorID *uuid.UUID, actorName, notifType string) error
	ListNotificationsForUser(ctx context.Context, orgID, userID uuid.UUID, limit int) ([]model.Notification, error)
	UnreadCount(ctx context.Context, orgID, userID uuid.UUID) (int, error)
	MarkAllNotificationsRead(ctx context.Context, orgID, userID uuid.UUID) error
}

type NotificationService struct {
	store notificationStore
}

func NewNotificationService(store notificationStore) *NotificationService {
	return &NotificationService{store: store}
}

func (s *NotificationService) Notify(ctx context.Context, orgID, userID uuid.UUID, ticketID *uuid.UUID, actorID *uuid.UUID, actorName, notifType string) {
	// best-effort — never block the caller on notification failure
	_ = s.store.CreateNotification(ctx, orgID, userID, ticketID, actorID, actorName, notifType)
}

func (s *NotificationService) List(ctx context.Context, orgID, userID uuid.UUID) ([]model.Notification, error) {
	return s.store.ListNotificationsForUser(ctx, orgID, userID, 50)
}

func (s *NotificationService) UnreadCount(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	return s.store.UnreadCount(ctx, orgID, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.store.MarkAllNotificationsRead(ctx, orgID, userID)
}
