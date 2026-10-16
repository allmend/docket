package api

import (
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
)

// Handler is the base struct for all HTTP handlers.
type Handler struct {
	auth          *service.AuthService
	teams         *service.TeamService
	boards        *service.BoardService
	tickets       *service.TicketService
	comments      *service.CommentService
	links         *service.LinkService
	notifications *service.NotificationService
	retro         *service.RetroService
	metrics       *service.MetricsService
	tokens        *service.TokenService
	tmpls         map[string]*template.Template
	// cookieSecure marks session cookies Secure. Off only for deployments served
	// over plain HTTP on a trusted network — browsers drop Secure cookies there,
	// which makes login silently fail.
	cookieSecure bool
}

func NewHandler(
	auth *service.AuthService,
	teams *service.TeamService,
	boards *service.BoardService,
	tickets *service.TicketService,
	comments *service.CommentService,
	links *service.LinkService,
	notifications *service.NotificationService,
	retro *service.RetroService,
	metrics *service.MetricsService,
	tokens *service.TokenService,
	tmplDir string,
	cookieSecure bool,
) (*Handler, error) {
	tmpls, err := parseTemplates(tmplDir)
	if err != nil {
		return nil, err
	}
	return &Handler{auth: auth, teams: teams, boards: boards, tickets: tickets, comments: comments, links: links, notifications: notifications, retro: retro, metrics: metrics, tokens: tokens, tmpls: tmpls, cookieSecure: cookieSecure}, nil
}

var authPages = map[string]bool{
	"login.html": true,
}

func parseTemplates(root string) (map[string]*template.Template, error) {
	base := filepath.Join(root, "layouts", "base.html")
	baseAuth := filepath.Join(root, "layouts", "base-auth.html")

	// Partials — only parse if directory exists
	partialPaths, _ := filepath.Glob(filepath.Join(root, "partials", "*.html"))

	pagePaths, err := filepath.Glob(filepath.Join(root, "pages", "*.html"))
	if err != nil {
		return nil, err
	}

	funcs := templateFuncs()

	// missingkey=error: a template referencing data its handler didn't pass is a
	// hard error, not a silently empty page. Without it, "the partial didn't
	// follow along" ships unnoticed — TestTemplateDataContract catches the same
	// class at build time, this is the runtime backstop for the render sites it
	// can't analyse statically (struct data, non-literal maps).
	// Note it also rejects nil data, so pass an empty map, never nil.
	shared := template.New("").Funcs(funcs).Option("missingkey=error")
	if len(partialPaths) > 0 {
		var err error
		shared, err = shared.ParseFiles(partialPaths...)
		if err != nil {
			return nil, err
		}
	}

	tmpls := make(map[string]*template.Template)

	for _, page := range pagePaths {
		name := filepath.Base(page)
		layout := base
		if authPages[name] {
			layout = baseAuth
		}
		// Only include layout if it exists
		files := []string{page}
		if _, err := filepath.Abs(layout); err == nil {
			files = append([]string{layout}, files...)
		}
		t, err := template.Must(shared.Clone()).Funcs(funcs).ParseFiles(files...)
		if err != nil {
			return nil, err
		}
		tmpls[name] = t
	}

	for _, partial := range partialPaths {
		tmpls[filepath.Base(partial)] = shared
	}

	return tmpls, nil
}

