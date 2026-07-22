package api

import (
	"net/http"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// boardByTeamSlug resolves {teamSlug} → team + board, writing errors on failure.
func (h *Handler) boardByTeamSlug(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) (*model.Team, *model.Board, bool) {
	team, ok := h.teamBySlug(w, r, orgID)
	if !ok {
		return nil, nil, false
	}
	board, err := h.teams.GetBoardForTeam(r.Context(), orgID, team.ID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return nil, nil, false
	}
	return team, board, true
}

// teamSlugForBoard looks up the workspace slug for a board's owning team.
// Returns empty string when the board has no team or the lookup fails.
func (h *Handler) teamSlugForBoard(r *http.Request, orgID uuid.UUID, board *model.Board) string {
	if board == nil || board.TeamID == nil {
		return ""
	}
	team, err := h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
	if err != nil {
		return ""
	}
	return team.Slug
}

// workspacePageURL builds the slug-based URL for a workspace page ("board",
// "planning", "backlog"), falling back to the UUID board route if the board
// has no team (should not happen in normal use).
func (h *Handler) workspacePageURL(r *http.Request, orgID, boardID uuid.UUID, page string) string {
	board, _ := h.boards.GetBoard(r.Context(), orgID, boardID)
	if slug := h.teamSlugForBoard(r, orgID, board); slug != "" {
		return "/workspaces/" + slug + "/" + page
	}
	return "/boards/" + boardID.String()
}

func (h *Handler) redirectToBoard(w http.ResponseWriter, r *http.Request, orgID, boardID uuid.UUID) {
	http.Redirect(w, r, h.workspacePageURL(r, orgID, boardID, "board"), http.StatusSeeOther)
}

func (h *Handler) redirectToPlanning(w http.ResponseWriter, r *http.Request, orgID, boardID uuid.UUID) {
	http.Redirect(w, r, h.workspacePageURL(r, orgID, boardID, "planning"), http.StatusSeeOther)
}

func (h *Handler) redirectToBacklog(w http.ResponseWriter, r *http.Request, orgID, boardID uuid.UUID) {
	http.Redirect(w, r, h.workspacePageURL(r, orgID, boardID, "backlog"), http.StatusSeeOther)
}

// hxRedirectToPage tells an HTMX poll to do a full-page navigation, used when
// the sprint state changed in another tab and the current page no longer applies.
func (h *Handler) hxRedirectToPage(w http.ResponseWriter, r *http.Request, orgID, boardID uuid.UUID, page string) {
	w.Header().Set("HX-Redirect", h.workspacePageURL(r, orgID, boardID, page))
	w.WriteHeader(http.StatusNoContent)
}

// boardViewData expands a BoardView struct into a template data map that also
// includes NavTeams so the sidebar can render the team list.
// SprintID is the active sprint ID (or first planning sprint ID) for drag-from-backlog support.
func (h *Handler) boardViewData(r *http.Request, view *model.BoardView) map[string]any {
	sprintID := ""
	hasPlanningSprint := false
	for _, s := range view.Sprints {
		if s.Status == model.SprintStatusPlanning {
			if sprintID == "" {
				sprintID = s.ID.String()
			}
			hasPlanningSprint = true
		}
	}
	if view.ActiveSprint != nil {
		sprintID = view.ActiveSprint.ID.String()
	}
	return h.pageData(r, map[string]any{
		"Board":               view.Board,
		"Team":                view.Team,
		"Columns":             view.Columns,
		"ActiveSprint":        view.ActiveSprint,
		"Sprints":             view.Sprints,
		"SprintViews":         view.SprintViews,
		"BacklogCount":        view.BacklogCount,
		"BacklogPoints":       view.BacklogPoints,
		"UnestimatedCount":    view.UnestimatedCount,
		"FirstColumnID":       view.FirstColumnID,
		"SprintID":            sprintID,
		"ActiveSprintSection": view.ActiveSprintSection,
		"HasPlanningSprint":   hasPlanningSprint,
		// Read by the board filter's Tracks section (board-columns.html). The
		// service has always populated it; this never passed it through, so that
		// filter silently listed nothing.
		"BoardTags": view.BoardTags,
	})
}

// TrackPage renders the track (tag) detail page showing all tickets with that tag.
func (h *Handler) TrackPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}

	tagID, ok := pathUUID(w, r, "tagID")
	if !ok {
		return
	}

	tag, err := h.boards.GetTag(r.Context(), orgID, tagID)
	if err != nil {
		http.Error(w, "tag not found", http.StatusNotFound)
		return
	}

	tickets, _ := h.boards.ListTicketsByTag(r.Context(), orgID, tagID)
	columns, _ := h.boards.ListColumns(r.Context(), orgID, board.ID)

	// Build column name lookup
	colNames := make(map[uuid.UUID]string)
	for _, c := range columns {
		colNames[c.ID] = c.Name
	}

	// Group by column
	type trackGroup struct {
		Name    string
		Tickets []model.Ticket
	}
	seen := make(map[uuid.UUID]int)
	var groups []trackGroup
	inProgress, inReview, done, open, totalPts := 0, 0, 0, 0, 0

	for _, t := range tickets {
		name := colNames[t.ColumnID]
		if name == "" {
			name = "Backlog"
		}
		if idx, ok := seen[t.ColumnID]; ok {
			groups[idx].Tickets = append(groups[idx].Tickets, t)
		} else {
			seen[t.ColumnID] = len(groups)
			groups = append(groups, trackGroup{Name: name, Tickets: []model.Ticket{t}})
		}
		col := strings.ToLower(name)
		switch {
		case strings.Contains(col, "progress"):
			inProgress++
		case strings.Contains(col, "review"):
			inReview++
		case t.ClosedAt != nil:
			done++
		}
		if t.StoryPoints != nil {
			totalPts += int(*t.StoryPoints)
		}
	}
	open = len(tickets) - done

	h.render(w, "track.html", h.pageData(r, map[string]any{
		"Tag":        tag,
		"Board":      board,
		"Team":       team,
		"Groups":     groups,
		"Open":       open,
		"InProgress": inProgress,
		"InReview":   inReview,
		"Done":       done,
		"TotalPts":   totalPts,
	}))
}

