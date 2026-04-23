package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CreateGlobalTicket handles the "New Ticket" button in the navbar.
// The user picks a team; the ticket lands in that team's board backlog.
func (h *Handler) CreateGlobalTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	teamID, err := uuid.Parse(r.FormValue("team_id"))
	if err != nil {
		http.Error(w, "invalid team ID", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	priority := model.Priority(r.FormValue("priority"))
	if priority == "" {
		priority = model.PriorityNone
	}

	board, err := h.boards.GetBoardByTeam(r.Context(), orgID, teamID)
	if err != nil {
		http.Error(w, "team has no board", http.StatusBadRequest)
		return
	}

	cols, err := h.boards.ListColumns(r.Context(), orgID, board.ID)
	if err != nil || len(cols) == 0 {
		http.Error(w, "board has no columns", http.StatusBadRequest)
		return
	}

	ticket, err := h.tickets.CreateTicketInTeam(r.Context(), orgID, board.ID, cols[0].ID, userID, teamID, title, r.FormValue("body"), priority)
	if err != nil {
		http.Error(w, "failed to create ticket", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/tickets/"+ticket.DisplayID(), http.StatusSeeOther)
}

func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
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

	columnID, err := uuid.Parse(r.FormValue("column_id"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}

	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	priority := model.Priority(r.FormValue("priority"))
	if priority == "" {
		priority = model.PriorityNone
	}

	ticket, err := h.tickets.CreateTicket(r.Context(), orgID, boardID, columnID, userID, title, r.FormValue("body"), priority)
	if err != nil {
		http.Error(w, "failed to create ticket", http.StatusInternalServerError)
		return
	}

	columns, _ := h.boards.ListColumns(r.Context(), orgID, boardID)
	w.Header().Set("HX-Trigger", `{"open-modal":{"id":"ticket-detail"}}`)
	h.render(w, "ticket-detail.html", map[string]any{
		"Ticket":    ticket,
		"Comments":  []model.Comment{},
		"History":   []model.HistoryEntry{},
		"Assignees": []model.User{},
		"Columns":   columns,
	})
}

// CreateBacklogTicket creates a ticket directly into the backlog (scrum boards).
// It lands in the first column with sprint_id NULL, then redirects to the backlog page.
func (h *Handler) CreateBacklogTicket(w http.ResponseWriter, r *http.Request) {
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
	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	priority := model.Priority(r.FormValue("priority"))
	if priority == "" {
		priority = model.PriorityNone
	}

	// Resolve first column for this board.
	cols, err := h.boards.ListColumns(r.Context(), orgID, boardID)
	if err != nil || len(cols) == 0 {
		http.Error(w, "board has no columns", http.StatusBadRequest)
		return
	}

	ticket, err := h.tickets.CreateTicket(r.Context(), orgID, boardID, cols[0].ID, userID, title, r.FormValue("body"), priority)
	if err != nil {
		http.Error(w, "failed to create ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"open-modal":{"id":"ticket-detail"}}`)
	h.render(w, "ticket-detail.html", map[string]any{
		"Ticket":    ticket,
		"Comments":  []model.Comment{},
		"History":   []model.HistoryEntry{},
		"Assignees": []model.User{},
		"Columns":   cols,
	})
}

// TicketQuickView renders the lightweight modal partial (triggered by card click on the board).
func (h *Handler) TicketQuickView(w http.ResponseWriter, r *http.Request) {
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

	comments, _ := h.comments.ListComments(r.Context(), orgID, ticket.ID)
	history, _ := h.comments.ListHistory(r.Context(), ticket.ID)
	assignees, _ := h.tickets.ListAssignees(r.Context(), ticket.ID)
	columns, _ := h.boards.ListColumns(r.Context(), orgID, ticket.BoardID)
	links, _ := h.links.ListLinks(r.Context(), orgID, ticket.ID)

	h.render(w, "ticket-detail.html", map[string]any{
		"Ticket":    ticket,
		"Comments":  comments,
		"History":   history,
		"Assignees": assignees,
		"Columns":   columns,
		"Links":     links,
	})
}

// TicketPage renders the full permalink page for a ticket, resolved by KEY-N ref.
// URL: /tickets/BE-42
func (h *Handler) TicketPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ref := chi.URLParam(r, "ref")

	ticket, err := h.resolveTicketRef(r, orgID, ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ticket == nil {
		http.Error(w, fmt.Sprintf("%s not found", ref), http.StatusNotFound)
		return
	}

	// Load board so we can render the breadcrumb.
	board, _ := h.boards.GetBoard(r.Context(), orgID, ticket.BoardID)
	var team *model.Team
	if board != nil && board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
	}

	comments, _ := h.comments.ListComments(r.Context(), orgID, ticket.ID)
	history, _ := h.comments.ListHistory(r.Context(), ticket.ID)
	assignees, _ := h.tickets.ListAssignees(r.Context(), ticket.ID)
	columns, _ := h.boards.ListColumns(r.Context(), orgID, ticket.BoardID)
	links, _ := h.links.ListLinks(r.Context(), orgID, ticket.ID)

	h.render(w, "ticket-page.html", h.pageData(r, map[string]any{
		"Ticket":    ticket,
		"Board":     board,
		"Team":      team,
		"Comments":  comments,
		"History":   history,
		"Assignees": assignees,
		"Columns":   columns,
		"Links":     links,
	}))
}

// TicketBodyView renders the read-only ticket body fragment (used by cancel in edit form).
func (h *Handler) TicketBodyView(w http.ResponseWriter, r *http.Request) {
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

	board, _ := h.boards.GetBoard(r.Context(), orgID, ticket.BoardID)
	var team *model.Team
	if board != nil && board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
	}

	h.render(w, "ticket-body.html", map[string]any{
		"Ticket":  ticket,
		"Board":   board,
		"Team": team,
	})
}

// TicketEditForm returns the inline edit form fragment (HTMX swap into the page).
func (h *Handler) TicketEditForm(w http.ResponseWriter, r *http.Request) {
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

	h.render(w, "ticket-edit-form.html", ticket)
}

func (h *Handler) UpdateTicket(w http.ResponseWriter, r *http.Request) {
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

	var assigneeID *uuid.UUID
	if raw := r.FormValue("assignee_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err == nil {
			assigneeID = &id
		}
	}

	userID := service.UserIDFromContext(r.Context())
	ticket, err := h.tickets.UpdateTicket(
		r.Context(), orgID, ticketID, userID,
		r.FormValue("title"), r.FormValue("body"),
		model.Priority(r.FormValue("priority")),
		assigneeID,
	)
	if err != nil {
		http.Error(w, "failed to update ticket", http.StatusInternalServerError)
		return
	}

	board, _ := h.boards.GetBoard(r.Context(), orgID, ticket.BoardID)
	var team *model.Team
	if board != nil && board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
	}

	// From the ticket page edit form: re-render the body section.
	h.render(w, "ticket-body.html", map[string]any{
		"Ticket":  ticket,
		"Board":   board,
		"Team": team,
	})
}

func (h *Handler) DeleteTicket(w http.ResponseWriter, r *http.Request) {
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

	if err := h.tickets.DeleteTicket(r.Context(), orgID, ticketID); err != nil {
		http.Error(w, "failed to delete ticket", http.StatusInternalServerError)
		return
	}

	// Redirect back to the board.
	board, _ := h.boards.GetBoard(r.Context(), orgID, ticket.BoardID)
	redirectTo := "/boards"
	if board != nil {
		redirectTo = "/boards/" + board.ID.String()
	}
	w.Header().Set("HX-Redirect", redirectTo)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) MoveTicket(w http.ResponseWriter, r *http.Request) {
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

	columnID, err := uuid.Parse(r.FormValue("column_id"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}

	prevPos, _ := strconv.ParseFloat(r.FormValue("prev_pos"), 64)
	nextPos, _ := strconv.ParseFloat(r.FormValue("next_pos"), 64)

	if err := h.tickets.MoveTicket(r.Context(), orgID, ticketID, columnID, prevPos, nextPos); err != nil {
		http.Error(w, "failed to move ticket", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTicketPriority(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.UpdatePriority(r.Context(), orgID, ticketID, userID, model.Priority(r.FormValue("priority")))
	if err != nil {
		http.Error(w, "failed to update priority", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-priority-select.html", ticket)
}

func (h *Handler) SearchTicketAssignees(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	q := r.URL.Query().Get("q")
	users, _ := h.tickets.SearchUsers(r.Context(), orgID, q)
	current, _ := h.tickets.ListAssignees(r.Context(), ticketID)
	currentSet := make(map[uuid.UUID]bool, len(current))
	for _, u := range current {
		currentSet[u.ID] = true
	}
	var filtered []model.User
	for _, u := range users {
		if !currentSet[u.ID] {
			filtered = append(filtered, u)
		}
	}
	h.render(w, "ticket-assignee-results.html", map[string]any{
		"TicketID": ticketID,
		"Users":    filtered,
	})
}

func (h *Handler) AddTicketAssignee(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	actorID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
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
	if err := h.tickets.AddAssignee(r.Context(), orgID, ticketID, userID, actorID); err != nil {
		http.Error(w, "failed to add assignee", http.StatusInternalServerError)
		return
	}
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	assignees, _ := h.tickets.ListAssignees(r.Context(), ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-assignees.html", map[string]any{
		"Ticket":    ticket,
		"Assignees": assignees,
	})
}

func (h *Handler) RemoveTicketAssignee(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	actorID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user ID", http.StatusBadRequest)
		return
	}
	if err := h.tickets.RemoveAssignee(r.Context(), orgID, ticketID, userID, actorID); err != nil {
		http.Error(w, "failed to remove assignee", http.StatusInternalServerError)
		return
	}
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	assignees, _ := h.tickets.ListAssignees(r.Context(), ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-assignees.html", map[string]any{
		"Ticket":    ticket,
		"Assignees": assignees,
	})
}

func (h *Handler) MyIssues(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	tickets, err := h.tickets.ListMyTickets(r.Context(), orgID, userID)
	if err != nil {
		http.Error(w, "failed to load issues", http.StatusInternalServerError)
		return
	}

	// Build column name lookup across all boards the tickets belong to.
	colNames := make(map[uuid.UUID]string)
	boardsSeen := make(map[uuid.UUID]bool)
	for _, t := range tickets {
		if !boardsSeen[t.BoardID] {
			boardsSeen[t.BoardID] = true
			if cols, err := h.boards.ListColumns(r.Context(), orgID, t.BoardID); err == nil {
				for _, c := range cols {
					colNames[c.ID] = c.Name
				}
			}
		}
	}

	// Group by column name (= status).
	type issueGroup struct {
		Name    string
		Tickets []model.Ticket
	}
	seen := make(map[uuid.UUID]int) // columnID → index in groups
	var groups []issueGroup
	for _, t := range tickets {
		name := colNames[t.ColumnID]
		if name == "" {
			name = "No status"
		}
		if idx, ok := seen[t.ColumnID]; ok {
			groups[idx].Tickets = append(groups[idx].Tickets, t)
		} else {
			seen[t.ColumnID] = len(groups)
			groups = append(groups, issueGroup{Name: name, Tickets: []model.Ticket{t}})
		}
	}

	h.render(w, "my-issues.html", h.pageData(r, map[string]any{
		"Groups": groups,
		"Total":  len(tickets),
	}))
}


func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	query := r.URL.Query().Get("q")

	tickets, err := h.tickets.Search(r.Context(), orgID, query)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	h.render(w, "search-results.html", map[string]any{
		"Query":   query,
		"Tickets": tickets,
	})
}

func (h *Handler) UpdateTicketColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	columnID, err := uuid.Parse(r.FormValue("column_id"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.MoveToColumn(r.Context(), orgID, ticketID, columnID, userID)
	if err != nil {
		http.Error(w, "failed to move ticket", http.StatusInternalServerError)
		return
	}

	// If this ticket is on a scrum board and has no sprint assigned, auto-assign it
	// to the active or planning sprint so the column change persists on the board view.
	if ticket.SprintID == nil {
		if board, _ := h.boards.GetBoard(r.Context(), orgID, ticket.BoardID); board != nil && board.Mode.IsScrum() {
			_ = h.boards.AutoAssignToSprint(r.Context(), orgID, ticket.ID, ticket.BoardID)
		}
	}

	w.Header().Set("HX-Trigger", `{"ticketUpdated":true,"boardUpdated":true}`)
	w.WriteHeader(http.StatusNoContent)
}

// SprintPlaceTicket assigns a ticket to a sprint and moves it to a target column.
// Called when dragging a ticket from the virtual backlog column into a sprint column on the board.
func (h *Handler) SprintPlaceTicket(w http.ResponseWriter, r *http.Request) {
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
	sprintID, err := uuid.Parse(r.FormValue("sprint_id"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	columnID, err := uuid.Parse(r.FormValue("column_id"))
	if err != nil {
		http.Error(w, "invalid column ID", http.StatusBadRequest)
		return
	}
	prevPos, _ := strconv.ParseFloat(r.FormValue("prev_pos"), 64)
	nextPos, _ := strconv.ParseFloat(r.FormValue("next_pos"), 64)

	if err := h.boards.AssignTicketToSprint(r.Context(), orgID, ticketID, &sprintID); err != nil {
		http.Error(w, "assign to sprint failed", http.StatusInternalServerError)
		return
	}
	if err := h.tickets.MoveTicket(r.Context(), orgID, ticketID, columnID, prevPos, nextPos); err != nil {
		http.Error(w, "move failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTicketTitle(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.UpdateTicketTitle(r.Context(), orgID, ticketID, userID, title)
	if err != nil {
		http.Error(w, "failed to update title", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-title.html", ticket)
}

func (h *Handler) UpdateTicketBody(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.UpdateTicketBody(r.Context(), orgID, ticketID, userID, r.FormValue("body"))
	if err != nil {
		http.Error(w, "failed to update description", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-body-section.html", ticket)
}

func (h *Handler) SearchTicketsForLink(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	q := r.URL.Query().Get("q")
	tickets, err := h.tickets.SearchTicketsForLink(r.Context(), orgID, ticketID, q)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	h.render(w, "link-search-results.html", map[string]any{
		"Tickets": tickets,
		"Query":   q,
	})
}

// resolveTicketRef resolves either a KEY-N ref (e.g. "BE-42") or a raw UUID.
func (h *Handler) resolveTicketRef(r *http.Request, orgID uuid.UUID, ref string) (*model.Ticket, error) {
	// Try UUID first.
	if id, err := uuid.Parse(ref); err == nil {
		return h.tickets.GetTicket(r.Context(), orgID, id)
	}
	// Try KEY-N.
	i := strings.LastIndex(ref, "-")
	if i <= 0 || i == len(ref)-1 {
		return nil, fmt.Errorf("invalid ticket ref %q", ref)
	}
	n, err := strconv.Atoi(ref[i+1:])
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("invalid ticket ref %q", ref)
	}
	key := strings.ToUpper(ref[:i])
	return h.tickets.GetByRef(r.Context(), orgID, key, n)
}
