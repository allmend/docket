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

	rawRelation := r.FormValue("relation_type")
	fromID, relation := ticketID, model.RelationType(rawRelation)

	// "Blocked by" is a UI convenience: store as ENG-11 blocks me (swap from/to).
	if rawRelation == "blocked_by_inverse" {
		fromID, toTicketID = toTicketID, ticketID
		relation = model.RelationBlocks
	}

	switch relation {
	case model.RelationBlocks, model.RelationDependsOn, model.RelationDuplicates, model.RelationRelatesTo:
	default:
		http.Error(w, "invalid relation_type", http.StatusBadRequest)
		return
	}

	if _, err := h.links.CreateLink(r.Context(), orgID, fromID, toTicketID, relation, userID); err != nil {
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
