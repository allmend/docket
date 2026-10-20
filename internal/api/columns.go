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
		serviceError(w, err, "failed to create column")
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
		serviceError(w, err, "failed to rename column")
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}

// MoveColumn moves a column one step left or right (form field direction).
func (h *Handler) MoveColumn(w http.ResponseWriter, r *http.Request) {
	orgID := service.OrgIDFromContext(r.Context())
	boardID, ok := pathUUID(w, r, "boardID")
	if !ok {
		return
	}
	columnID, ok := pathUUID(w, r, "columnID")
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	dir := 0
	switch r.FormValue("direction") {
	case "left":
		dir = -1
	case "right":
		dir = 1
	}
	if err := h.boards.MoveColumn(r.Context(), orgID, boardID, columnID, dir); err != nil {
		serviceError(w, err, "failed to move column")
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
		serviceError(w, err, "failed to delete column")
		return
	}
	w.Header().Set("HX-Trigger", "boardUpdated")
	w.WriteHeader(http.StatusNoContent)
}
