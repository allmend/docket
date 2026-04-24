package api

import (
	"net/http"

	"github.com/allmend/docket/internal/model"
	"github.com/allmend/docket/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) SprintReviewPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}
	data, err := h.retro.GetSprintReviewData(r.Context(), orgID, boardID, sprintID)
	if err != nil {
		http.Error(w, "failed to load sprint review", http.StatusInternalServerError)
		return
	}
	h.render(w, "sprint-review.html", h.pageData(r, map[string]any{
		"ReviewData": data,
	}))
}

func (h *Handler) CloseSprintAndStartRetro(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "sprintID"))
	if err != nil {
		http.Error(w, "invalid sprint ID", http.StatusBadRequest)
		return
	}

	if _, err := h.boards.CloseSprint(r.Context(), orgID, sprintID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create (or find) the retro for this sprint and redirect to it.
	rb, err := h.retro.GetOrCreateRetroBoard(r.Context(), orgID, boardID, &sprintID)
	if err != nil {
		// Fallback to board if retro creation fails.
		http.Redirect(w, r, "/boards/"+boardID.String(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String()+"/retro/"+rb.ID.String(), http.StatusSeeOther)
}

func (h *Handler) RetrosListPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	retros, err := h.retro.ListRetros(r.Context(), orgID, boardID)
	if err != nil {
		http.Error(w, "failed to list retros", http.StatusInternalServerError)
		return
	}
	board, _ := h.boards.GetBoard(r.Context(), orgID, boardID)
	var team *model.Team
	if board != nil && board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *board.TeamID)
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
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	retroBoardID, err := uuid.Parse(chi.URLParam(r, "retroBoardID"))
	if err != nil {
		http.Error(w, "invalid retro board ID", http.StatusBadRequest)
		return
	}
	if err := h.retro.CloseRetroBoard(r.Context(), orgID, retroBoardID, userID); err != nil {
		http.Error(w, "failed to close retro", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/boards/"+boardID.String()+"/retros", http.StatusSeeOther)
}

func (h *Handler) RetroBoardPage(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	retroBoardID, err := uuid.Parse(chi.URLParam(r, "retroBoardID"))
	if err != nil {
		http.Error(w, "invalid retro board ID", http.StatusBadRequest)
		return
	}

	rb, err := h.retro.GetRetroBoard(r.Context(), orgID, retroBoardID)
	if err != nil {
		http.Error(w, "retro not found", http.StatusNotFound)
		return
	}

	view, err := h.retro.GetRetroView(r.Context(), orgID, boardID, rb.SprintID)
	if err != nil {
		http.Error(w, "failed to load retro", http.StatusInternalServerError)
		return
	}

	var team *model.Team
	if view.Board.TeamID != nil {
		team, _ = h.teams.GetTeam(r.Context(), orgID, *view.Board.TeamID)
	}
	h.render(w, "retro.html", h.pageData(r, map[string]any{
		"RetroView":     view,
		"CurrentUserID": userID.String(),
		"Team":          team,
	}))
}

func (h *Handler) CreateRetroCard(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	userID := service.UserIDFromContext(r.Context())
	retroBoardID, err := uuid.Parse(chi.URLParam(r, "retroBoardID"))
	if err != nil {
		http.Error(w, "invalid retro board ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	col := model.RetroColumn(r.FormValue("column"))
	body := r.FormValue("body")
	if body == "" {
		http.Error(w, "body is required", http.StatusBadRequest)
		return
	}

	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
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
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	retroBoardID, err := uuid.Parse(chi.URLParam(r, "retroBoardID"))
	if err != nil {
		http.Error(w, "invalid retro board ID", http.StatusBadRequest)
		return
	}
	cardID, err := uuid.Parse(chi.URLParam(r, "cardID"))
	if err != nil {
		http.Error(w, "invalid card ID", http.StatusBadRequest)
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
	boardID, err := uuid.Parse(chi.URLParam(r, "boardID"))
	if err != nil {
		http.Error(w, "invalid board ID", http.StatusBadRequest)
		return
	}
	retroBoardID, err := uuid.Parse(chi.URLParam(r, "retroBoardID"))
	if err != nil {
		http.Error(w, "invalid retro board ID", http.StatusBadRequest)
		return
	}
	cardID, err := uuid.Parse(chi.URLParam(r, "cardID"))
	if err != nil {
		http.Error(w, "invalid card ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
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
