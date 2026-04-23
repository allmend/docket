package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
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

	if _, err := h.links.CreateLink(r.Context(), orgID, fromID, toTicketID, relation); err != nil {
		http.Error(w, "failed to create link", http.StatusInternalServerError)
		return
	}

	links, _ := h.links.ListLinks(r.Context(), orgID, ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-links.html", map[string]any{
		"TicketID": ticketID,
		"Links":    links,
	})
}

func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		http.Error(w, "invalid ticket ID", http.StatusBadRequest)
		return
	}
	linkID, err := uuid.Parse(chi.URLParam(r, "linkID"))
	if err != nil {
		http.Error(w, "invalid link ID", http.StatusBadRequest)
		return
	}

	if err := h.links.DeleteLink(r.Context(), orgID, linkID); err != nil {
		http.Error(w, "failed to delete link", http.StatusInternalServerError)
		return
	}

	links, _ := h.links.ListLinks(r.Context(), orgID, ticketID)
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-links.html", map[string]any{
		"TicketID": ticketID,
		"Links":    links,
	})
}
