package model

import "github.com/google/uuid"

// DashboardBlockedTicket is a compact view of a blocked sprint ticket for the dashboard.
type DashboardBlockedTicket struct {
	ID           uuid.UUID
	DisplayID    string
	Title        string
	Priority     string
	ColumnName   string
	AssigneeName string
}

// DashboardMyTicket is an active-sprint ticket assigned to the current user, for "My day".
type DashboardMyTicket struct {
	ID          uuid.UUID
	DisplayID   string
	Title       string
	Priority    string
	StoryPoints *float64
	ColumnName  string
}
