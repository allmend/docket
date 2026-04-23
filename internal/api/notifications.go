package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/allmend/docket/internal/service"
)

func (h *Handler) NavUnreadCount(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	count, err := h.notifications.UnreadCount(r.Context(), orgID, userID)
	if err != nil || count == 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<span id="nav-unread-count"></span>`)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span id="nav-unread-count" class="ml-auto text-[10px] font-bold bg-primary text-base-100 rounded-full min-w-[16px] h-4 flex items-center justify-center px-1">%d</span>`, count)
}

func (h *Handler) Inbox(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())

	notifications, _ := h.notifications.List(r.Context(), orgID, userID)
	_ = h.notifications.MarkAllRead(r.Context(), orgID, userID)

	h.render(w, "inbox.html", h.pageData(r, map[string]any{
		"Notifications": notifications,
	}))
}

func (h *Handler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	_ = h.notifications.MarkAllRead(r.Context(), orgID, userID)
	w.WriteHeader(http.StatusNoContent)
}

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
		Username string `json:"username"`
		Name     string `json:"name"`
	}
	out := make([]result, 0, len(users))
	for _, u := range users {
		out = append(out, result{Username: u.Username, Name: u.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
