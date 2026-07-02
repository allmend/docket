package api

import (
	"net/http"

	"github.com/allmend/docket/internal/service"
)

func (h *Handler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	_, err := h.boards.AddColumn(r.Context(), orgID, boardID, name)
	if err != nil {
		http.Error(w, "failed to create column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RenameColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	columnID, ok := pathUUID(w, r, "columnID")
	if !ok {
		return
	}

	if !parseForm(w, r) {
		return
	}
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if _, err := h.boards.RenameColumn(r.Context(), orgID, columnID, name); err != nil {
		http.Error(w, "failed to rename column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	columnID, ok := pathUUID(w, r, "columnID")
	if !ok {
		return
	}

	if err := h.boards.DeleteColumn(r.Context(), orgID, columnID); err != nil {
		http.Error(w, "failed to delete column", http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}
