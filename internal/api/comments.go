package api

import (
	"net/http"
	"regexp"

	"github.com/allmend/docket/internal/service"
)

var mentionRe = regexp.MustCompile(`@(\w+)`)

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	ticketID, ok := pathUUID(w, r, "ticketID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}
	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	comment, err := h.comments.CreateComment(r.Context(), orgID, ticketID, userID, body)
	if err != nil {
		http.Error(w, "failed to create comment", http.StatusInternalServerError)
		return
	}

	_ = h.comments.AppendHistory(r.Context(), ticketID, userID, comment.AuthorName, "comment", "", "")

	if h.notifications != nil {
		actorName := comment.AuthorName
		// notify each assignee (except the commenter)
		if assignees, err := h.tickets.ListAssignees(r.Context(), ticketID); err == nil {
			for _, a := range assignees {
				if a.ID != userID {
					h.notifications.Notify(r.Context(), orgID, a.ID, &ticketID, &userID, actorName, "comment")
				}
			}
		}
		// notify @mentioned users
		for _, match := range mentionRe.FindAllStringSubmatch(body, -1) {
			if u, err := h.tickets.GetUserByUsername(r.Context(), orgID, match[1]); err == nil && u != nil && u.ID != userID {
				h.notifications.Notify(r.Context(), orgID, u.ID, &ticketID, &userID, actorName, "mentioned")
			}
		}
	}

	h.render(w, "comment.html", comment)
}

func (h *Handler) CommentEditForm(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	commentID, ok := pathUUID(w, r, "commentID")
	if !ok {
		return
	}

	comment, err := h.comments.GetComment(r.Context(), orgID, commentID)
	if err != nil {
		http.Error(w, "comment not found", http.StatusNotFound)
		return
	}

	h.render(w, "comment-edit-form.html", comment)
}

func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	commentID, ok := pathUUID(w, r, "commentID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}

	comment, err := h.comments.UpdateComment(r.Context(), orgID, commentID, r.FormValue("body"))
	if err != nil {
		http.Error(w, "failed to update comment", http.StatusInternalServerError)
		return
	}

	h.render(w, "comment.html", comment)
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	commentID, ok := pathUUID(w, r, "commentID")
	if !ok {
		return
	}

	if err := h.comments.DeleteComment(r.Context(), orgID, commentID); err != nil {
		http.Error(w, "failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK) // empty response — hx-swap="outerHTML" removes the element
}

func (h *Handler) CommentView(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	commentID, ok := pathUUID(w, r, "commentID")
	if !ok {
		return
	}

	comment, err := h.comments.GetComment(r.Context(), orgID, commentID)
	if err != nil {
		http.Error(w, "comment not found", http.StatusNotFound)
		return
	}

	h.render(w, "comment.html", comment)
}
