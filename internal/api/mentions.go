package api

import (
	"encoding/json"
	"net/http"

	"github.com/allmend/docket/internal/service"
)

// Autocomplete endpoints for @user and #ticket mentions in the rich editor.

func (h *Handler) SearchUsersForMention(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	q := r.URL.Query().Get("q")
	if len(q) < 1 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}
	users, _ := h.tickets.SearchUsers(r.Context(), orgID, q)
	type result struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}
	out := make([]result, 0, len(users))
	for _, u := range users {
		out = append(out, result{ID: u.ID.String(), Username: u.Username, Name: u.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) SearchTicketsForMention(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	q := r.URL.Query().Get("q")
	if len(q) < 1 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}
	tickets, _ := h.tickets.SearchTicketsForMention(r.Context(), orgID, q)
	type result struct {
		DisplayID string `json:"display_id"`
		Title     string `json:"title"`
	}
	out := make([]result, 0, len(tickets))
	for _, t := range tickets {
		out = append(out, result{DisplayID: t.DisplayID(), Title: t.Title})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
