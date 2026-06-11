package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/allmend/docket/internal/metrics"
	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CreateGlobalTicket handles the "New Ticket" modal form submission.
// The workspace chip pre-selects the current workspace; the ticket lands in its backlog.
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

	for _, raw := range r.Form["assignee_id"] {
		uid, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		_ = h.tickets.AddAssignee(r.Context(), orgID, ticket.ID, uid, userID)
	}
	for _, raw := range r.Form["tag_id"] {
		uid, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		_ = h.boards.AddTagToTicket(r.Context(), orgID, ticket.ID, uid)
	}
	if raw := r.FormValue("story_points"); raw != "" {
		if pts, err := strconv.ParseFloat(raw, 64); err == nil {
			_, _ = h.tickets.UpdatePoints(r.Context(), orgID, ticket.ID, userID, &pts)
		}
	}

	if r.FormValue("create_more") == "true" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("HX-Redirect", "/tickets/"+ticket.DisplayID())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
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
		"Tags":      []model.Tag{},
		"BoardTags": []model.Tag{},
	})
}

// CreateBacklogTicket creates a ticket directly into the backlog (scrum boards).
// It lands in the first column with sprint_id NULL, then redirects to the backlog page.
func (h *Handler) CreateBacklogTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
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
		"Tags":      []model.Tag{},
		"BoardTags": []model.Tag{},
	})
}

// TicketQuickView renders the lightweight modal partial (triggered by card click on the board).
func (h *Handler) TicketQuickView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticket.ID)
	allBoardTags, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
	boardTags := filterUnusedTags(allBoardTags, tags)
	dodItems, _ := h.boards.ListDodItems(r.Context(), orgID, ticket.BoardID)
	sprintActive := false
	if ticket.SprintID != nil {
		if sp, err := h.boards.GetSprint(r.Context(), orgID, *ticket.SprintID); err == nil {
			sprintActive = sp.Status.IsActive()
		}
	}

	h.render(w, "ticket-detail.html", map[string]any{
		"Ticket":       ticket,
		"Comments":     comments,
		"History":      history,
		"Assignees":    assignees,
		"Columns":      columns,
		"Links":        links,
		"Tags":         tags,
		"BoardTags":    boardTags,
		"DodItems":     dodItems,
		"SprintActive": sprintActive,
		"CurrentUser":  h.auth.GetCurrentUser(r.Context()),
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
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticket.ID)
	allBoardTags, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
	boardTags := filterUnusedTags(allBoardTags, tags)
	dodItems, _ := h.boards.ListDodItems(r.Context(), orgID, ticket.BoardID)
	sprintActive := false
	if ticket.SprintID != nil {
		if sp, err := h.boards.GetSprint(r.Context(), orgID, *ticket.SprintID); err == nil {
			sprintActive = sp.Status.IsActive()
		}
	}

	h.render(w, "ticket-page.html", h.pageData(r, map[string]any{
		"Ticket":       ticket,
		"Board":        board,
		"Team":         team,
		"Comments":     comments,
		"History":      history,
		"Assignees":    assignees,
		"Columns":      columns,
		"Links":        links,
		"Tags":         tags,
		"BoardTags":    boardTags,
		"DodItems":     dodItems,
		"SprintActive": sprintActive,
	}))
}

// TicketRefineView renders the refinement detail pane for a single ticket.
// Used by the backlog refinement side-by-side view (right pane).
func (h *Handler) TicketRefineView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}

	ticket, err := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	if err != nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticket.ID)
	allBoardTags, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
	boardTags := filterUnusedTags(allBoardTags, tags)

	h.render(w, "ticket-refine.html", map[string]any{
		"Ticket":    ticket,
		"Tags":      tags,
		"BoardTags": boardTags,
	})
}

// TicketBodyView renders the read-only ticket body fragment (used by cancel in edit form).
func (h *Handler) TicketBodyView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
		"Ticket": ticket,
		"Board":  board,
		"Team":   team,
	})
}

// TicketEditForm returns the inline edit form fragment (HTMX swap into the page).
func (h *Handler) TicketEditForm(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
		"Ticket": ticket,
		"Board":  board,
		"Team":   team,
	})
}

