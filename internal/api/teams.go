package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// teamBySlug resolves the {teamSlug} URL param to a Team, writing an error response on failure.
func (h *Handler) teamBySlug(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) (*model.Team, bool) {
	slug := chi.URLParam(r, "teamSlug")
	team, err := h.teams.GetTeamBySlug(r.Context(), orgID, slug)
	if err != nil {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return nil, false
	}
	return team, true
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := service.OrgIDFromContext(ctx)
	userID := service.UserIDFromContext(ctx)

	blocked, _ := h.tickets.ListBlockedTickets(ctx, orgID)
	myTickets, _ := h.tickets.ListMyOpenTickets(ctx, orgID, userID)
	activity, _ := h.tickets.ListRecentOrgActivity(ctx, orgID)

	var myPts float64
	for _, t := range myTickets {
		if t.StoryPoints != nil {
			myPts += *t.StoryPoints
		}
	}

	h.render(w, "dashboard.html", h.pageData(r, map[string]any{
		"BlockedTickets":  blocked,
		"MyTickets":       myTickets,
		"MyTicketsPoints": myPts,
		"Activity":        activity,
	}))
}

func (h *Handler) TeamList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := service.OrgIDFromContext(ctx)
	teams, err := h.teams.ListTeamsWithBoards(ctx, orgID)
	if err != nil {
		http.Error(w, "failed to load teams", http.StatusInternalServerError)
		return
	}

	activeSprintCount := 0
	totalTickets := 0
	for _, t := range teams {
		if t.ActiveSprint != nil {
			activeSprintCount++
		}
		totalTickets += t.Team.TicketCounter
	}

	memberCount, _ := h.teams.CountOrgUsers(ctx, orgID)

	h.render(w, "teams.html", h.pageData(r, map[string]any{
		"Teams":             teams,
		"ActiveSprintCount": activeSprintCount,
		"TotalTickets":      totalTickets,
		"MemberCount":       memberCount,
	}))
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	if !parseForm(w, r) {
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

	team, _, err := h.teams.CreateTeam(r.Context(), orgID, userID, name, key, r.FormValue("description"), mode)
	if err != nil {
		http.Error(w, "failed to create team", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/workspaces/"+team.Slug, http.StatusSeeOther)
}

func (h *Handler) TeamView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID := service.OrgIDFromContext(ctx)
	userID := service.UserIDFromContext(ctx)

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	data := map[string]any{"Team": team}

	if board, err := h.teams.GetBoardForTeam(ctx, orgID, team.ID); err == nil {
		data["Board"] = board
	}

	blocked, _ := h.tickets.ListBlockedTicketsByTeam(ctx, orgID, team.ID)
	myTickets, _ := h.tickets.ListMyOpenTicketsByTeam(ctx, orgID, userID, team.ID)
	activity, _ := h.tickets.ListRecentTeamActivity(ctx, orgID, team.ID)

	var myPts float64
	for _, t := range myTickets {
		if t.StoryPoints != nil {
			myPts += *t.StoryPoints
		}
	}

	data["BlockedTickets"] = blocked
	data["MyTickets"] = myTickets
	data["MyTicketsPoints"] = myPts
	data["Activity"] = activity

	h.render(w, "team.html", h.pageData(r, data))
}

func (h *Handler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}

	updated, err := h.teams.UpdateTeam(r.Context(), orgID, team.ID, r.FormValue("name"), r.FormValue("description"))
	if err != nil {
		http.Error(w, "failed to update team", http.StatusInternalServerError)
		return
	}

	// If the name changed the slug changed; redirect so the browser URL stays valid.
	if updated.Slug != team.Slug {
		w.Header().Set("HX-Redirect", "/workspaces/"+updated.Slug+"/settings")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("HX-Trigger", "teamUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SearchTeamNonMembers(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	q := r.URL.Query().Get("q")
	users, _ := h.teams.SearchNonMembers(r.Context(), orgID, team.ID, q)
	h.render(w, "team-member-results.html", map[string]any{
		"Team":  team,
		"Users": users,
	})
}

func (h *Handler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}
	userID, err := uuid.Parse(r.FormValue("user_id"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}
	if err := h.teams.AddTeamMember(r.Context(), orgID, team.ID, userID); err != nil {
		http.Error(w, "failed to add member", http.StatusInternalServerError)
		return
	}
	members, _ := h.teams.ListTeamMembers(r.Context(), orgID, team.ID)
	h.render(w, "team-members.html", map[string]any{
		"Team":    team,
		"Members": members,
	})
}

func (h *Handler) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	userID, ok := pathUUID(w, r, "userID")
	if !ok {
		return
	}
	if err := h.teams.RemoveTeamMember(r.Context(), orgID, team.ID, userID); err != nil {
		http.Error(w, "failed to remove member", http.StatusInternalServerError)
		return
	}
	members, _ := h.teams.ListTeamMembers(r.Context(), orgID, team.ID)
	h.render(w, "team-members.html", map[string]any{
		"Team":    team,
		"Members": members,
	})
}

func (h *Handler) TeamSettings(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	data := map[string]any{"Team": team}
	if board, err := h.teams.GetBoardForTeam(r.Context(), orgID, team.ID); err == nil {
		data["Board"] = board
	}
	if members, err := h.teams.ListTeamMembers(r.Context(), orgID, team.ID); err == nil {
		data["Members"] = members
	}

	h.render(w, "settings.html", h.pageData(r, data))
}

func (h *Handler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	if err := h.teams.DeleteTeam(r.Context(), orgID, team.ID); err != nil {
		http.Error(w, "failed to delete team", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusNoContent)
}
