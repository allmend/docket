package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	h.render(w, "dashboard.html", h.pageData(r, nil))
}

func (h *Handler) TeamList(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teams, err := h.teams.ListTeamsWithBoards(r.Context(), orgID)
	if err != nil {
		http.Error(w, "failed to load teams", http.StatusInternalServerError)
		return
	}
	h.render(w, "teams.html", h.pageData(r, map[string]any{"Teams": teams}))
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	key := r.FormValue("key")
	if name == "" || key == "" {
		http.Error(w, "name and key are required", http.StatusBadRequest)
		return
	}

	mode := model.BoardMode(r.FormValue("mode"))
	if mode != model.BoardModeKanban && mode != model.BoardModeScrum && mode != model.BoardModeBlank {
		mode = model.BoardModeKanban
	}

	team, board, err := h.teams.CreateTeam(r.Context(), orgID, userID, name, key, r.FormValue("description"), mode)
	if err != nil {
		http.Error(w, "failed to create team", http.StatusInternalServerError)
		return
	}

	// Redirect straight to the team's board.
	http.Redirect(w, r, "/boards/"+board.ID.String()+"?team="+team.ID.String(), http.StatusSeeOther)
}

func (h *Handler) TeamView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}

	team, err := h.teams.GetTeam(r.Context(), orgID, teamID)
	if err != nil {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}

	data := map[string]any{"Team": team}

	if board, err := h.teams.GetBoardForTeam(r.Context(), orgID, teamID); err == nil {
		data["Board"] = board
	}
	if members, err := h.teams.ListTeamMembers(r.Context(), orgID, teamID); err == nil {
		data["Members"] = members
	}

	h.render(w, "team.html", h.pageData(r, data))
}

func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	_, err = h.teams.UpdateTeam(r.Context(), orgID, teamID, r.FormValue("name"), r.FormValue("description"))
	if err != nil {
		http.Error(w, "failed to update team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "teamUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SearchTeamNonMembers(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}
	q := r.URL.Query().Get("q")
	users, _ := h.teams.SearchNonMembers(r.Context(), orgID, teamID, q)
	h.render(w, "team-member-results.html", map[string]any{
		"TeamID": teamID,
		"Users":  users,
	})
}

func (h *Handler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userID, err := uuid.Parse(r.FormValue("user_id"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}
	if err := h.teams.AddTeamMember(r.Context(), orgID, teamID, userID); err != nil {
		http.Error(w, "failed to add member", http.StatusInternalServerError)
		return
	}
	members, _ := h.teams.ListTeamMembers(r.Context(), orgID, teamID)
	h.render(w, "team-members.html", map[string]any{
		"Team":    &model.Team{ID: teamID},
		"Members": members,
	})
}

func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}
	if err := h.teams.RemoveTeamMember(r.Context(), orgID, teamID, userID); err != nil {
		http.Error(w, "failed to remove member", http.StatusInternalServerError)
		return
	}
	members, _ := h.teams.ListTeamMembers(r.Context(), orgID, teamID)
	h.render(w, "team-members.html", map[string]any{
		"Team":    &model.Team{ID: teamID},
		"Members": members,
	})
}

func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	teamID, err := uuid.Parse(chi.URLParam(r, "teamID"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}

	if err := h.teams.DeleteTeam(r.Context(), orgID, teamID); err != nil {
		http.Error(w, "failed to delete team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/teams")
	w.WriteHeader(http.StatusNoContent)
}