func (h *Handler) BoardView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	view, err := h.boards.GetBoardView(r.Context(), orgID, board.ID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}
	// A scrum board without an active sprint has nothing to show — planning is
	// the only meaningful view, so go straight there.
	if board.Mode.IsScrum() && view.ActiveSprint == nil {
		http.Redirect(w, r, "/workspaces/"+team.Slug+"/planning", http.StatusSeeOther)
		return
	}
	h.render(w, "board.html", h.boardViewData(r, view))
}

// BoardPlanning renders the sprint planning page. One URL, one view: while a
// sprint is active (or the board is not scrum) it redirects to the board.
// Visiting with no draft sprint starts one automatically — there is never an
// empty placeholder between sprints.
func (h *Handler) BoardPlanning(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	view, err := h.boards.GetBoardView(r.Context(), orgID, board.ID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}
	if !board.Mode.IsScrum() || view.ActiveSprint != nil {
		http.Redirect(w, r, "/workspaces/"+team.Slug+"/board", http.StatusSeeOther)
		return
	}
	data := h.boardViewData(r, view)
	if data["HasPlanningSprint"] != true {
		name := h.nextSprintName(r, orgID, board.ID)
		if _, err := h.boards.CreateSprint(r.Context(), orgID, board.ID, userID, name, "", nil, nil); err != nil {
			http.Error(w, "failed to create sprint", http.StatusInternalServerError)
			return
		}
		view, err = h.boards.GetBoardView(r.Context(), orgID, board.ID)
		if err != nil {
			http.Error(w, "board not found", http.StatusNotFound)
			return
		}
		data = h.boardViewData(r, view)
	}
	h.render(w, "planning.html", data)
}

