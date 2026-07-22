package api

import (
	"net/http"
	"strconv"

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

	data := map[string]any{"Team": team, "Board": nil}

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
		w.Header().Set("HX-Redirect", "/workspaces/"+updated.Slug+"/settings?tab=general")
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

// UpdateTeamCapacity sets the team's sprint capacity (story points per sprint).
func (h *Handler) UpdateTeamCapacity(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}
	capacity, err := strconv.Atoi(r.FormValue("sprint_capacity"))
	if err != nil || capacity < 1 || capacity > 999 {
		http.Error(w, "capacity must be between 1 and 999", http.StatusBadRequest)
		return
	}
	if _, err := h.teams.UpdateTeamCapacity(r.Context(), orgID, team.ID, capacity); err != nil {
		http.Error(w, "failed to update capacity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "teamUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TeamSettings(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}

	tab := r.URL.Query().Get("tab")
	// Seeded up front, not only on the success paths: settings.html is shared
	// with the org-level settings view and reads all of these, and the templates
	// run under missingkey=error, so a conditional assignment is a latent 500.
	data := map[string]any{
		"Team": team, "Tab": tab,
		"Board": nil, "Members": nil, "Tracks": nil, "Tokens": nil,
	}
	var board *model.Board
	if b, err := h.teams.GetBoardForTeam(r.Context(), orgID, team.ID); err == nil {
		board = b
		data["Board"] = b
	}
	if members, err := h.teams.ListTeamMembers(r.Context(), orgID, team.ID); err == nil {
		data["Members"] = members
	}
	if tab == "tracks" && board != nil {
		tracks, _ := h.boards.ListTrackStats(r.Context(), orgID, board.ID)
		data["Tracks"] = tracks
	}

	h.render(w, "settings.html", h.pageData(r, data))
}

// renderTracksPanel re-renders the settings Tracks panel after a mutation.
func (h *Handler) renderTracksPanel(w http.ResponseWriter, r *http.Request, orgID uuid.UUID, team *model.Team, boardID uuid.UUID) {
	tracks, _ := h.boards.ListTrackStats(r.Context(), orgID, boardID)
	members, _ := h.teams.ListTeamMembers(r.Context(), orgID, team.ID)
	h.render(w, "settings-tracks-partial.html", map[string]any{
		"Team":    team,
		"Tracks":  tracks,
		"Members": members,
	})
}

// SaveTrack creates or updates a track (board tag) from workspace settings.
// A non-empty tag_id form value selects update.
func (h *Handler) SaveTrack(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}
	board, err := h.teams.GetBoardForTeam(r.Context(), orgID, team.ID)
	if err != nil {
		http.Error(w, "workspace has no board", http.StatusNotFound)
		return
	}

	if !parseForm(w, r) {
		return
	}
	name := r.FormValue("name")
	color := r.FormValue("color")
	if name == "" || !hexColorRe.MatchString(color) {
		http.Error(w, "name and a #RRGGBB color are required", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")
	var leadUserID *uuid.UUID
	if v := r.FormValue("lead_user_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			leadUserID = &id
		}
	}

	if tid := r.FormValue("tag_id"); tid != "" {
		tagID, err := uuid.Parse(tid)
		if err != nil {
			http.Error(w, "invalid tag ID", http.StatusBadRequest)
			return
		}
		if _, err := h.boards.UpdateTag(r.Context(), orgID, tagID, name, color, description, leadUserID); err != nil {
			http.Error(w, "failed to update track", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := h.boards.CreateTag(r.Context(), orgID, board.ID, name, color, description, leadUserID); err != nil {
			http.Error(w, "failed to create track", http.StatusInternalServerError)
			return
		}
	}

	h.renderTracksPanel(w, r, orgID, team, board.ID)
}

// DeleteTrack removes a track from workspace settings.
func (h *Handler) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())

	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return
	}
	board, err := h.teams.GetBoardForTeam(r.Context(), orgID, team.ID)
	if err != nil {
		http.Error(w, "workspace has no board", http.StatusNotFound)
		return
	}
	tagID, ok := pathUUID(w, r, "tagID")
	if !ok {
		return
	}
	_ = h.boards.DeleteTag(r.Context(), orgID, tagID)
	h.renderTracksPanel(w, r, orgID, team, board.ID)
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
