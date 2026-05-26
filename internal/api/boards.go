package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)


// boardViewData expands a BoardView struct into a template data map that also
// includes NavTeams so the sidebar can render the team list.
// SprintID is the active sprint ID (or first planning sprint ID) for drag-from-backlog support.
func (h *Handler) boardViewData(r *http.Request, view *model.BoardView) map[string]any {
	sprintID := ""
	if view.ActiveSprint != nil {
		sprintID = view.ActiveSprint.ID.String()
	} else {
		for _, s := range view.Sprints {
			if s.Status == model.SprintStatusPlanning {
				sprintID = s.ID.String()
				break
			}
		}
	}
	return h.pageData(r, map[string]any{
		"Board":               view.Board,
		"Team":                view.Team,
		"Columns":             view.Columns,
		"ActiveSprint":        view.ActiveSprint,
		"Sprints":             view.Sprints,
		"BacklogCount":        view.BacklogCount,
		"FirstColumnID":       view.FirstColumnID,
		"SprintID":            sprintID,
		"ActiveSprintSection": view.ActiveSprintSection,
	})
}

func (h *Handler) BoardView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}

	view, err := h.boards.GetBoardView(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}

	h.render(w, "board.html", h.boardViewData(r, view))
}

func (h *Handler) UpdateBoard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	_, err = h.boards.UpdateBoard(r.Context(), orgID, boardID, r.FormValue("name"), r.FormValue("description"))
	if err != nil {
		http.Error(w, "failed to update board", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}

	board, err := h.boards.GetBoard(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}

	if err := h.boards.DeleteBoard(r.Context(), orgID, boardID); err != nil {
		http.Error(w, "failed to delete board", http.StatusInternalServerError)
		return
	}

	redirectTo := "/teams"
	if board.TeamID != nil {
		redirectTo = "/teams/" + board.TeamID.String()
	}
	w.Header().Set("HX-Redirect", redirectTo)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	_, err = h.boards.AddColumn(r.Context(), orgID, boardID, name)
	if err != nil {
		http.Error(w, "failed to create column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RenameColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	columnID, err := uuid.Parse(chi.URLParam(r, "columnID"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if _, err = h.boards.RenameColumn(r.Context(), orgID, columnID, name); err != nil {
		http.Error(w, "failed to rename column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	columnID, err := uuid.Parse(chi.URLParam(r, "columnID"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}

	if err := h.boards.DeleteColumn(r.Context(), orgID, columnID); err != nil {
		http.Error(w, "failed to delete column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

// --- Sprint handlers ---

func (h *Handler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	var startDate, endDate *time.Time
	if v := r.FormValue("start_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			startDate = &t
		}
	}
	if v := r.FormValue("end_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			endDate = &t
		}
	}
	_, err = h.boards.CreateSprint(r.Context(), orgID, boardID, userID, name, r.FormValue("goal"), startDate, endDate)
	if err != nil {
		http.Error(w, "failed to create sprint", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String(), http.StatusSeeOther)
}

func (h *Handler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var startDate, endDate *time.Time
	if v := r.FormValue("start_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			startDate = &t
		}
	}
	if v := r.FormValue("end_date"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			endDate = &t
		}
	}
	_, err = h.boards.UpdateSprint(r.Context(), orgID, sprintID, r.FormValue("name"), r.FormValue("goal"), startDate, endDate)
	if err != nil {
		http.Error(w, "failed to update sprint", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "sprintUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) StartSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	if _, err := h.boards.StartSprint(r.Context(), orgID, sprintID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String(), http.StatusSeeOther)
}

func (h *Handler) CloseSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	userID := service.UserIDFromContext(r.Context())
	if _, err := h.boards.CloseSprint(r.Context(), orgID, sprintID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String(), http.StatusSeeOther)
}

func (h *Handler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	if err := h.boards.DeleteSprint(r.Context(), orgID, sprintID); err != nil {
		http.Error(w, "failed to delete sprint", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String(), http.StatusSeeOther)
}

func (h *Handler) AssignTicketToSprint(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
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
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	for _, raw := range r.Form["ticket_ids"] {
		ticketID, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		_ = h.boards.AssignTicketToSprint(r.Context(), orgID, ticketID, &sprintID)
	}
	http.Redirect(w, r, "/boards/"+boardID.String()+"/backlog", http.StatusSeeOther)
}

func (h *Handler) BoardRoadmap(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	board, err := h.boards.GetBoard(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}
	var team *model.Team
	if board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
	}
	sprints, err := h.boards.GetRoadmap(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "roadmap not found", http.StatusInternalServerError)
		return
	}
	h.render(w, "roadmap.html", h.pageData(r, map[string]any{
		"Board":   board,
		"Team":    team,
		"Sprints": sprints,
	}))
}

func (h *Handler) BoardBacklog(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	view, err := h.boards.GetBacklog(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "backlog not found", http.StatusNotFound)
		return
	}
	data := h.boardViewData(r, view)
	if strings.Contains(r.Header.Get("HX-Current-URL"), "/refinement") {
		data["InitRefineMode"] = true
	}
	h.render(w, "backlog.html", data)
}

func (h *Handler) BoardRefinement(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	view, err := h.boards.GetBacklog(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "backlog not found", http.StatusNotFound)
		return
	}
	data := h.boardViewData(r, view)
	data["InitRefineMode"] = true
	h.render(w, "backlog.html", data)
}

func (h *Handler) BoardDailyScrum(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	filters := model.DailyScrumFilters{
		Q:                r.URL.Query().Get("q"),
		AssigneeIDs:      r.URL.Query()["assignee_id"],
		TagIDs:           r.URL.Query()["tag_id"],
		Priorities:       r.URL.Query()["priority"],
		FilterUnassigned: r.URL.Query().Get("unassigned") == "1",
	}
	view, err := h.boards.GetDailyScrumView(r.Context(), orgID, boardID, filters)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.render(w, "daily.html", h.pageData(r, map[string]any{
		"Board":        view.Board,
		"Team":         view.Team,
		"ActiveSprint": view.ActiveSprint,
		"Groups":       view.Groups,
		"Unassigned":   view.Unassigned,
		"AllAssignees": view.AllAssignees,
		"AllTags":      view.AllTags,
		"Filters":      view.Filters,
	}))
}

func (h *Handler) BoardDailyScrumTickets(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	filters := model.DailyScrumFilters{
		Q:                r.URL.Query().Get("q"),
		AssigneeIDs:      r.URL.Query()["assignee_id"],
		TagIDs:           r.URL.Query()["tag_id"],
		Priorities:       r.URL.Query()["priority"],
		FilterUnassigned: r.URL.Query().Get("unassigned") == "1",
	}
	view, err := h.boards.GetDailyScrumView(r.Context(), orgID, boardID, filters)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.render(w, "daily-tickets.html", map[string]any{
		"Board":      view.Board,
		"Groups":     view.Groups,
		"Unassigned": view.Unassigned,
		"Filters":    view.Filters,
	})
}

// --- Tag handlers ---

func filterUnusedTags(all, applied []model.Tag) []model.Tag {
	used := make(map[uuid.UUID]bool, len(applied))
	for _, t := range applied {
		used[t.ID] = true
	}
	out := make([]model.Tag, 0, len(all))
	for _, t := range all {
		if !used[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

func (h *Handler) BoardTagsJSON(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	type tagResult struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	out := make([]tagResult, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagResult{ID: t.ID.String(), Name: t.Name, Color: t.Color})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) BoardTagsPanel(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	color := r.FormValue("color")
	if name == "" || color == "" {
		http.Error(w, "name and color required", http.StatusBadRequest)
		return
	}
	if _, err := h.boards.CreateTag(r.Context(), orgID, boardID, name, color); err != nil {
		http.Error(w, "failed to create tag", http.StatusInternalServerError)
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		http.Error(w, "invalid tag ID", http.StatusBadRequest)
		return
	}
	_ = h.boards.DeleteTag(r.Context(), orgID, tagID)
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) AddTagToTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tagID, err := uuid.Parse(r.FormValue("tag_id"))
	if err != nil {
		http.Error(w, "invalid tag ID", http.StatusBadRequest)
		return
	}
	_ = h.boards.AddTagToTicket(r.Context(), orgID, ticketID, tagID)
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticketID)
	var boardTags []model.Tag
	if ticket != nil {
		all, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
		boardTags = filterUnusedTags(all, tags)
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-tags.html", map[string]any{
		"Ticket":    ticket,
		"Tags":      tags,
		"BoardTags": boardTags,
	})
}

func (h *Handler) RemoveTagFromTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		http.Error(w, "invalid tag ID", http.StatusBadRequest)
		return
	}
	_ = h.boards.RemoveTagFromTicket(r.Context(), orgID, ticketID, tagID)
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticketID)
	var boardTags []model.Tag
	if ticket != nil {
		all, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
		boardTags = filterUnusedTags(all, tags)
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-tags.html", map[string]any{
		"Ticket":    ticket,
		"Tags":      tags,
		"BoardTags": boardTags,
	})
}

