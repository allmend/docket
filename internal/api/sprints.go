package api

import (
	"fmt"
	"net/http"

	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// nextSprintName numbers a new draft after the sprints that already exist.
func (h *Handler) nextSprintName(r *http.Request, orgID, boardID uuid.UUID) string {
	existing, err := h.boards.ListSprints(r.Context(), orgID, boardID)
	if err != nil {
		return "Sprint 1"
	}
	return fmt.Sprintf("Sprint %d", len(existing)+1)
}

// --- Sprint handlers ---

func (h *Handler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	name := r.FormValue("name")
	if name == "" {
		name = h.nextSprintName(r, orgID, boardID)
	}
	_, err := h.boards.CreateSprint(r.Context(), orgID, boardID, userID, name, r.FormValue("goal"), formDate(r, "start_date"), formDate(r, "end_date"))
	if err != nil {
		http.Error(w, "failed to create sprint", http.StatusInternalServerError)
		return
	}
	h.redirectToPlanning(w, r, orgID, boardID)
}

func (h *Handler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	_, err := h.boards.UpdateSprint(r.Context(), orgID, sprintID, r.FormValue("name"), r.FormValue("goal"), formDate(r, "start_date"), formDate(r, "end_date"))
	if err != nil {
		http.Error(w, "failed to update sprint", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "sprintUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) StartSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	if _, err := h.boards.StartSprint(r.Context(), orgID, sprintID, formDate(r, "start_date"), formDate(r, "end_date")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.redirectToBoard(w, r, orgID, boardID)
}

func (h *Handler) CloseSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if _, err := h.boards.CloseSprint(r.Context(), orgID, sprintID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.redirectToBoard(w, r, orgID, boardID)
}

func (h *Handler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	if err := h.boards.DeleteSprint(r.Context(), orgID, sprintID); err != nil {
		http.Error(w, "failed to delete sprint", http.StatusInternalServerError)
		return
	}
	// Land on the backlog, not the board: the board would bounce straight back
	// to planning and auto-create a fresh draft, making cancel a no-op.
	h.redirectToBacklog(w, r, orgID, boardID)
}

func (h *Handler) AssignTicketToSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	var sprintID *uuid.UUID
	if v := r.FormValue("sprint_id"); v != "" && v != "backlog" {
		id, err := uuid.Parse(v)
		if err != nil {
			http.Error(w, "invalid sprint ID", http.StatusBadRequest)
			return
		}
		sprintID = &id
	}
	if err := h.boards.AssignTicketToSprint(r.Context(), orgID, ticketID, sprintID); err != nil {
		http.Error(w, "failed to assign ticket", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", `{"backlogUpdated":true,"boardUpdated":true}`)
	w.WriteHeader(http.StatusNoContent)
}

// AssignTicketsToSprint bulk-assigns selected backlog tickets to a sprint.
// Called from the backlog page "Move selected to Sprint N" button.
func (h *Handler) AssignTicketsToSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	for _, raw := range r.Form["ticket_ids"] {
		ticketID, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		_ = h.boards.AssignTicketToSprint(r.Context(), orgID, ticketID, &sprintID)
	}
	h.redirectToBacklog(w, r, orgID, boardID)
}

// SprintCapacityPartial renders the capacity section for one sprint (HTMX swap target).
func (h *Handler) SprintCapacityPartial(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	cap, err := h.boards.GetSprintCapacity(r.Context(), orgID, boardID, sprintID)
	if err != nil {
		http.Error(w, "capacity not found", http.StatusInternalServerError)
		return
	}
	sprint, err := h.boards.GetSprint(r.Context(), orgID, sprintID)
	if err != nil {
		http.Error(w, "sprint not found", http.StatusNotFound)
		return
	}
	h.render(w, "sprint-capacity-partial.html", map[string]any{
		"Capacity": cap,
		"Sprint":   sprint,
		"BoardID":  boardID,
	})
}

// UpdateMemberCapacity sets one member's focus_pct for a sprint.
func (h *Handler) UpdateMemberCapacity(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	userID, ok := pathUUID(w, r, "userID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	var focusPct int
	if _, err := fmt.Sscanf(r.FormValue("focus_pct"), "%d", &focusPct); err != nil {
		http.Error(w, "invalid focus_pct", http.StatusBadRequest)
		return
	}
	if err := h.boards.SetMemberCapacity(r.Context(), orgID, sprintID, userID, focusPct); err != nil {
		http.Error(w, "failed to update capacity", http.StatusInternalServerError)
		return
	}
	cap, err := h.boards.GetSprintCapacity(r.Context(), orgID, boardID, sprintID)
	if err != nil {
		http.Error(w, "capacity not found", http.StatusInternalServerError)
		return
	}
	sprint, err := h.boards.GetSprint(r.Context(), orgID, sprintID)
	if err != nil {
		http.Error(w, "sprint not found", http.StatusNotFound)
		return
	}
	h.render(w, "sprint-capacity-partial.html", map[string]any{
		"Capacity": cap,
		"Sprint":   sprint,
		"BoardID":  boardID,
	})
}
