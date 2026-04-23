package api

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

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
	tmpls         map[string]*template.Template
}

func NewHandler(
	auth *service.AuthService,
	teams *service.TeamService,
	boards *service.BoardService,
	tickets *service.TicketService,
	comments *service.CommentService,
	links *service.LinkService,
	notifications *service.NotificationService,
	tmplDir string,
) (*Handler, error) {
	tmpls, err := parseTemplates(tmplDir)
	if err != nil {
		return nil, err
	}
	return &Handler{auth: auth, teams: teams, boards: boards, tickets: tickets, comments: comments, links: links, notifications: notifications, tmpls: tmpls}, nil
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

	funcs := template.FuncMap{
		"sub": func(a, b int) int { return a - b },
		"initials": func(s string) string {
			runes := []rune(s)
			if len(runes) == 0 {
				return "?"
			}
			return string(runes[0])
		},
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of arguments")
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key must be string")
				}
				m[key] = pairs[i+1]
			}
			return m, nil
		},
	}

	shared := template.New("").Funcs(funcs)
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
	if teams, err := h.teams.ListTeamsWithBoards(ctx, orgID); err == nil {
		data["NavTeams"] = teams
	}
	if u := h.auth.GetCurrentUser(ctx); u != nil {
		data["CurrentUser"] = u
	}
	data["CurrentPath"] = r.URL.Path
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
	r.Get("/", h.Dashboard)
	r.Post("/tickets/new", h.CreateGlobalTicket)
	r.Get("/my-issues", h.MyIssues)
	r.Get("/inbox", h.Inbox)
	r.Post("/inbox/mark-read", h.MarkNotificationsRead)
	r.Get("/nav/unread-count", h.NavUnreadCount)

	r.Get("/teams", h.TeamList)
	r.Post("/teams", h.CreateTeam)
	r.Get("/teams/{teamID}", h.TeamView)
	r.Put("/teams/{teamID}", h.UpdateTeam)
	r.Delete("/teams/{teamID}", h.DeleteTeam)
	r.Get("/teams/{teamID}/members/search", h.SearchTeamNonMembers)
	r.Post("/teams/{teamID}/members", h.AddTeamMember)
	r.Delete("/teams/{teamID}/members/{userID}", h.RemoveTeamMember)

	r.Get("/boards/{boardID}", h.BoardView)
	r.Put("/boards/{boardID}", h.UpdateBoard)
	r.Delete("/boards/{boardID}", h.DeleteBoard)

	r.Get("/boards/{boardID}/columns", h.BoardColumnsPartial)
	r.Post("/boards/{boardID}/columns", h.CreateColumn)
	r.Put("/boards/{boardID}/columns/{columnID}", h.RenameColumn)
	r.Delete("/boards/{boardID}/columns/{columnID}", h.DeleteColumn)

	r.Get("/boards/{boardID}/backlog", h.BoardBacklog)
	r.Get("/boards/{boardID}/backlog/tickets", h.BacklogTicketList)
	r.Post("/boards/{boardID}/sprints", h.CreateSprint)
	r.Post("/boards/{boardID}/sprints/{sprintID}/assign", h.AssignTicketsToSprint)
	r.Put("/boards/{boardID}/sprints/{sprintID}", h.UpdateSprint)
	r.Post("/boards/{boardID}/sprints/{sprintID}/start", h.StartSprint)
	r.Post("/boards/{boardID}/sprints/{sprintID}/close", h.CloseSprint)
	r.Delete("/boards/{boardID}/sprints/{sprintID}", h.DeleteSprint)
	r.Post("/boards/{boardID}/sprints/{sprintID}/delete", h.DeleteSprint)

	r.Post("/tickets/{ticketID}/sprint", h.AssignTicketToSprint)

	r.Post("/boards/{boardID}/tickets", h.CreateTicket)
	r.Post("/boards/{boardID}/backlog/tickets", h.CreateBacklogTicket)
	r.Get("/tickets/{ref}", h.TicketPage)
	r.Get("/tickets/{ticketID}/quick", h.TicketQuickView)
	r.Get("/tickets/{ticketID}/edit", h.TicketEditForm)
	r.Get("/tickets/{ticketID}/view", h.TicketBodyView)
	r.Put("/tickets/{ticketID}", h.UpdateTicket)
	r.Put("/tickets/{ticketID}/title", h.UpdateTicketTitle)
	r.Put("/tickets/{ticketID}/body", h.UpdateTicketBody)
	r.Put("/tickets/{ticketID}/priority", h.UpdateTicketPriority)
	r.Put("/tickets/{ticketID}/column", h.UpdateTicketColumn)
	r.Get("/tickets/{ticketID}/link-search", h.SearchTicketsForLink)
	r.Delete("/tickets/{ticketID}", h.DeleteTicket)
	r.Post("/tickets/{ticketID}/move", h.MoveTicket)
	r.Post("/tickets/{ticketID}/sprint-place", h.SprintPlaceTicket)
	r.Get("/tickets/{ticketID}/assignees/search", h.SearchTicketAssignees)
	r.Post("/tickets/{ticketID}/assignees", h.AddTicketAssignee)
	r.Delete("/tickets/{ticketID}/assignees/{userID}", h.RemoveTicketAssignee)

	r.Get("/search", h.Search)
	r.Get("/users/search", h.SearchUsersForMention)

	r.Post("/tickets/{ticketID}/comments", h.CreateComment)
	r.Get("/comments/{commentID}/edit", h.CommentEditForm)
	r.Get("/comments/{commentID}/view", h.CommentView)
	r.Put("/comments/{commentID}", h.UpdateComment)
	r.Delete("/comments/{commentID}", h.DeleteComment)

	r.Post("/tickets/{ticketID}/links", h.CreateLink)
	r.Delete("/tickets/{ticketID}/links/{linkID}", h.DeleteLink)

}
