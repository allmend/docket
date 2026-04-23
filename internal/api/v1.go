package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// V1Routes mounts the versioned public JSON API under /api/v1.
//
// URL conventions:
//   - Teams identified by key:         /api/v1/teams/{key}
//   - Tickets identified by KEY-N ref: /api/v1/tickets/{ref}  (e.g. ENG-42)
//
// All mutations accept JSON bodies. All responses are JSON.
func (h *Handler) V1Routes(r chi.Router) {
	// Teams
	r.Get("/teams", h.v1ListTeams)
	r.Post("/teams", h.v1CreateTeam)
	r.Get("/teams/{key}", h.v1GetTeam)
	r.Put("/teams/{key}", h.v1UpdateTeam)
	r.Delete("/teams/{key}", h.v1DeleteTeam)

	// Tickets nested under team key
	r.Get("/teams/{key}/tickets", h.v1ListTickets)
	r.Post("/teams/{key}/tickets", h.v1CreateTicket)

	// Ticket by ref (KEY-N)
	r.Get("/tickets/{ref}", h.v1GetTicket)
	r.Put("/tickets/{ref}", h.v1UpdateTicket)
	r.Delete("/tickets/{ref}", h.v1DeleteTicket)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func apiError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// parseRef splits "ENG-42" into ("ENG", 42). Returns an error on bad format.
func parseRef(ref string) (key string, number int, err error) {
	i := strings.LastIndex(ref, "-")
	if i <= 0 || i == len(ref)-1 {
		return "", 0, fmt.Errorf("invalid ticket ref %q: expected KEY-N", ref)
	}
	n, err := strconv.Atoi(ref[i+1:])
	if err != nil || n <= 0 {
		return "", 0, fmt.Errorf("invalid ticket ref %q: number must be a positive integer", ref)
	}
	return strings.ToUpper(ref[:i]), n, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// --- team handlers ---

func (h *Handler) v1ListTeams(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teams, err := h.teams.ListTeams(r.Context(), orgID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

func (h *Handler) v1CreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	var body struct {
		Name        string `json:"name"`
		Key         string `json:"key"`
		Description string `json:"description"`
		Mode        string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Name == "" || body.Key == "" {
		apiError(w, http.StatusBadRequest, "name and key are required")
		return
	}

	mode := model.BoardMode(body.Mode)
	if mode != model.BoardModeKanban && mode != model.BoardModeScrum && mode != model.BoardModeBlank {
		mode = model.BoardModeKanban
	}

	team, _, err := h.teams.CreateTeam(r.Context(), orgID, userID, body.Name, body.Key, body.Description, mode)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to create team")
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

func (h *Handler) v1GetTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key := strings.ToUpper(chi.URLParam(r, "key"))

	team, err := h.teams.GetTeamByKey(r.Context(), orgID, key)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("team %q not found", key))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get team")
		return
	}
	writeJSON(w, http.StatusOK, team)
}

func (h *Handler) v1UpdateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key := strings.ToUpper(chi.URLParam(r, "key"))

	team, err := h.teams.GetTeamByKey(r.Context(), orgID, key)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("team %q not found", key))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get team")
		return
	}

	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Name == "" {
		body.Name = team.Name
	}

	updated, err := h.teams.UpdateTeam(r.Context(), orgID, team.ID, body.Name, body.Description)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to update team")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) v1DeleteTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key := strings.ToUpper(chi.URLParam(r, "key"))

	team, err := h.teams.GetTeamByKey(r.Context(), orgID, key)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("team %q not found", key))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get team")
		return
	}

	if err := h.teams.DeleteTeam(r.Context(), orgID, team.ID); err != nil {
		apiError(w, http.StatusInternalServerError, "failed to delete team")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- ticket handlers ---

func (h *Handler) v1ListTickets(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key := strings.ToUpper(chi.URLParam(r, "key"))

	team, err := h.teams.GetTeamByKey(r.Context(), orgID, key)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("team %q not found", key))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get team")
		return
	}

	tickets, err := h.tickets.ListByTeam(r.Context(), orgID, team.ID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}
	writeJSON(w, http.StatusOK, tickets)
}

func (h *Handler) v1CreateTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	key := strings.ToUpper(chi.URLParam(r, "key"))

	team, err := h.teams.GetTeamByKey(r.Context(), orgID, key)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("team %q not found", key))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get team")
		return
	}

	var body struct {
		BoardID  string `json:"board_id"`
		ColumnID string `json:"column_id"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		Priority string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" || body.BoardID == "" || body.ColumnID == "" {
		apiError(w, http.StatusBadRequest, "title, board_id, and column_id are required")
		return
	}

	boardID, err := uuid.Parse(body.BoardID)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid board_id")
		return
	}
	columnID, err := uuid.Parse(body.ColumnID)
	if err != nil {
		apiError(w, http.StatusBadRequest, "invalid column_id")
		return
	}

	priority := model.Priority(body.Priority)
	if priority == "" {
		priority = model.PriorityMedium
	}

	ticket, err := h.tickets.CreateTicketInTeam(r.Context(), orgID, boardID, columnID, userID, team.ID, body.Title, body.Body, priority)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func (h *Handler) v1GetTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key, number, err := parseRef(chi.URLParam(r, "ref"))
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}

	ticket, err := h.tickets.GetByRef(r.Context(), orgID, key, number)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("%s-%d not found", key, number))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (h *Handler) v1UpdateTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key, number, err := parseRef(chi.URLParam(r, "ref"))
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}

	ticket, err := h.tickets.GetByRef(r.Context(), orgID, key, number)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("%s-%d not found", key, number))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}

	var body struct {
		Title      string  `json:"title"`
		Body       string  `json:"body"`
		Priority   string  `json:"priority"`
		AssigneeID *string `json:"assignee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if body.Title == "" {
		body.Title = ticket.Title
	}
	if body.Body == "" {
		body.Body = ticket.Body
	}
	priority := model.Priority(body.Priority)
	if priority == "" {
		priority = ticket.Priority
	}

	var assigneeID *uuid.UUID
	if body.AssigneeID != nil {
		id, err := uuid.Parse(*body.AssigneeID)
		if err != nil {
			apiError(w, http.StatusBadRequest, "invalid assignee_id")
			return
		}
		assigneeID = &id
	}

	actorID := service.UserIDFromContext(r.Context())
	updated, err := h.tickets.UpdateTicket(r.Context(), orgID, ticket.ID, actorID, body.Title, body.Body, priority, assigneeID)
	if err != nil {
		apiError(w, http.StatusInternalServerError, "failed to update ticket")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) v1DeleteTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	key, number, err := parseRef(chi.URLParam(r, "ref"))
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}

	ticket, err := h.tickets.GetByRef(r.Context(), orgID, key, number)
	if err != nil {
		if isNotFound(err) {
			apiError(w, http.StatusNotFound, fmt.Sprintf("%s-%d not found", key, number))
			return
		}
		apiError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}

	if err := h.tickets.DeleteTicket(r.Context(), orgID, ticket.ID); err != nil {
		apiError(w, http.StatusInternalServerError, "failed to delete ticket")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
