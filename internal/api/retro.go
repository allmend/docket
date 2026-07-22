package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/google/uuid"
)

func (h *Handler) SprintReviewPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}
	data, err := h.retro.GetSprintReviewData(r.Context(), orgID, board.ID, sprintID)
	if err != nil {
		http.Error(w, "failed to load sprint review", http.StatusInternalServerError)
		return
	}
	h.render(w, "sprint-review.html", h.pageData(r, map[string]any{
		"ReviewData": data,
		"Team":       team,
	}))
}

func (h *Handler) CloseSprintAndStartRetro(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	sprintID, ok := pathUUID(w, r, "sprintID")
	if !ok {
		return
	}

	if _, err := h.boards.CloseSprint(r.Context(), orgID, sprintID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	board, _ := h.boards.GetBoard(r.Context(), orgID, boardID)
	slug := h.teamSlugForBoard(r, orgID, board)

	rb, err := h.retro.GetOrCreateRetroBoard(r.Context(), orgID, boardID, &sprintID)
	if err != nil {
		if slug != "" {
			http.Redirect(w, r, "/workspaces/"+slug+"/board", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
		return
	}
	if slug != "" {
		http.Redirect(w, r, "/workspaces/"+slug+"/retros/"+rb.ID.String(), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (h *Handler) RetrosListPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	retros, err := h.retro.ListRetros(r.Context(), orgID, board.ID)
	if err != nil {
		http.Error(w, "failed to list retros", http.StatusInternalServerError)
		return
	}
	h.render(w, "retros.html", h.pageData(r, map[string]any{
		"Retros": retros,
		"Board":  board,
		"Team":   team,
	}))
}

func (h *Handler) CloseRetroBoard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	if err := h.retro.CloseRetroBoard(r.Context(), orgID, retroBoardID, userID); err != nil {
		http.Error(w, "failed to close retro", http.StatusInternalServerError)
		return
	}
	board, _ := h.boards.GetBoard(r.Context(), orgID, boardID)
	slug := h.teamSlugForBoard(r, orgID, board)
	if slug != "" {
		http.Redirect(w, r, "/workspaces/"+slug+"/retros", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (h *Handler) RetroBoardPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	team, board, ok := h.boardByTeamSlug(w, r, orgID)
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}

	rb, err := h.retro.GetRetroBoard(r.Context(), orgID, retroBoardID)
	if err != nil {
		http.Error(w, "retro not found", http.StatusNotFound)
		return
	}

	view, err := h.retro.GetRetroView(r.Context(), orgID, board.ID, rb.SprintID)
	if err != nil {
		http.Error(w, "failed to load retro", http.StatusInternalServerError)
		return
	}

	h.render(w, "retro.html", h.pageData(r, map[string]any{
		"RetroView":     view,
		"CurrentUserID": userID.String(),
		"Team":          team,
		"Board":         board,
	}))
}

func (h *Handler) CreateRetroCard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}

	col := model.RetroColumn(r.FormValue("column"))
	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}

	if _, err := h.retro.CreateCard(r.Context(), orgID, boardID, retroBoardID, userID, col, body); err != nil {
		http.Error(w, "failed to create card", http.StatusInternalServerError)
		return
	}

	h.renderRetroColumns(w, r, orgID, boardID, retroBoardID, userID)
}

func (h *Handler) DeleteRetroCard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	cardID, ok := pathUUID(w, r, "cardID")
	if !ok {
		return
	}

	if err := h.retro.DeleteCard(r.Context(), orgID, cardID, userID); err != nil {
		http.Error(w, "not found or not yours", http.StatusForbidden)
		return
	}

	h.renderRetroColumns(w, r, orgID, boardID, retroBoardID, userID)
}

func (h *Handler) AssignRetroCardOwner(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	cardID, ok := pathUUID(w, r, "cardID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	ownerID, err := uuid.Parse(r.FormValue("owner_id"))
	if err != nil {
		http.Error(w, "invalid owner ID", http.StatusBadRequest)
		return
	}

	if _, err := h.retro.AssignActionItemOwner(r.Context(), orgID, cardID, ownerID, userID); err != nil {
		http.Error(w, "failed to assign owner: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.renderRetroColumns(w, r, orgID, boardID, retroBoardID, userID)
}

func (h *Handler) StackRetroCard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	cardID, ok := pathUUID(w, r, "cardID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	parentID, err := uuid.Parse(r.FormValue("parent_id"))
	if err != nil {
		http.Error(w, "invalid parent_id", http.StatusBadRequest)
		return
	}
	if err := h.retro.StackCard(r.Context(), orgID, cardID, parentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.renderRetroColumns(w, r, orgID, boardID, retroBoardID, userID)
}

func (h *Handler) UnstackRetroCard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	retroBoardID, ok := pathUUID(w, r, "retroBoardID")
	if !ok {
		return
	}
	cardID, ok := pathUUID(w, r, "cardID")
	if !ok {
		return
	}
	if err := h.retro.UnstackCard(r.Context(), orgID, cardID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.renderRetroColumns(w, r, orgID, boardID, retroBoardID, userID)
}

func (h *Handler) renderRetroColumns(w http.ResponseWriter, r *http.Request, orgID, boardID, retroBoardID, userID uuid.UUID) {
	rb, err := h.retro.GetRetroBoard(r.Context(), orgID, retroBoardID)
	if err != nil {
		http.Error(w, "retro not found", http.StatusNotFound)
		return
	}
	view, err := h.retro.GetRetroView(r.Context(), orgID, boardID, rb.SprintID)
	if err != nil {
		http.Error(w, "failed to reload retro", http.StatusInternalServerError)
		return
	}
	h.render(w, "retro-columns.html", map[string]any{
		"RetroView":     view,
		"CurrentUserID": userID.String(),
	})
}