// pageData merges nav-level data into the given map.
func (h *Handler) pageData(r *http.Request, data map[string]any) map[string]any {
	if data == nil {
		data = make(map[string]any)
	}
	ctx := r.Context()
	orgID := service.OrgIDFromContext(ctx)

	// Seed every nav key the base layout reads, so the contract is total: the
	// templates run under missingkey=error, and these used to be set only on the
	// success path (NavTags/NavActiveSprint only when the page passed a Board),
	// which would turn a failed lookup — or simply an org-level page — into a 500.
	// Real values overwrite these below. Kept in step with pageDataKeys in
	// templatecontract_test.go.
	for _, k := range []string{"NavTeams", "CurrentUser", "NavTeam", "NavBoard", "NavTags", "NavActiveSprint"} {
		if _, has := data[k]; !has {
			data[k] = nil
		}
	}
	data["NavUnreadCount"] = 0
	data["NavMyIssueCount"] = 0
	data["OrgName"] = ""
	data["OrgSlug"] = ""

	// Minimal nav for org-level pages (dashboard, teams list, inbox, my-issues).
	if teams, err := h.teams.ListTeamsWithBoards(ctx, orgID); err == nil {
		data["NavTeams"] = teams
	}
	if u := h.auth.GetCurrentUser(ctx); u != nil {
		data["CurrentUser"] = u
		userID := service.UserIDFromContext(ctx)
		if count, err := h.notifications.UnreadCount(ctx, orgID, userID); err == nil {
			data["NavUnreadCount"] = count
		}
		if tickets, err := h.tickets.ListMyTickets(ctx, orgID, userID); err == nil {
			data["NavMyIssueCount"] = len(tickets)
		}
	}
	if org := h.auth.GetCurrentOrg(ctx); org != nil {
		data["OrgName"] = org.Name
		data["OrgSlug"] = org.Slug
	}
	data["CurrentPath"] = r.URL.Path

	// Workspace-context nav: normalise Team → NavTeam and Board → NavBoard.
	var navTeam *model.Team
	switch v := data["Team"].(type) {
	case model.Team:
		t := v
		navTeam = &t
	case *model.Team:
		navTeam = v
	}
	if navTeam != nil {
		data["NavTeam"] = navTeam
	}

	var navBoard *model.Board
	switch v := data["Board"].(type) {
	case model.Board:
		b := v
		navBoard = &b
	case *model.Board:
		navBoard = v
	}
	if navBoard != nil {
		data["NavBoard"] = navBoard
		if tags, err := h.boards.ListBoardTags(ctx, orgID, navBoard.ID); err == nil {
			data["NavTags"] = tags
		}
		if _, has := data["ActiveSprint"]; has {
			data["NavActiveSprint"] = data["ActiveSprint"]
		} else if sp, err := h.boards.GetActiveSprint(ctx, orgID, navBoard.ID); err == nil {
			data["NavActiveSprint"] = sp
		}
	}

	return data
}

// render executes the named template. Pages use the "base" entry point; partials use their define name.
func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	t, ok := h.tmpls[name]
	if !ok {
		slog.Error("template not found", "name", name)
		http.NotFound(w, nil)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	entry := "base"
	if t.Lookup("base") == nil {
		ext := filepath.Ext(name)
		entry = name[:len(name)-len(ext)]
	}

	if err := t.ExecuteTemplate(w, entry, data); err != nil {
		slog.Error("template render", "name", name, "err", err)
	}
}

