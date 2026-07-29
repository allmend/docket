package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}

	toTicketID, err := uuid.Parse(r.FormValue("to_ticket_id"))
	if err != nil {
		http.Error(w, "invalid to_ticket_id", http.StatusBadRequest)
		return
	}

	// An inverse choice ("Blocked by", "Is cloned by", …) is a UI convenience:
	// store the forward relation with the tickets swapped, so a relationship is
	// always exactly one row.
	relation, swap, ok := model.ParseRelationInput(r.FormValue("relation_type"))
	if !ok {
		http.Error(w, "invalid relation_type", http.StatusBadRequest)
		return
	}
	fromID := ticketID
	if swap {
		fromID, toTicketID = toTicketID, ticketID
	}

	if _, err := h.links.CreateLink(r.Context(), orgID, ticketID, fromID, toTicketID, relation, userID); err != nil {
		serviceError(w, err, "failed to create link")
		return
	}

	links, _ := h.links.ListLinks(r.Context(), orgID, ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-links.html", map[string]any{
		"TicketID": ticketID,
		"Links":    links,
		// Link mutations are rejected on a closed ticket, so this path is
		// only reachable while it is open.
		"Closed": nil,
	})
}

func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	linkID, ok := pathUUID(w, r, "linkID")
	if !ok {
		return
	}

	if err := h.links.DeleteLink(r.Context(), orgID, ticketID, linkID, userID); err != nil {
		serviceError(w, err, "failed to delete link")
		return
	}

	links, _ := h.links.ListLinks(r.Context(), orgID, ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-links.html", map[string]any{
		"TicketID": ticketID,
		"Links":    links,
		// Link mutations are rejected on a closed ticket, so this path is
		// only reachable while it is open.
		"Closed": nil,
	})
}
