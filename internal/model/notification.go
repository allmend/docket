package model

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	UserID    uuid.UUID
	TicketID  *uuid.UUID
	ActorID   *uuid.UUID
	ActorName string
	Type      string // assigned | mentioned | comment
	ReadAt    *time.Time
	CreatedAt time.Time

	// denormalized for display
	TicketDisplayID string
	TicketTitle     string
}

func (n *Notification) IsRead() bool { return n.ReadAt != nil }
