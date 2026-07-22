package api

import (
	"net/http"

	"github.com/allmend/docket/internal/service"
)

// BoardDodPanel renders the DoD management panel for a board.
func (h *Handler) BoardDodPanel(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
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
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
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
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	itemID, ok := pathUUID(w, r, "itemID")
	if !ok {
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
	ticket, orgID, ok := h.ticketFromPath(w, r)
	if !ok {
		return
	}
	items, _ := h.boards.GetTicketDod(r.Context(), orgID, ticket.BoardID, ticket.ID)
	h.render(w, "ticket-dod-partial.html", map[string]any{
		"TicketID": ticket.ID,
		"BoardID":  ticket.BoardID,
		"Items":    items,
		"IsClosed": ticket.ClosedAt != nil,
	})
}

// ToggleDodCheck checks or unchecks a DoD item for a ticket.
func (h *Handler) ToggleDodCheck(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	itemID, ok := pathUUID(w, r, "itemID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	checked := r.FormValue("checked") == "true"
	if err := h.boards.ToggleDodCheck(r.Context(), orgID, ticketID, itemID, checked); err != nil {
		serviceError(w, err, "failed to toggle check")
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