// BoardDodPanel renders the DoD management panel for a board.
func (h *Handler) BoardDodPanel(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	items, _ := h.boards.ListDodItems(r.Context(), orgID, boardID)
	h.render(w, "board-dod-panel.html", map[string]any{
		"BoardID": boardID,
		"Items":   items,
	})
}

// CreateDodItem adds a new DoD item to a board.
func (h *Handler) CreateDodItem(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	if _, err := h.boards.CreateDodItem(r.Context(), orgID, boardID, text); err != nil {
		http.Error(w, "failed to create item", http.StatusInternalServerError)
		return
	}
	items, _ := h.boards.ListDodItems(r.Context(), orgID, boardID)
	h.render(w, "board-dod-panel.html", map[string]any{
		"BoardID": boardID,
		"Items":   items,
	})
}

// DeleteDodItem removes a DoD item from a board.
func (h *Handler) DeleteDodItem(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		http.Error(w, "invalid item ID", http.StatusBadRequest)
		return
	}
	_ = h.boards.DeleteDodItem(r.Context(), orgID, itemID)
	items, _ := h.boards.ListDodItems(r.Context(), orgID, boardID)
	h.render(w, "board-dod-panel.html", map[string]any{
		"BoardID": boardID,
		"Items":   items,
	})
}