// Routes mounts all authenticated application routes.
func (h *Handler) Routes(r chi.Router) {
	a := routes{r: r}

	a.get(model.TierRead, "/", h.TeamList)
	a.get(model.TierRead, "/dashboard", h.Dashboard)
	a.get(model.TierRead, "/me", h.ProfilePage)
	a.post(model.TierWrite, "/tickets/new", h.CreateGlobalTicket)
	a.get(model.TierRead, "/my-issues", h.MyIssues)
	a.get(model.TierRead, "/inbox", h.Inbox)
	a.post(model.TierWrite, "/inbox/mark-read", h.MarkNotificationsRead)
	a.get(model.TierRead, "/nav/unread-count", h.NavUnreadCount)

	a.get(model.TierRead, "/workspaces", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})
	a.post(model.TierWrite, "/workspaces", h.CreateTeam)
	a.get(model.TierRead, "/workspaces/{teamSlug}", h.TeamView)
	a.get(model.TierRead, "/workspaces/{teamSlug}/settings", h.TeamSettings)
	a.put(model.TierWrite, "/workspaces/{teamSlug}", h.UpdateTeam)
	a.put(model.TierWrite, "/workspaces/{teamSlug}/capacity", h.UpdateTeamCapacity)
	a.post(model.TierWrite, "/workspaces/{teamSlug}/tracks", h.SaveTrack)
	a.delete(model.TierWrite, "/workspaces/{teamSlug}/tracks/{tagID}", h.DeleteTrack)
	a.delete(model.TierWrite, "/workspaces/{teamSlug}", h.DeleteTeam)
	a.get(model.TierRead, "/workspaces/{teamSlug}/members/search", h.SearchTeamNonMembers)
	a.post(model.TierWrite, "/workspaces/{teamSlug}/members", h.AddTeamMember)
	a.delete(model.TierWrite, "/workspaces/{teamSlug}/members/{userID}", h.RemoveTeamMember)

	// Workspace page routes — slug-based, appear in the address bar
	a.get(model.TierRead, "/workspaces/{teamSlug}/board", h.BoardView)
	a.get(model.TierRead, "/workspaces/{teamSlug}/planning", h.BoardPlanning)
	a.get(model.TierRead, "/workspaces/{teamSlug}/backlog", h.BoardBacklog)
	a.get(model.TierRead, "/workspaces/{teamSlug}/backlog/refinement", h.BoardRefinement)
	a.get(model.TierRead, "/workspaces/{teamSlug}/board/daily", h.BoardDailyScrum)
	a.get(model.TierRead, "/workspaces/{teamSlug}/roadmap", h.BoardRoadmap)
	a.get(model.TierRead, "/workspaces/{teamSlug}/tracks/{tagID}", h.TrackPage)
	a.get(model.TierRead, "/workspaces/{teamSlug}/retros", h.RetrosListPage)
	a.get(model.TierRead, "/workspaces/{teamSlug}/retros/{retroBoardID}", h.RetroBoardPage)
	a.get(model.TierRead, "/workspaces/{teamSlug}/sprints/{sprintID}/review", h.SprintReviewPage)

	// Board action routes — UUID-based, used by HTMX, never in the address bar
	a.get(model.TierRead, "/boards/{boardID}/columns", h.BoardColumnsPartial)
	a.get(model.TierRead, "/boards/{boardID}/planning/columns", h.PlanningColumnsPartial)
	a.put(model.TierWrite, "/boards/{boardID}", h.UpdateBoard)
	a.delete(model.TierWrite, "/boards/{boardID}", h.DeleteBoard)

	a.post(model.TierWrite, "/boards/{boardID}/columns", h.CreateColumn)
	a.put(model.TierWrite, "/boards/{boardID}/columns/{columnID}", h.RenameColumn)
	a.delete(model.TierWrite, "/boards/{boardID}/columns/{columnID}", h.DeleteColumn)

	a.get(model.TierRead, "/boards/{boardID}/daily/tickets", h.BoardDailyScrumTickets)
	a.get(model.TierRead, "/boards/{boardID}/tags", h.BoardTagsJSON)
	a.get(model.TierRead, "/boards/{boardID}/tags/manage", h.BoardTagsPanel)
	a.post(model.TierWrite, "/boards/{boardID}/tags", h.CreateTag)
	a.delete(model.TierWrite, "/boards/{boardID}/tags/{tagID}", h.DeleteTag)

	a.get(model.TierRead, "/boards/{boardID}/dod", h.BoardDodPanel)
	a.post(model.TierWrite, "/boards/{boardID}/dod", h.CreateDodItem)
	a.delete(model.TierWrite, "/boards/{boardID}/dod/{itemID}", h.DeleteDodItem)

	a.get(model.TierRead, "/tickets/{ticketID}/dod", h.TicketDodPartial)
	a.post(model.TierWrite, "/tickets/{ticketID}/dod/{itemID}/toggle", h.ToggleDodCheck)

	a.post(model.TierWrite, "/boards/{boardID}/sprints", h.CreateSprint)
	a.post(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/assign", h.AssignTicketsToSprint)
	a.put(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}", h.UpdateSprint)
	a.post(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/start", h.StartSprint)
	a.post(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/close", h.CloseSprint)
	a.delete(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}", h.DeleteSprint)
	a.post(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/delete", h.DeleteSprint)
	a.get(model.TierRead, "/boards/{boardID}/sprints/{sprintID}/capacity", h.SprintCapacityPartial)
	a.put(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/capacity/{userID}", h.UpdateMemberCapacity)

	a.post(model.TierWrite, "/tickets/{ticketID}/sprint", h.AssignTicketToSprint)

	a.post(model.TierWrite, "/boards/{boardID}/tickets", h.CreateTicket)
	a.post(model.TierWrite, "/boards/{boardID}/backlog/tickets", h.CreateBacklogTicket)
	a.get(model.TierRead, "/tickets/{ref}", h.TicketPage)
	a.get(model.TierRead, "/tickets/{ticketID}/quick", h.TicketQuickView)
	a.get(model.TierRead, "/tickets/{ticketID}/refine", h.TicketRefineView)
	a.get(model.TierRead, "/tickets/{ticketID}/edit", h.TicketEditForm)
	a.get(model.TierRead, "/tickets/{ticketID}/view", h.TicketBodyView)
	a.put(model.TierWrite, "/tickets/{ticketID}", h.UpdateTicket)
	a.put(model.TierWrite, "/tickets/{ticketID}/title", h.UpdateTicketTitle)
	a.get(model.TierRead, "/tickets/{ticketID}/body/edit", h.TicketBodyEditForm)
	a.get(model.TierRead, "/tickets/{ticketID}/body/view", h.TicketBodySectionView)
	a.put(model.TierWrite, "/tickets/{ticketID}/body", h.UpdateTicketBody)
	a.put(model.TierWrite, "/tickets/{ticketID}/ac", h.UpdateTicketAC)
	a.post(model.TierWrite, "/tickets/{ticketID}/ac/add", h.AddACItem)
	a.post(model.TierWrite, "/tickets/{ticketID}/ac/toggle/{index}", h.ToggleACCheckbox)
	a.post(model.TierWrite, "/tickets/{ticketID}/ac/delete/{index}", h.DeleteACItem)
	a.put(model.TierWrite, "/tickets/{ticketID}/priority", h.UpdateTicketPriority)
	a.put(model.TierWrite, "/tickets/{ticketID}/points", h.UpdateTicketPoints)
	a.put(model.TierWrite, "/tickets/{ticketID}/column", h.UpdateTicketColumn)
	a.get(model.TierRead, "/tickets/{ticketID}/link-search", h.SearchTicketsForLink)
	a.post(model.TierWrite, "/tickets/{ticketID}/close", h.CloseTicket)
	a.post(model.TierWrite, "/tickets/{ticketID}/reopen", h.ReopenTicket)
	a.delete(model.TierWrite, "/tickets/{ticketID}", h.DeleteTicket)
	a.post(model.TierWrite, "/tickets/{ticketID}/move", h.MoveTicket)
	a.post(model.TierWrite, "/tickets/{ticketID}/sprint-place", h.SprintPlaceTicket)
	a.get(model.TierRead, "/tickets/{ticketID}/assignees/search", h.SearchTicketAssignees)
	a.post(model.TierWrite, "/tickets/{ticketID}/assignees", h.AddTicketAssignee)
	a.delete(model.TierWrite, "/tickets/{ticketID}/assignees/{userID}", h.RemoveTicketAssignee)
	a.post(model.TierWrite, "/tickets/{ticketID}/tags", h.AddTagToTicket)
	a.delete(model.TierWrite, "/tickets/{ticketID}/tags/{tagID}", h.RemoveTagFromTicket)

	a.get(model.TierRead, "/search", h.Search)
	a.get(model.TierRead, "/users/search", h.SearchUsersForMention)
	a.get(model.TierRead, "/users/{username}", h.UserPage)
	a.get(model.TierRead, "/tickets/search-mention", h.SearchTicketsForMention)

	a.post(model.TierWrite, "/tickets/{ticketID}/comments", h.CreateComment)
	a.get(model.TierRead, "/comments/{commentID}/edit", h.CommentEditForm)
	a.get(model.TierRead, "/comments/{commentID}/view", h.CommentView)
	a.put(model.TierWrite, "/comments/{commentID}", h.UpdateComment)
	a.delete(model.TierWrite, "/comments/{commentID}", h.DeleteComment)

	a.post(model.TierWrite, "/tickets/{ticketID}/links", h.CreateLink)
	a.delete(model.TierWrite, "/tickets/{ticketID}/links/{linkID}", h.DeleteLink)

	a.post(model.TierWrite, "/boards/{boardID}/sprints/{sprintID}/close-and-retro", h.CloseSprintAndStartRetro)

	a.post(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/close", h.CloseRetroBoard)
	a.post(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/cards", h.CreateRetroCard)
	a.delete(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/cards/{cardID}", h.DeleteRetroCard)
	a.post(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/cards/{cardID}/assign", h.AssignRetroCardOwner)
	a.post(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/cards/{cardID}/stack", h.StackRetroCard)
	a.delete(model.TierWrite, "/boards/{boardID}/retro/{retroBoardID}/cards/{cardID}/stack", h.UnstackRetroCard)

	a.get(model.TierRead, "/settings", h.SettingsPage)
	a.post(model.TierWrite, "/settings/tokens", h.CreateToken)
	a.delete(model.TierWrite, "/settings/tokens/{tokenID}", h.RevokeToken)
	a.post(model.TierAdmin, "/settings/members", h.CreateMember)
	a.put(model.TierAdmin, "/settings/members/{userID}/role", h.UpdateMemberRole)
}