// PlanningColumnsPartial serves the inner HTML of #board-columns on the
// planning page for HTMX polling. If the sprint was started or cancelled in
// another tab, it redirects the client back to the board.
func (h *Handler) PlanningColumnsPartial(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}

	view, err := h.boards.GetBoardView(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}

	data := h.boardViewData(r, view)
	if view.ActiveSprint != nil || data["HasPlanningSprint"] != true {
		h.hxRedirectToPage(w, r, orgID, boardID, "board")
		return
	}
	h.render(w, "planning-columns.html", data)
}

// BoardColumnsPartial serves the inner HTML of #board-columns for HTMX polling.
func (h *Handler) BoardColumnsPartial(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}

	view, err := h.boards.GetBoardView(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "board not found", http.StatusNotFound)
		return
	}

	// Sprint closed in another tab — move this tab to planning on the next poll.
	if view.Board.Mode.IsScrum() && view.ActiveSprint == nil {
		h.hxRedirectToPage(w, r, orgID, boardID, "planning")
		return
	}

	h.render(w, "board-columns.html", h.boardViewData(r, view))
}

func (h *Handler) UpdateBoard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}

	_, err := h.boards.UpdateBoard(r.Context(), orgID, boardID, r.FormValue("name"), r.FormValue("description"))
	if err != nil {
		http.Error(w, "failed to update board", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
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

	redirectTo := "/"
	if slug := h.teamSlugForBoard(r, orgID, board); slug != "" {
		redirectTo = "/workspaces/" + slug
	}
	w.Header().Set("HX-Redirect", redirectTo)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BoardRoadmap(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	sprints, err := h.boards.GetRoadmap(r.Context(), orgID, board.ID)
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
	_, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	view, err := h.boards.GetBacklog(r.Context(), orgID, board.ID)
	if err != nil {
		http.Error(w, "backlog not found", http.StatusNotFound)
		return
	}
	data := h.boardViewData(r, view)
	data["InitRefineMode"] = strings.Contains(r.Header.Get("HX-Current-URL"), "/refinement")
	h.render(w, "backlog.html", data)
}

func (h *Handler) BoardRefinement(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	_, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	view, err := h.boards.GetBacklog(r.Context(), orgID, board.ID)
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
	_, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	boardID := board.ID
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
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	assigneeIDs := r.URL.Query()["assignee_id"]
	filters := model.DailyScrumFilters{
		Q:                r.URL.Query().Get("q"),
		AssigneeIDs:      assigneeIDs,
		TagIDs:           r.URL.Query()["tag_id"],
		Priorities:       r.URL.Query()["priority"],
		FilterUnassigned: r.URL.Query().Get("unassigned") == "1",
	}
	view, err := h.boards.GetDailyScrumView(r.Context(), orgID, boardID, filters)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Speaker mode: exactly one assignee → render the speaker panel partial
	if len(assigneeIDs) == 1 && assigneeIDs[0] != "" {
		speakerID, err := uuid.Parse(assigneeIDs[0])
		if err == nil {
			activity, _ := h.tickets.ListActivityByActor(r.Context(), orgID, speakerID)
			// Find the speaker's User from AllAssignees
			var speakerUser *model.User
			for i, u := range view.AllAssignees {
				if u.ID == speakerID {
					uu := view.AllAssignees[i]
					speakerUser = &uu
					break
				}
			}
			h.render(w, "daily-speaker.html", map[string]any{
				"Board":       view.Board,
				"Groups":      view.Groups,
				"Unassigned":  view.Unassigned,
				"SpeakerUser": speakerUser,
				"Activity":    activity,
			})
			return
		}
	}

	h.render(w, "daily-tickets.html", map[string]any{
		"Board":        view.Board,
		"Groups":       view.Groups,
		"Unassigned":   view.Unassigned,
		"Filters":      view.Filters,
		"ActiveSprint": view.ActiveSprint,
	})
}