func (h *Handler) CloseTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	reason := r.FormValue("reason")
	if r.FormValue("reason") == "other" {
		reason = r.FormValue("reason_note")
	}
	if reason == "" {
		reason = "Closed"
	}
	if _, err := h.tickets.CloseTicket(r.Context(), orgID, ticketID, userID, reason); err != nil {
		http.Error(w, "failed to close ticket", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReopenTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if _, err := h.tickets.ReopenTicket(r.Context(), orgID, ticketID, userID); err != nil {
		http.Error(w, "failed to reopen ticket", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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

	newPos, err := h.tickets.MoveTicket(r.Context(), orgID, ticketID, columnID, prevPos, nextPos)
	if err != nil {
		http.Error(w, "failed to move ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("X-New-Position", strconv.FormatFloat(newPos, 'f', -1, 64))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTicketPriority(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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

func (h *Handler) UpdateTicketPoints(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var points *float64
	if v := r.FormValue("story_points"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n < 0 {
			http.Error(w, "invalid story points", http.StatusBadRequest)
			return
		}
		points = &n
	}
	if _, err := h.tickets.UpdatePoints(r.Context(), orgID, ticketID, userID, points); err != nil {
		http.Error(w, "failed to update story points", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SearchTicketAssignees(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
		"Ticket":      ticket,
		"Assignees":   assignees,
		"CurrentUser": h.auth.GetCurrentUser(r.Context()),
	})
}

func (h *Handler) RemoveTicketAssignee(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	actorID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	userID, ok := pathUUID(w, r, "userID")
	if !ok {
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
		"Ticket":      ticket,
		"Assignees":   assignees,
		"CurrentUser": h.auth.GetCurrentUser(r.Context()),
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

	totalPts := 0
	for _, t := range tickets {
		if t.StoryPoints != nil {
			totalPts += int(*t.StoryPoints)
		}
	}

	h.render(w, "my-issues.html", h.pageData(r, map[string]any{
		"Groups":      groups,
		"Total":       len(tickets),
		"TotalPoints": totalPts,
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	newPos, err := h.tickets.MoveTicket(r.Context(), orgID, ticketID, columnID, prevPos, nextPos)
	if err != nil {
		http.Error(w, "move failed", http.StatusInternalServerError)
		return
	}
	if r.FormValue("unplanned") == "1" {
		if t, err := h.tickets.GetTicket(r.Context(), orgID, ticketID); err == nil && t.StoryPoints != nil {
			metrics.SprintUnplannedPoints.WithLabelValues(orgID.String(), sprintID.String()).Add(float64(*t.StoryPoints))
		}
	}
	w.Header().Set("X-New-Position", strconv.FormatFloat(newPos, 'f', -1, 64))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTicketTitle(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
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

func (h *Handler) ToggleACCheckbox(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	var index int
	if _, err := fmt.Sscanf(chi.URLParam(r, "index"), "%d", &index); err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.ToggleACCheckbox(r.Context(), orgID, ticketID, index)
	if err != nil {
		http.Error(w, "failed to toggle checkbox", http.StatusInternalServerError)
		return
	}
	h.render(w, "ticket-ac-section.html", ticket)
}

func (h *Handler) UpdateTicketAC(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.UpdateTicketAC(r.Context(), orgID, ticketID, userID, r.FormValue("ac"))
	if err != nil {
		http.Error(w, "failed to update acceptance criteria", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-ac-section.html", ticket)
}

func (h *Handler) AddACItem(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.AddACItem(r.Context(), orgID, ticketID, text)
	if err != nil {
		http.Error(w, "failed to add item", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-ac-section.html", ticket)
}

func (h *Handler) DeleteACItem(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	var index int
	if _, err := fmt.Sscanf(chi.URLParam(r, "index"), "%d", &index); err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	ticket, err := h.tickets.DeleteACItem(r.Context(), orgID, ticketID, index)
	if err != nil {
		http.Error(w, "failed to delete item", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-ac-section.html", ticket)
}

func (h *Handler) SearchTicketsForLink(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	tickets, err := h.tickets.SearchTicketsForLink(r.Context(), orgID, ticketID, q)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	h.render(w, "link-search-results.html", map[string]any{
		"Tickets":  tickets,
		"Query":    q,
		"TicketID": ticketID,
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