// TicketDodPartial renders the DoD checklist for a ticket (HTMX swap target).
func (h *Handler) TicketDodPartial(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	if err != nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}
	items, _ := h.boards.GetTicketDod(r.Context(), orgID, ticket.BoardID, ticketID)
	h.render(w, "ticket-dod-partial.html", map[string]any{
		"TicketID": ticketID,
		"BoardID":  ticket.BoardID,
		"Items":    items,
		"IsClosed": ticket.ClosedAt != nil,
	})
}

// ToggleDodCheck checks or unchecks a DoD item for a ticket.
func (h *Handler) ToggleDodCheck(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		http.Error(w, "invalid item ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	checked := r.FormValue("checked") == "true"
	if err := h.boards.ToggleDodCheck(r.Context(), orgID, ticketID, itemID, checked); err != nil {
		http.Error(w, "failed to toggle check", http.StatusInternalServerError)
		return
	}
	ticket, err := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	if err != nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}
	items, _ := h.boards.GetTicketDod(r.Context(), orgID, ticket.BoardID, ticketID)
	h.render(w, "ticket-dod-partial.html", map[string]any{
		"TicketID": ticketID,
		"BoardID":  ticket.BoardID,
		"Items":    items,
		"IsClosed": ticket.ClosedAt != nil,
	})
}

// SprintCapacityPartial renders the capacity section for one sprint (HTMX swap target).
func (h *Handler) SprintCapacityPartial(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
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
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
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

