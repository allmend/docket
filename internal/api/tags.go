package api

import (
	"encoding/json"
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

// --- Tag handlers ---

func filterUnusedTags(all, applied []model.Tag) []model.Tag {
	used := make(map[uuid.UUID]bool, len(applied))
	for _, t := range applied {
		used[t.ID] = true
	}
	out := make([]model.Tag, 0, len(all))
	for _, t := range all {
		if !used[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

func (h *Handler) BoardTagsJSON(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	type tagResult struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	out := make([]tagResult, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagResult{ID: t.ID.String(), Name: t.Name, Color: t.Color})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) BoardTagsPanel(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) CreateTag(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	color := r.FormValue("color")
	if name == "" || color == "" {
		http.Error(w, "name and color required", http.StatusBadRequest)
		return
	}
	if _, err := h.boards.CreateTag(r.Context(), orgID, boardID, name, color); err != nil {
		http.Error(w, "failed to create tag", http.StatusInternalServerError)
		return
	}
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	tagID, ok := pathUUID(w, r, "tagID")
	if !ok {
		return
	}
	_ = h.boards.DeleteTag(r.Context(), orgID, tagID)
	tags, _ := h.boards.ListBoardTags(r.Context(), orgID, boardID)
	h.render(w, "board-tags-panel.html", map[string]any{
		"BoardID": boardID,
		"Tags":    tags,
	})
}

func (h *Handler) AddTagToTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tagID, err := uuid.Parse(r.FormValue("tag_id"))
	if err != nil {
		http.Error(w, "invalid tag ID", http.StatusBadRequest)
		return
	}
	_ = h.boards.AddTagToTicket(r.Context(), orgID, ticketID, tagID)
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticketID)
	var boardTags []model.Tag
	if ticket != nil {
		all, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
		boardTags = filterUnusedTags(all, tags)
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-tags.html", map[string]any{
		"Ticket":    ticket,
		"Tags":      tags,
		"BoardTags": boardTags,
	})
}

func (h *Handler) RemoveTagFromTicket(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}
	tagID, ok := pathUUID(w, r, "tagID")
	if !ok {
		return
	}
	_ = h.boards.RemoveTagFromTicket(r.Context(), orgID, ticketID, tagID)
	ticket, _ := h.tickets.GetTicket(r.Context(), orgID, ticketID)
	tags, _ := h.boards.ListTicketTags(r.Context(), orgID, ticketID)
	var boardTags []model.Tag
	if ticket != nil {
		all, _ := h.boards.ListBoardTags(r.Context(), orgID, ticket.BoardID)
		boardTags = filterUnusedTags(all, tags)
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	h.render(w, "ticket-tags.html", map[string]any{
		"Ticket":    ticket,
		"Tags":      tags,
		"BoardTags": boardTags,
	})
}
